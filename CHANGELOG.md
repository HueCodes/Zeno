# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2024-11-17

### Added
- Initial public release
- Core reconciliation loop for autoscaling
- GitHub API client for queue depth monitoring
- Docker provider support (partial implementation)
- REST API endpoints: /health, /api/v1/metrics, /api/v1/runners, /api/v1/history
- Analytics tracker for metrics and scaling decisions
- Configuration via environment variables
- Prometheus-compatible metrics endpoint
- CI/CD pipeline with GitHub Actions
- Multi-platform binary releases (Linux, macOS, Windows)
- Docker image with multi-arch support (amd64, arm64)
- Comprehensive documentation and API reference
- Test suite with 70%+ coverage for core packages
- Makefile with development targets
- Contributing guidelines and code standards

### Infrastructure
- Multi-stage Dockerfile with Alpine base
- Docker Compose with monitoring stack
- GitHub Actions workflows for CI and releases
- Issue and PR templates
- golangci-lint configuration

### Documentation
- Quick start guide
- Setup guide with troubleshooting
- Architecture documentation
- API reference
- Security policy
- Roadmap

[Unreleased]: https://github.com/HueCodes/Zeno/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/HueCodes/Zeno/releases/tag/v0.1.0
