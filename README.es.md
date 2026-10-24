# tooltrack

[English](README.md)

Herramienta de terminal multiplataforma escrita en Go que detecta la versión instalada de tus herramientas de desarrollo, consulta la última versión disponible y las actualiza por ti.

> **Estado:** desarrollo inicial. Todavía no hay versiones publicadas; compílala desde el código fuente (ver abajo). La especificación técnica completa está en [spec/spec.md](spec/spec.md).

## Funcionalidades

- Detecta la versión local de `uv`, `bun`, `deno`, `rustup`, `fnm`, `conda` y `gh`.
- Consulta la última versión con la API de GitHub Releases (o con un comando de la propia herramienta).
- TUI interactiva (Bubble Tea): panel de estado, actualizar todas, elegir cuáles actualizar o excluir, progreso en vivo y resumen final.
- Encuentra herramientas fuera del `PATH` (por ejemplo `conda.bat` en Windows o `~/miniconda3/bin/conda`).
- Comandos de actualización según el sistema operativo (`winget`, `brew`, `cargo`, actualizadores propios).
- La salida de cada actualización va a un archivo de log, nunca sobre la interfaz.
- Interfaz en español e inglés.

Objetivos: Windows 11 (x86_64), Linux (x86_64), macOS (x86_64 y arm64). Los binarios CLI son estáticos (`CGO_ENABLED=0`).

## Uso

```sh
go run ./cmd/cli          # desde el código fuente
./tooltrack               # una vez compilado
./tooltrack --version
./tooltrack --home DIR    # guarda data/ y logs/ en DIR
```

El programa abre un menú (ver estatus, preferencias, salir). Las versiones se buscan al elegir **Ver estatus de herramientas**, que muestra el panel; ahí, `Enter` abre las acciones (actualizar todas, elegir cuáles actualizar o excluir).

| Tecla | Acción |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Mover |
| `Espacio` | Marcar una casilla o cambiar una preferencia |
| `Enter` | Confirmar |
| `Esc` | Volver |
| `r` | Volver a comprobar (panel) |
| `l` | Abrir el log (tras una actualización fallida) |
| `q` / `Ctrl+C` | Salir (cancela antes una actualización en curso) |

## Configuración

Los archivos se crean la primera vez en la carpeta de configuración del usuario (`%APPDATA%\tooltrack`, `~/.config/tooltrack`, `~/Library/Application Support/tooltrack`). Define `TOOLTRACK_HOME` o usa `--home` para otra carpeta.

```text
data/config.json   preferencias
data/tools.json    catálogo de herramientas (tus entradas ganan sobre los presets)
logs/updater.log   salida de cada actualización, rota a los 5 MB
```

Campos de `config.json`: `language` (`es` | `en`), `default_action` (`ask` | `update_all` | `update_selected` | `exclude_selected`), `github_token`, `max_concurrent_checks` y `timeout_seconds`. El token personal de GitHub es opcional y sube el límite de la API de 60 a 5,000 peticiones por hora; si el archivo lo deja vacío se usa la variable de entorno `GITHUB_TOKEN`.

## Compilación

Requiere Go 1.24 o superior.

```sh
make check        # gofmt, go vet y go test
make cli-all      # binarios estáticos para todas las plataformas en dist/
make cli-linux    # o cli-windows, cli-darwin-amd64, cli-darwin-arm64
scripts/build.sh  # lo mismo sin make (scripts/build.ps1 en Windows)
```

## Contribuir

Los cambios se integran en la rama `develop`; `main` no se modifica directamente. Consulta [CLAUDE.md](CLAUDE.md) para las convenciones de colaboradores y agentes de código.

## Licencia

MIT. Consulta [LICENSE](LICENSE).
