# Security Policy

## Supported Versions

We currently provide security updates for the latest stable release only.

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < latest | :x:               |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue
in scrappy, please **do not** open a public GitHub issue. Instead, report it
privately:

1. **Email**: [INSERT EMAIL]
2. **GitHub**: Use the private vulnerability reporting feature at:
   https://github.com/arinbalyan/scrappy/security/advisories/new

We will acknowledge receipt within **48 hours** and provide a timeline for
a fix and disclosure within **7 days**.

### What to include

- Description of the vulnerability
- Steps to reproduce (PoC preferred)
- Affected versions and configurations
- Potential impact (data exposure, RCE, etc.)

## Disclosure Policy

- We will investigate and fix the issue as quickly as possible
- A security advisory will be published via GitHub
- Credit will be given to the reporter (unless anonymity is requested)
- We will notify downstream packagers before public disclosure

## Scope

We care about:

- Remote code execution
- SQL injection (if applicable)
- Cross-site scripting (in exported HTML)
- Exposure of API keys or credentials
- Memory safety issues

We do **not** consider the following as security issues:

- Job board rate limiting / IP blocking
- Missing job board features
- CLI flag abuse (the tool is designed for personal use)

## Security-Related Configuration

If you find a potential vulnerability in one of our dependencies, please
report it using the process above. We use Dependabot for automated dependency
scanning and will patch known CVEs promptly.

## Bug Bounty

This project does not currently offer a bug bounty program, but we will
publicly acknowledge all valid security reports.

---

*Thank you for helping keep scrappy and its users safe.*
