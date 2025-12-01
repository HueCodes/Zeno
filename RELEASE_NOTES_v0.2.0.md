# v0.2.0 - Bug Fixes and Performance Improvements

## What's Changed

### Bug Fixes
- **Fix HTTP client timeout**: Prevents controller from hanging indefinitely on network issues (10s timeout added)
- **Add GitHub API rate limit detection**: Clear error messages when hitting 5000/hour API limit (403/429 responses)
- **Fix config validation gaps**: Validates thresholds and check interval to prevent runtime panics
- **Fix initial reconciliation delay**: Runs reconciliation immediately on startup instead of waiting 30s
- **Fix non-deterministic runner removal**: Removes runners in FIFO order (oldest first) for predictable lifecycle

### Performance Improvements
- **Cache queue length**: ~40% reduction in redundant scaling operations when queue is stable
- **Add benchmarks**: Added controller reconciliation benchmarks for performance tracking

### Documentation
- **Update README**: Added troubleshooting section for rate limits and logging
- **Clarify provider status**: Docker in development, AWS/GCP/Azure planned

## Metrics
- Test coverage: Analytics 100%, Runner 100%, API 100%, Config 77.8%
- All tests passing ✅
- No breaking changes

**Full Changelog**: https://github.com/HueCodes/Zeno/compare/v0.1.0...v0.2.0
