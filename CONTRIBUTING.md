# Contributing

Thank you for considering contributing to Zeno! This guide will help you get started.

## Quick Start

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/Zeno`
3. Create a branch: `git checkout -b feature/your-feature`
4. Make changes and test
5. Commit with conventional commit format
6. Push and open a pull request

## Development Setup

### Requirements

- Go 1.21+
- Docker (for local testing)
- golangci-lint (for linting)

### Install Dependencies

```bash
cd Zeno
make deps
```

### Run Locally

```bash
# Set required environment variables
export GITHUB_TOKEN=your_token
export GITHUB_ORG=your_org

# Run controller
make run
```

### Run with Hot Reload

```bash
make dev
```

This uses [air](https://github.com/cosmtrek/air) for automatic reloading on file changes.

## Testing

### Run All Tests

```bash
make test
```

### Run with Coverage

```bash
make test-coverage
```

Coverage must be above 70% for each package. The build will fail if coverage drops below this threshold.

### Run Integration Tests

```bash
# Start test environment
docker-compose -f docker/docker-compose.yml up -d

# Run tests
go test -v ./test/integration/...

# Cleanup
docker-compose -f docker/docker-compose.yml down
```

## Code Quality

### Format Code

```bash
make fmt
```

### Run Linter

```bash
make lint
```

All linters must pass before PR can be merged.

## Code Standards

- Follow Go standard formatting (`gofmt`)
- Functions should be under 50 lines
- Add tests for new features
- Keep external dependencies minimal
- Use meaningful variable names
- Comment only when necessary for clarity

### Package Structure

- `cmd/` - Application entry points
- `internal/` - Private application code
- `pkg/` - Public libraries (importable by others)
- `docs/` - Documentation
- `examples/` - Usage examples
- `test/` - Integration and E2E tests

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): subject

body (optional)

footer (optional)
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements

**Examples**:
```
feat(docker): add multi-arch build support

fix(controller): handle nil pointer in reconciliation loop

docs: update quick start guide with binary install

test(api): add handler tests for error cases
```

## Adding a Provider

To add support for a new infrastructure provider:

1. **Create provider file**: `internal/runner/provider_<name>.go`
2. **Implement interface**:
   ```go
   type Provider interface {
       ScaleUp(count int) error
       ScaleDown(count int) error
       Count() int
       List() ([]Runner, error)
   }
   ```
3. **Add configuration**: Update `internal/config/config.go`
4. **Write tests**: Create `internal/runner/provider_<name>_test.go`
5. **Document**: Add `docs/providers/<name>.md` with setup instructions
6. **Update README**: Add to supported providers list

## Priority Areas

We're especially looking for contributions in:

- **Providers**: AWS, GCP, Azure implementations
- **Testing**: Integration tests and edge cases
- **Documentation**: Setup guides and troubleshooting
- **Observability**: Prometheus metrics, logging improvements
- **Performance**: Scaling speed optimizations
- **Features**: Webhook support, runner label matching

## Pull Request Process

1. **Update documentation** if adding features
2. **Add tests** for new functionality
3. **Run full CI pipeline**: `make ci`
4. **Update CHANGELOG.md** if user-facing change
5. **Request review** from maintainers
6. **Address feedback** in review
7. **Squash commits** if requested

### PR Checklist

- [ ] Tests added/updated
- [ ] All tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Code formatted (`make fmt`)
- [ ] Documentation updated
- [ ] Conventional commit format used
- [ ] CHANGELOG.md updated (if applicable)

## Reporting Issues

### Bug Reports

Use the [Bug Report template](.github/ISSUE_TEMPLATE/bug_report.yml) and include:

- Zeno version
- Go version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs (sanitize tokens!)

### Feature Requests

Use the [Feature Request template](.github/ISSUE_TEMPLATE/feature_request.yml) and include:

- Problem statement
- Proposed solution
- Alternatives considered
- Willingness to contribute

## Code Review Guidelines

Reviewers will check for:

- Code quality and readability
- Test coverage and correctness
- Documentation completeness
- Breaking changes (require major version bump)
- Security implications
- Performance impact

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/HueCodes/Zeno/discussions)
- **Bugs**: [GitHub Issues](https://github.com/HueCodes/Zeno/issues)
- **Real-time**: Comment on relevant issues/PRs

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors will be:
- Listed in release notes
- Mentioned in CHANGELOG.md
- Added to a future CONTRIBUTORS.md file

Thank you for contributing to Zeno! 🚀
