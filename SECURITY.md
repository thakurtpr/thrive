# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| main    | Yes       |
| < 0.1.0 | No        |

## Reporting a Vulnerability

**Do not file a public GitHub issue for security vulnerabilities.**

Report privately by emailing **thakurprasadrout72@gmail.com** with:

- Description of the issue and affected component
- Steps to reproduce
- Potential impact

You will receive an acknowledgement within 48 hours. We aim to release a fix within 14 days for critical issues.

## Security Model

- **Container isolation**: Linux namespaces (PID, mount, UTS, IPC, net) + cgroups v2
- **Secrets**: AES-256-GCM encryption at rest; secrets injected via environment, never written to disk
- **Image integrity**: Ed25519 signatures; sign and verify commands
- **Rootless**: User namespace support; no setuid bits required
- **Network**: Bridge isolation; explicit port-forward rules via iptables NAT
- **Daemon socket**: Unix socket at `/run/thrive/thrived.sock`, mode 0660
