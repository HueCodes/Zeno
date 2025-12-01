package ec2

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/provider"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
)

const (
	tagManagedBy  = "zeno:managed-by"
	tagRunnerID   = "zeno:runner-id"
	tagRunnerName = "zeno:runner-name"
	tagCreatedAt  = "zeno:created-at"
)

type EC2Provider struct {
	client *ec2.Client
	config config.AWSConfig
	logger *slog.Logger
	mu     sync.RWMutex
}

// New creates a new EC2 provider
func New(cfg config.AWSConfig, logger *slog.Logger) (*EC2Provider, error) {
	ctx := context.Background()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &EC2Provider{
		client: ec2.NewFromConfig(awsCfg),
		config: cfg,
		logger: logger.With("provider", "ec2"),
	}, nil
}

func (p *EC2Provider) Name() string {
	return "ec2"
}

func (p *EC2Provider) ListRunners(ctx context.Context) ([]*provider.Runner, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + tagManagedBy),
				Values: []string{"zeno"},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"pending",
					"running",
					"stopping",
					"stopped",
				},
			},
		},
	}

	result, err := p.client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var runners []*provider.Runner
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			runner := p.instanceToRunner(&instance)
			runners = append(runners, runner)
		}
	}

	return runners, nil
}

func (p *EC2Provider) GetRunner(ctx context.Context, id string) (*provider.Runner, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + tagRunnerID),
				Values: []string{id},
			},
		},
	}

	result, err := p.client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("runner %s not found", id)
	}

	return p.instanceToRunner(&result.Reservations[0].Instances[0]), nil
}

func (p *EC2Provider) CreateRunner(ctx context.Context, req *provider.CreateRunnerRequest) (*provider.Runner, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	runnerID := uuid.New().String()

	p.logger.Info("creating EC2 instance",
		"id", runnerID,
		"name", req.Name,
		"instance_type", p.config.InstanceType,
		"use_spot", p.config.UseSpot,
	)

	userData := p.buildUserData(req)
	userDataB64 := base64.StdEncoding.EncodeToString([]byte(userData))

	tags := p.buildTags(runnerID, req)
	tagSpecs := []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags:         tags,
		},
		{
			ResourceType: types.ResourceTypeVolume,
			Tags:         tags,
		},
	}

	blockDeviceMappings := []types.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/sda1"),
			Ebs: &types.EbsBlockDevice{
				VolumeSize:          aws.Int32(p.config.VolumeSize),
				VolumeType:          types.VolumeType(p.config.VolumeType),
				DeleteOnTermination: aws.Bool(true),
			},
		},
	}

	var instanceID string
	var err error

	if p.config.UseSpot {
		instanceID, err = p.createSpotInstance(ctx, userDataB64, tagSpecs, blockDeviceMappings)
	} else {
		instanceID, err = p.createOnDemandInstance(ctx, userDataB64, tagSpecs, blockDeviceMappings)
	}

	if err != nil {
		return nil, err
	}

	p.logger.Info("EC2 instance created",
		"id", runnerID,
		"instance_id", instanceID,
	)

	return &provider.Runner{
		ID:         runnerID,
		Name:       req.Name,
		Status:     provider.StatusProvisioning,
		Labels:     req.Labels,
		Provider:   "ec2",
		ProviderID: instanceID,
		CreatedAt:  time.Now(),
		Metadata: map[string]string{
			"instance_id":   instanceID,
			"instance_type": p.config.InstanceType,
			"region":        p.config.Region,
			"spot":          fmt.Sprintf("%t", p.config.UseSpot),
		},
	}, nil
}

func (p *EC2Provider) RemoveRunner(ctx context.Context, id string, graceful bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	runner, err := p.GetRunner(ctx, id)
	if err != nil {
		return err
	}

	p.logger.Info("terminating EC2 instance",
		"id", id,
		"instance_id", runner.ProviderID,
		"graceful", graceful,
	)

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{runner.ProviderID},
	}

	_, err = p.client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	p.logger.Info("EC2 instance termination initiated", "id", id)
	return nil
}

