# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in IronGolem OS, please report it
responsibly. **Do not open a public GitHub issue for security vulnerabilities.**

### How to Report

1. Email: [security contact to be established]
2. Use GitHub's private vulnerability reporting feature on this repository

### What to Include

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact assessment
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours of report
- **Initial assessment**: Within 5 business days
- **Fix timeline**: Depends on severity; critical issues targeted within 14 days

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Scope

Security concerns relevant to this project include:

- Prompt injection attacks against agent loops
- Cross-tenant data access in team mode
- SSRF vulnerabilities in connectors and research modules
- Privilege escalation through policy bypass
- Credential exposure or insecure secret storage
- Command injection through tool execution

## Security Architecture

IronGolem OS implements a five-layer security model. See
[docs/architecture/security-model.md](docs/architecture/security-model.md) for details.
