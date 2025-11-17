# Architecture

## System Overview

```
┌─────────────┐
│   GitHub    │
│   Actions   │
│   API       │
└──────┬──────┘
       │ Queue Depth
       │ Runner Status
       ▼
┌─────────────────────────────────────┐
│     Zeno Controller                 │
│  ┌───────────────────────────────┐  │
│  │  Reconciliation Loop          │  │
│  │  - Query queue depth          │  │
│  │  - Calculate desired runners  │  │
│  │  - Scale up/down              │  │
│  │  - Record decisions           │  │
│  └───────────────────────────────┘  │
│                                     │
│  ┌───────────┐  ┌───────────────┐  │
│  │ Analytics │  │   REST API    │  │
│  │ Tracker   │  │  /health      │  │
│  └───────────┘  │  /metrics     │  │
│                 │  /runners     │  │
│                 └───────────────┘  │
└─────────┬───────────────────────────┘
          │ Scale Commands
          ▼
    ┌──────────────┐
    │   Provider   │
    │  Interface   │
    └──────┬───────┘
           │
    ┌──────┴───────┬────────┬────────┐
    ▼              ▼        ▼        ▼
┌────────┐   ┌────────┐  ┌───┐  ┌───────┐
│ Docker │   │  AWS   │  │GCP│  │ Azure │
└────────┘   └────────┘  └───┘  └───────┘
```

## Core Components

### 1. Reconciliation Loop

**Location**: `internal/controller/controller.go`

The heart of Zeno. Runs continuously on a configurable interval:

1. **Query**: Fetch queued workflow jobs from GitHub API
2. **Calculate**: Determine desired runner count based on thresholds
3. **Scale**: Issue scale up/down commands to provider
4. **Record**: Track decision in analytics for observability
5. **Sleep**: Wait for next interval

**Key Logic**:
```go
if queueDepth > scaleUpThreshold {
    desired = min(queueDepth, maxRunners)
} else if queueDepth < scaleDownThreshold {
    desired = minRunners
}
```

### 2. GitHub Client

**Location**: `internal/github/client.go`

Handles all GitHub API interactions:
- Authentication via Personal Access Token
- Querying queued workflow jobs (org or repo level)
- Rate limit handling
- Error retry logic

Uses GitHub REST API v3: `/repos/{owner}/{repo}/actions/runs` or `/orgs/{org}/actions/runs`

### 3. Runner Manager

**Location**: `internal/runner/manager.go`

Abstracts runner lifecycle across providers:
- `ScaleUp(count)` - Create new runners
- `ScaleDown(count)` - Terminate runners
- `Count()` - Get current runner count
- `List()` - List active runners

Each provider implements this interface.

### 4. Provider Interface

**Location**: `internal/runner/provider.go` (conceptual)

Pluggable backends for different infrastructure:

- **Docker**: Local containers with `myoung34/github-runner` image
- **AWS**: EC2 instances with spot support (in progress)
- **GCP**: Compute Engine VMs (in progress)
- **Azure**: Virtual Machines (in progress)

### 5. Analytics Tracker

**Location**: `internal/analytics/tracker.go`

In-memory metrics and decision history:
- Current runner count
- Queue depth over time
- Scale up/down events
- API request metrics
- Last 100 decisions retained

### 6. REST API

**Location**: `pkg/api/v1/handlers.go`

HTTP endpoints for monitoring:
- `GET /health` - Health check
- `GET /api/v1/metrics` - Current metrics
- `GET /api/v1/runners` - Runner list
- `GET /api/v1/history` - Scaling history

## Data Flow

### Scale Up Scenario

```
GitHub Queue = 8 jobs
Current Runners = 2
Threshold = 5

1. Controller queries GitHub → 8 queued
2. Calculates: 8 > 5 → scale up to 8
3. Checks max: min(8, 10) = 8
4. Manager.ScaleUp(6) → create 6 new runners
5. Provider spins up 6 containers/VMs
6. Tracker records decision: +6 runners at timestamp
7. API /metrics updated with new count
```

### Scale Down Scenario

```
GitHub Queue = 0 jobs
Current Runners = 8
Threshold = 0

1. Controller queries GitHub → 0 queued
2. Calculates: 0 < 0 → scale down to min
3. Checks min: max(1, minRunners=1) = 1
4. Manager.ScaleDown(7) → terminate 7 runners
5. Provider destroys 7 containers/VMs
6. Tracker records decision: -7 runners
```

## Configuration

**Location**: `internal/config/config.go`

Loads from environment variables:
- GitHub credentials
- Scaling thresholds
- Provider settings
- Intervals and timeouts

Validated on startup. Fails fast on missing required fields.

## Concurrency Model

- Single reconciliation loop (no concurrent scaling)
- HTTP handlers run concurrently (safe read-only access)
- Analytics tracker uses mutex for writes
- Provider calls are synchronous (no parallel creates)

## Future Architecture

### Phase 2: Webhooks
- GitHub webhook receiver for instant scaling
- Event-driven vs polling hybrid

### Phase 3: HA Mode
- Leader election (etcd/Consul)
- Shared state in database
- Multiple controller instances

### Phase 4: Dashboard
- WebSocket for live metrics
- React SPA in `web/` directory
- Embedded in binary via `embed`

## Development Notes

### Adding a Provider

1. Implement `Provider` interface in `internal/runner/`
2. Add provider-specific config to `internal/config/`
3. Register in provider factory
4. Add tests in `internal/runner/<provider>_test.go`
5. Document in `docs/providers/<name>.md`

### Extending Metrics

1. Add field to `Metrics` struct in `internal/models/types.go`
2. Update tracker in `internal/analytics/tracker.go`
3. Expose in API handler if needed
4. Add to Prometheus exporter (future)

### Testing Strategy

- **Unit**: Individual package tests with mocks
- **Integration**: Full controller with fake GitHub API
- **E2E**: Docker Compose stack with real GitHub repo (manual)

## References

- [Kubernetes Controller Pattern](https://kubernetes.io/docs/concepts/architecture/controller/)
- [GitHub Actions API](https://docs.github.com/en/rest/actions)
- [Reconciliation Loop](https://hackernoon.com/level-triggering-and-reconciliation-in-kubernetes-1f17fe30333d)