func (p *EC2Provider) HealthCheck(ctx context.Context) error {
	// Simple check: describe regions to verify API access
	svc := p.client
	_, err := svc.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("EC2 health check failed: %w", err)
	}
	return nil
}

func (p *EC2Provider) Close() error {
	return nil
}

func (p *EC2Provider) createOnDemandInstance(
	ctx context.Context,
	userData string,
	tagSpecs []types.TagSpecification,
	blockDeviceMappings []types.BlockDeviceMapping,
) (string, error) {
	input := &ec2.RunInstancesInput{
		ImageId:             aws.String(p.config.AMI),
		InstanceType:        types.InstanceType(p.config.InstanceType),
		MinCount:            aws.Int32(1),
		MaxCount:            aws.Int32(1),
		UserData:            aws.String(userData),
		SubnetId:            aws.String(p.config.SubnetID),
		SecurityGroupIds:    p.config.SecurityGroupIDs,
		TagSpecifications:   tagSpecs,
		BlockDeviceMappings: blockDeviceMappings,
	}

	if p.config.KeyName != "" {
		input.KeyName = aws.String(p.config.KeyName)
	}

	if p.config.IAMInstanceProfile != "" {
		input.IamInstanceProfile = &types.IamInstanceProfileSpecification{
			Name: aws.String(p.config.IAMInstanceProfile),
		}
	}

	result, err := p.client.RunInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to run on-demand instance: %w", err)
	}

	if len(result.Instances) == 0 {
		return "", fmt.Errorf("no instances created")
	}

	return *result.Instances[0].InstanceId, nil
}

func (p *EC2Provider) createSpotInstance(
	ctx context.Context,
	userData string,
	tagSpecs []types.TagSpecification,
	blockDeviceMappings []types.BlockDeviceMapping,
) (string, error) {
	launchSpec := &types.RequestSpotLaunchSpecification{
		ImageId:             aws.String(p.config.AMI),
		InstanceType:        types.InstanceType(p.config.InstanceType),
		UserData:            aws.String(userData),
		SubnetId:            aws.String(p.config.SubnetID),
		SecurityGroupIds:    p.config.SecurityGroupIDs,
		BlockDeviceMappings: blockDeviceMappings,
	}

	if p.config.KeyName != "" {
		launchSpec.KeyName = aws.String(p.config.KeyName)
	}

	if p.config.IAMInstanceProfile != "" {
		launchSpec.IamInstanceProfile = &types.IamInstanceProfileSpecification{
			Name: aws.String(p.config.IAMInstanceProfile),
		}
	}

	input := &ec2.RequestSpotInstancesInput{
		SpotPrice:           aws.String(p.config.SpotMaxPrice),
		InstanceCount:       aws.Int32(1),
		Type:                types.SpotInstanceTypeOneTime,
		LaunchSpecification: launchSpec,
		TagSpecifications:   tagSpecs,
	}

	result, err := p.client.RequestSpotInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to request spot instance: %w", err)
	}

	if len(result.SpotInstanceRequests) == 0 {
		return "", fmt.Errorf("no spot requests created")
	}

	requestID := *result.SpotInstanceRequests[0].SpotInstanceRequestId

	// Wait for spot request to be fulfilled
	waiter := ec2.NewSpotInstanceRequestFulfilledWaiter(p.client)
	waitInput := &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []string{requestID},
	}

	if err := waiter.Wait(ctx, waitInput, 5*time.Minute); err != nil {
		return "", fmt.Errorf("spot request not fulfilled: %w", err)
	}

	// Get instance ID from fulfilled request
	descResult, err := p.client.DescribeSpotInstanceRequests(ctx, waitInput)
	if err != nil {
		return "", fmt.Errorf("failed to describe spot request: %w", err)
	}

	if len(descResult.SpotInstanceRequests) == 0 || descResult.SpotInstanceRequests[0].InstanceId == nil {
		return "", fmt.Errorf("spot request has no instance ID")
	}

	instanceID := *descResult.SpotInstanceRequests[0].InstanceId

	// Tag the instance (spot instances don't inherit tags from request)
	tagInput := &ec2.CreateTagsInput{
		Resources: []string{instanceID},
		Tags:      tagSpecs[0].Tags,
	}
	_, err = p.client.CreateTags(ctx, tagInput)
	if err != nil {
		p.logger.Warn("failed to tag spot instance", "error", err)
	}

	return instanceID, nil
}

