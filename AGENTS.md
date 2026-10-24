# AGENTS.md

Las instrucciones para agentes de código de este repositorio están en [CLAUDE.md](CLAUDE.md) y aplican a cualquier agente (Claude Code, Cursor, Cline, Aider, etc.).

Lo esencial:

- Solo se permiten cambios en la rama `develop` (PRs hacia `develop`); nunca tocar `main`.
- Antes de hacer commit: `gofmt -l .`, `go vet ./...` y `go test ./...`.
- Responder al usuario en español.
