# Quick Start Guide

Get Zeno running in 5 minutes.

## Prerequisites

- GitHub Personal Access Token with `repo` or `admin:org` scope
- Either Docker installed OR a Linux/macOS/Windows machine

## Option 1: Binary Install

### 1. Download Binary

```bash
# Linux AMD64
curl -LO https://github.com/HueCodes/Zeno/releases/latest/download/zeno-linux-amd64
chmod +x zeno-linux-amd64
sudo mv zeno-linux-amd64 /usr/local/bin/zeno

# macOS ARM64
curl -LO https://github.com/HueCodes/Zeno/releases/latest/download/zeno-darwin-arm64
chmod +x zeno-darwin-arm64
sudo mv zeno-darwin-arm64 /usr/local/bin/zeno
```

### 2. Set Environment Variables

```bash
export GITHUB_TOKEN="ghp_your_token_here"
export GITHUB_ORG="your-org-name"  # or GITHUB_REPO="owner/repo"
```

### 3. Run

```bash
zeno
```

### 4. Verify

```bash
curl http://localhost:8080/health
# Should return: {"status":"healthy"}
```

## Option 2: Docker

### 1. Create `.env` File

```bash
cat > .env <<EOF
GITHUB_TOKEN=ghp_your_token_here
GITHUB_ORG=your-org-name
MIN_RUNNERS=1
MAX_RUNNERS=5
EOF
```

### 2. Run with Docker

```bash
docker run -d \
  --name zeno \
  --env-file .env \
  -p 8080:8080 \
  ghcr.io/huecodes/zeno:latest
```

### 3. Check Logs

```bash
docker logs -f zeno
```

### 4. Verify

```bash
curl http://localhost:8080/api/v1/metrics
```

## What's Next?

- **Monitor**: Check `/api/v1/runners` to see runner count
- **Metrics**: View `/api/v1/metrics` for scaling metrics
- **History**: Check `/api/v1/history` for scaling decisions
- **Configure**: See [setup guide](setup.md) for advanced configuration
- **Develop**: Check [CONTRIBUTING.md](../CONTRIBUTING.md) to contribute

## Troubleshooting

**"Authentication failed"**
- Verify token has correct scopes (repo or admin:org)
- Check token hasn't expired

**"No runners scaling"**
- Verify workflow jobs are queued in GitHub
- Check `SCALE_UP_THRESHOLD` (default: 5 queued jobs needed)
- Review logs for errors

**"Port already in use"**
- Change port with `-p 9090:8080` flag (Docker)
- Or set `PORT` environment variable

## Example Configuration

```bash
# Aggressive scaling
export MIN_RUNNERS=2
export MAX_RUNNERS=20
export SCALE_UP_THRESHOLD=2
export CHECK_INTERVAL_SEC=15

# Conservative scaling
export MIN_RUNNERS=1
export MAX_RUNNERS=5
export SCALE_UP_THRESHOLD=10
export CHECK_INTERVAL_SEC=60
```

See [.env.example](../.env.example) for all options.
