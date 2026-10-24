# tooltrack

[English](README.md)

Herramienta de terminal multiplataforma escrita en Go que detecta la versión instalada de tus herramientas de desarrollo, consulta la última versión disponible y las actualiza por ti.

> **Estado:** desarrollo inicial. Nada de lo siguiente está publicado todavía; este README describe el alcance planificado.

## Funcionalidades previstas

- Detecta las versiones locales de `uv`, `bun`, `deno`, `rustup`, `fnm`, `conda` y `gh`.
- Consulta la última versión mediante la API de GitHub Releases o un comando de la propia herramienta.
- TUI interactiva (Bubble Tea): panel de estado, selección de herramientas a actualizar o excluir y progreso en vivo.
- Encuentra herramientas fuera del `$PATH` (por ejemplo `conda.bat` en Windows).
- Interfaz en español e inglés.
- GUI de escritorio opcional (Wails) en una fase posterior.

Objetivos: Windows 11 (x86_64), Linux (x86_64), macOS (x86_64 y arm64). Los binarios CLI son estáticos (`CGO_ENABLED=0`).

## Configuración

Las preferencias están en `config.json` (`language`, `default_action`, `github_token`, `max_concurrent_checks`, `timeout_seconds`). El token personal de GitHub es opcional y sube el límite de la API de 60 a 5,000 peticiones por hora.

## Compilación

Requiere Go 1.24 o superior.

```sh
go build ./...
go test ./...
```

## Contribuir

Los cambios se integran en la rama `develop`; `main` no se modifica directamente. Consulta [CLAUDE.md](CLAUDE.md) para las convenciones de colaboradores y agentes de código.

## Licencia

MIT. Consulta [LICENSE](LICENSE).
