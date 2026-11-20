# SakuraOps

Self-hosted infrastructure management platform.

## Version 0.1

- Keycloak SSO (OpenID Connect)
- Minimal web dashboard
- Interactive administrative console
- CLI command: skr
- Shared API for CLI and future GUI

## Development roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for delivered features, the gated PR #18–21 plan, and long-term milestones.

## Technology

- Backend: Python / FastAPI
- Frontend: React / TypeScript
- CLI: Python / Typer
- Automation: Ansible Runner
- Deployment: Docker Compose

## Security

- Authentication required
- Role-based authorization
- No unrestricted system shell
- No secrets committed to Git
- Infrastructure changes require explicit authorization