func (p *EC2Provider) buildUserData(req *provider.CreateRunnerRequest) string {
	if p.config.UserDataScript != "" {
		// Use custom user data script
		script := p.config.UserDataScript
		script = strings.ReplaceAll(script, "{{RUNNER_NAME}}", req.Name)
		script = strings.ReplaceAll(script, "{{GITHUB_TOKEN}}", req.GitHubToken)
		script = strings.ReplaceAll(script, "{{GITHUB_ORG}}", req.GitHubOrg)
		script = strings.ReplaceAll(script, "{{GITHUB_REPO}}", req.GitHubRepo)
		script = strings.ReplaceAll(script, "{{LABELS}}", strings.Join(req.Labels, ","))
		return script
	}

	// Default user data script
	return fmt.Sprintf(`#!/bin/bash
set -e

# Install GitHub Actions runner
cd /home/ubuntu
mkdir actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-2.311.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-x64-2.311.0.tar.gz
tar xzf ./actions-runner-linux-x64-2.311.0.tar.gz

# Configure runner
./config.sh --url https://github.com/%s --token %s --name %s --labels %s --unattended --ephemeral

# Start runner
./run.sh
`,
		req.GitHubOrg,
		req.GitHubToken,
		req.Name,
		strings.Join(req.Labels, ","),
	)
}

func (p *EC2Provider) buildTags(runnerID string, req *provider.CreateRunnerRequest) []types.Tag {
	tags := []types.Tag{
		{
			Key:   aws.String(tagManagedBy),
			Value: aws.String("zeno"),
		},
		{
			Key:   aws.String(tagRunnerID),
			Value: aws.String(runnerID),
		},
		{
			Key:   aws.String(tagRunnerName),
			Value: aws.String(req.Name),
		},
		{
			Key:   aws.String(tagCreatedAt),
			Value: aws.String(time.Now().Format(time.RFC3339)),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String(fmt.Sprintf("zeno-runner-%s", runnerID[:8])),
		},
	}

	// Add custom tags from config
	for k, v := range p.config.Tags {
		tags = append(tags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	return tags
}

func (p *EC2Provider) instanceToRunner(instance *types.Instance) *provider.Runner {
	runnerID := ""
	runnerName := ""
	createdAt := time.Now()

	for _, tag := range instance.Tags {
		switch *tag.Key {
		case tagRunnerID:
			runnerID = *tag.Value
		case tagRunnerName:
			runnerName = *tag.Value
		case tagCreatedAt:
			if t, err := time.Parse(time.RFC3339, *tag.Value); err == nil {
				createdAt = t
			}
		}
	}

	status := mapInstanceState(instance.State.Name)

	metadata := map[string]string{
		"instance_id":    *instance.InstanceId,
		"instance_type":  string(instance.InstanceType),
		"state":          string(instance.State.Name),
		"az":             *instance.Placement.AvailabilityZone,
	}

	if instance.PrivateIpAddress != nil {
		metadata["private_ip"] = *instance.PrivateIpAddress
	}
	if instance.PublicIpAddress != nil {
		metadata["public_ip"] = *instance.PublicIpAddress
	}

	return &provider.Runner{
		ID:         runnerID,
		Name:       runnerName,
		Status:     status,
		Provider:   "ec2",
		ProviderID: *instance.InstanceId,
		CreatedAt:  createdAt,
		Metadata:   metadata,
	}
}

func mapInstanceState(state types.InstanceStateName) provider.RunnerStatus {
	switch state {
	case types.InstanceStateNamePending:
		return provider.StatusProvisioning
	case types.InstanceStateNameRunning:
		return provider.StatusRunning
	case types.InstanceStateNameStopping:
		return provider.StatusTerminating
	case types.InstanceStateNameStopped:
		return provider.StatusTerminated
	case types.InstanceStateNameShuttingDown:
		return provider.StatusTerminating
	case types.InstanceStateNameTerminated:
		return provider.StatusTerminated
	default:
		return provider.StatusFailed
	}
}
