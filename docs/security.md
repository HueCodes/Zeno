# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

Once v1.0.0 is released, we will support the latest minor version and one previous minor version.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

**security@huecodes.dev**

You should receive a response within 48 hours. If for some reason you do not, please follow up via email to ensure we received your original message.

Please include the following information in your report:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

## Vulnerability Disclosure Process

1. **Report received**: We acknowledge receipt within 48 hours
2. **Confirmation**: We confirm the issue and determine severity (1-7 days)
3. **Fix development**: We develop and test a fix (timeline depends on severity)
4. **Release**: We release a patched version
5. **Disclosure**: We publish a security advisory on GitHub
6. **Credit**: We credit the reporter (if desired) in release notes

## Security Best Practices

When deploying Zeno:

### GitHub Token Security
- Use tokens with minimal required scopes (repo or admin:org only)
- Rotate tokens regularly (every 90 days recommended)
- Never commit tokens to source code
- Use environment variables or secret management systems
- Consider using GitHub App tokens instead of PAT for production

### Network Security
- Run Zeno behind a firewall or VPN
- Use TLS/SSL for API endpoints if exposing publicly
- Restrict API access to trusted networks
- Consider adding authentication to API endpoints (future feature)

### Container Security
- Use official Docker images from ghcr.io/huecodes/zeno
- Verify image checksums before deployment
- Run containers as non-root user
- Scan images for vulnerabilities regularly
- Keep base images updated

### Infrastructure
- Follow provider-specific security best practices
- Use IAM roles instead of access keys (AWS)
- Enable audit logging on cloud resources
- Encrypt runner disks at rest
- Use private networks for runner communication

### Monitoring
- Enable audit logging in Zeno
- Monitor for suspicious scaling activity
- Alert on authentication failures
- Track API access patterns
- Review scaling history regularly

## Known Limitations

Current version (0.1.x) limitations:

- No authentication on REST API endpoints
- GitHub tokens stored in environment variables
- No built-in TLS support (use reverse proxy)
- Analytics data stored in memory (lost on restart)
- No rate limiting on API endpoints

These will be addressed in future releases. See [ROADMAP.md](roadmap.md).

## Security Updates

Security updates will be released as patch versions (0.1.x) and announced via:

- GitHub Security Advisories
- Release notes with `[SECURITY]` prefix
- GitHub Discussions (pinned post)

Subscribe to the repository to receive notifications.

## Bug Bounty

We do not currently have a bug bounty program. However, we greatly appreciate security researchers who responsibly disclose vulnerabilities. We will:

- Acknowledge your contribution publicly (if desired)
- Credit you in release notes and CHANGELOG
- Respond promptly and keep you updated on fix progress

## Contact

For security concerns: **security@huecodes.dev**

For general questions: [GitHub Discussions](https://github.com/HueCodes/Zeno/discussions)

## Additional Resources

- [GitHub Token Security](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
