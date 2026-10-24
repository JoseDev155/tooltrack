# Especificación técnica: tooltrack

Actualizador multiplataforma de herramientas de desarrollo en Go (CLI/TUI). Este documento parte de la especificación inicial y refleja lo que está implementado, con las correcciones acordadas. Las diferencias con la versión original están resumidas en la sección 12.

## 1. Alcance

- **Propósito:** detectar la versión local de herramientas de desarrollo (`uv`, `bun`, `deno`, `rustup`, `fnm`, `conda`, `gh`), consultar la última versión disponible y ejecutar su actualización.
- **Sistemas objetivo:** Windows 11 (x86_64), Linux (x86_64) y macOS (x86_64 y arm64).
- **Principios:**
  - Binarios CLI estáticos (`CGO_ENABLED=0`).
  - Sin procesos en segundo plano: todo se ejecuta bajo demanda, con goroutines acotadas.
  - Tolerancia a binarios ausentes, sin permisos o fuera de `PATH`; nunca `panic` por eso.
  - Soporte para scripts `.bat`/`.cmd` en Windows.
- **Fuera de alcance por ahora:** la GUI de escritorio (Wails). Ver sección 11.

## 2. Estructura del proyecto

```text
tooltrack/
├── assets/
│   ├── assets.go            # embed.FS en la raíz del módulo (go:embed no admite "..")
│   ├── defaults.json        # catálogo base
│   └── locales/{es,en}.json
├── cmd/cli/main.go          # entrypoint (--version, --home)
├── internal/
│   ├── app/                 # lógica compartida por cualquier interfaz
│   │   ├── app.go           # carga, Check, Updatable, Plan, RunUpdates, preferencias
│   │   └── state.go         # State (pantallas) y Status (estado de una herramienta)
│   ├── config/
│   │   ├── config.go        # config.json, Dir(), PathsFor()
│   │   └── defaults.go      # EnsureDefaults: crea data/ y logs/ la primera vez
│   ├── executor/
│   │   ├── command.go       # BuildCommand, SplitCommand
│   │   └── runner.go        # Output (con timeout) y Run (actualizaciones)
│   ├── i18n/i18n.go
│   ├── logger/logger.go     # log con rotación por tamaño
│   ├── tool/
│   │   ├── model.go         # Tool, CheckMode, ParseCatalog
│   │   ├── checker.go       # Checker: resolución, comprobación y UpdateJob
│   │   ├── github.go        # cliente de GitHub Releases
│   │   ├── parser.go        # Version, ParseVersion, HasUpdate
│   │   └── store.go         # Load, Save, Merge, LoadMerged
│   └── ui/
│       ├── tui.go           # modelo de Bubble Tea
│       ├── views/           # funciones de render puras
│       └── styles/          # paleta de Lip Gloss
├── scripts/{build.sh,build.ps1}
├── spec/spec.md
├── Makefile
├── .github/ , .gitlab/ , .gitlab-ci.yml
└── CLAUDE.md, AGENTS.md, README.md, README.es.md
```

## 3. Ubicación de los archivos de usuario

Los datos de usuario **no** viven en el directorio de trabajo:

| Ubicación | Cuándo |
| --- | --- |
| `$TOOLTRACK_HOME` | Si la variable está definida (modo portable). |
| `os.UserConfigDir()/tooltrack` | En otro caso (`%APPDATA%\tooltrack`, `~/.config/tooltrack`, `~/Library/Application Support/tooltrack`). |

El flag `--home <dir>` tiene prioridad sobre ambas. Bajo esa carpeta:

```text
data/config.json
data/tools.json
logs/updater.log
```

La primera ejecución crea las carpetas y escribe `config.json` y `tools.json` desde los valores embebidos. Los archivos existentes nunca se sobrescriben.

## 4. Contratos de datos

### 4.1 `data/config.json`

```json
{
  "language": "es",
  "default_action": "ask",
  "github_token": "",
  "max_concurrent_checks": 5,
  "timeout_seconds": 30
}
```

- `language`: `"es"` o `"en"`.
- `default_action`: `"ask"`, `"update_all"`, `"update_selected"` o `"exclude_selected"`. Se aplica una vez al iniciar (sección 8).
- `github_token`: opcional; sube el límite de la API de 60 a 5 000 peticiones por hora. Si está vacío se usa la variable `GITHUB_TOKEN`. El valor de la variable nunca se escribe en el archivo ni en los logs.
- `max_concurrent_checks` y `timeout_seconds` deben ser al menos 1. `timeout_seconds` aplica a las peticiones HTTP.
- Los campos ausentes toman su valor por defecto; un valor inválido es un error. El archivo se guarda con permisos `0600`.

### 4.2 `data/tools.json` y `assets/defaults.json`

```json
{
  "id": "anaconda",
  "name": "Conda Package Manager",
  "binary": "conda",
  "check_mode": "command",
  "version_command": "conda --version",
  "version_regex": "conda\\s+(\\d+\\.\\d+\\.\\d+)",
  "update_commands": { "default": "conda update -n base -c defaults conda -y" },
  "fallbacks": {
    "windows": ["%USERPROFILE%\\anaconda3\\condabin\\conda.bat"],
    "linux": ["~/anaconda3/bin/conda"]
  },
  "enabled": true
}
```

| Campo | Descripción |
| --- | --- |
| `id`, `name`, `binary` | Identificador único, nombre visible y ejecutable a buscar. |
| `check_mode` | `"github"`: última versión desde GitHub Releases (exige `github_repo`). `"command"`: se consulta a la herramienta (`remote_command`). |
| `github_repo` | `owner/name`. Se rechazan segmentos `.` y `..`. |
| `version_command`, `version_regex` | Comando de versión local y regex cuyo grupo 1 es la versión. |
| `remote_command`, `remote_regex` | Solo en modo `command`. Sin `remote_command` la versión remota queda desconocida. `remote_regex` usa `version_regex` si falta. |
| `update_commands` | Mapa `GOOS` → línea de comandos. La clave `default` aplica a cualquier sistema sin entrada propia. Sin comando para el sistema actual, la herramienta no se ofrece para actualizar. |
| `fallbacks` | Mapa `GOOS` → rutas a probar si el binario no está en `PATH`. Admiten `~`, `%USERPROFILE%`, `%LOCALAPPDATA%` y otras `%VARIABLE%`. |
| `enabled` | Las deshabilitadas no se comprueban ni se muestran. |

`ParseCatalog` valida el catálogo: campos obligatorios, regex compilables, `check_mode` conocido e ids únicos.

### 4.3 Campos de ejecución de `tool.Tool`

No se serializan (`json:"-"`): `ResolvedPath`, `InstalledVersion`, `RemoteVersion`, `HasUpdate`, `IsInstalled`, `LastError`.

### 4.4 Fusión del catálogo

`LoadMerged` combina `tools.json` del usuario con los valores embebidos: las entradas del usuario conservan su orden y ganan sobre las del mismo `id`; los presets nuevos de versiones posteriores se agregan al final. No se escribe nada en disco salvo que se llame a `Save`.

## 5. Ejecución de subprocesos

### 5.1 Resolución del binario (`Checker.Resolve`)

1. `exec.LookPath(binary)`.
2. Si falla, se recorren los `fallbacks` del sistema actual (no solo Windows), expandiendo variables.
3. Cada ruta se valida con `os.Stat`; debe existir y no ser un directorio.

### 5.2 Invocación (`executor.BuildCommand`)

En Windows, un `.bat` o `.cmd` se ejecuta como `cmd.exe /c <ruta> <args…>` (la comparación de extensión ignora mayúsculas). En cualquier otro caso se ejecuta el binario directamente. `BuildCommandContext` añade cancelación por contexto y un `WaitDelay` de 1 s: si el proceso se mata pero un hijo mantiene abiertas las tuberías de salida, `Wait` no se bloquea más allá de ese plazo.

`SplitCommand` divide una línea de comandos en palabras respetando comillas simples y dobles, sin secuencias de escape, para conservar las rutas de Windows con `\`.

### 5.3 Captura y registro

- **Versión:** `executor.Output` ejecuta con tiempo límite (5 s por defecto, `tool.DefaultVersionTimeout`) y captura stdout y stderr juntos, porque algunas herramientas imprimen la versión por stderr.
- **Actualización:** `executor.Run` envía stdout y stderr de cada herramienta al log, nunca a la pantalla. El stdin queda cerrado, de modo que una herramienta que pregunte algo falla en lugar de bloquear la TUI. Las actualizaciones son **secuenciales** por defecto (los gestores de paquetes suelen usar bloqueos globales); `Options.Workers` permite paralelizar.
- **Log:** `logs/updater.log`, líneas `RFC3339 [NIVEL] mensaje`. Rota al superar 5 MB y conserva 3 copias (`updater.log.1` a `.3`). Es seguro entre goroutines y sirve como `io.Writer`.

## 6. Detección y comparación de versiones

### 6.1 GitHub Releases (`tool.GitHubClient`)

- `GET {base}/repos/{owner}/{repo}/releases/latest`; se lee `tag_name`.
- Cabeceras: `User-Agent: Updater-CLI-App`, `Accept: application/vnd.github.v3+json` y, si hay token, `Authorization: Bearer <token>`.
- Errores claros para 404 (sin release publicado), límite de tasa (403 con `X-RateLimit-Remaining: 0`, o 429), 403 sin más y otros códigos. Ningún error incluye el token.

### 6.2 Normalización y comparación (`tool/parser.go`)

1. La versión local se extrae con `version_regex` (grupo 1).
2. `ParseVersion` toma el **primer** número `X`, `X.Y` o `X.Y.Z` de la cadena. Así funcionan `v1.2.3`, `release-1.2.3`, `version/1.2.3` y `bun-v1.1.30`. Las partes que faltan valen 0 y los sufijos (`-rc.1`) se ignoran.
3. La comparación es **lexicográfica**: primero Major, luego Minor y luego Patch.

```text
HasUpdate = Remote > Local   (comparando Major, después Minor, después Patch)
```

La fórmula original (un OR independiente por componente) daba falsos positivos: con local `2.0.0` y remota `1.5.0` marcaba actualización porque `5 > 0`. Hay pruebas que fijan estos casos.

## 7. Comprobación (`Checker.Check` / `CheckAll`)

`Check` rellena los campos de ejecución de una herramienta. Los fallos se guardan en `LastError` y no detienen a las demás:

| Situación | Resultado |
| --- | --- |
| No se encuentra el binario | `IsInstalled = false`, sin error. |
| Falla el comando de versión o la regex | `IsInstalled = true`, `LastError` con el motivo. |
| Falla la consulta remota | Se conserva `InstalledVersion`; `LastError` con el motivo. |
| Modo `command` sin `remote_command` | Versión remota vacía, sin error, nunca marca actualización. |

`CheckAll` omite las deshabilitadas y limita la concurrencia a `max_concurrent_checks`. `UpdateJob` construye la tarea de actualización del sistema actual: si la primera palabra del comando es el `binary` de la herramienta usa `ResolvedPath` (así funciona `conda.bat` fuera de `PATH`); si no, la busca con `LookPath`. Sin comando para el sistema devuelve `ErrNoUpdateCommand`.

## 8. Interfaz TUI

### 8.1 Estados (`app.State`)

| Estado | Pantalla |
| --- | --- |
| `StateLoading` | Spinner mientras se comprueban las herramientas. |
| `StateDashboard` | Tabla alfabética: herramienta, versión local, última versión y estado. |
| `StateActionSelect` | Menú: actualizar todas, seleccionar para actualizar, seleccionar para excluir, preferencias, salir. |
| `StateSelectUpdate` / `StateSelectExclude` | Lista con casillas de las herramientas actualizables. |
| `StateUpdating` | Una línea por herramienta con spinner, ✓ o ✗. |
| `StateFinished` | Resumen de éxitos y fallos; si hubo fallos, muestra la ruta del log. |
| `StateSettings` | Idioma y acción por defecto (se guardan al cambiar). |

Estados de una herramienta (`app.Status`): *Actualizada*, *Nueva versión disponible*, *No instalada* y *Sin verificar* (instalada, pero sin poder determinar la última versión; se muestra el motivo debajo).

### 8.2 Teclas

| Tecla | Acción |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Mover (con vuelta al otro extremo). |
| `Espacio` | Marcar o desmarcar. Cambia el valor en preferencias. |
| `Enter` | Confirmar. |
| `Esc` | Volver. |
| `r` | Volver a comprobar (en el panel). |
| `l` | Abrir el log en el visor predeterminado (solo tras un fallo). |
| `q` / `Ctrl+C` | Salir. Durante una actualización cancela la herramienta en curso y sale cuando termina de cerrarse. |

### 8.3 Acción por defecto

Se aplica una sola vez al iniciar, tras la primera comprobación: `update_all` actualiza de inmediato si hay algo que actualizar; `update_selected` y `exclude_selected` abren la pantalla de selección; `ask` muestra el panel. Solo se ofrecen herramientas instaladas, habilitadas, con actualización disponible y con comando de actualización para el sistema actual.

## 9. Internacionalización

Los archivos `assets/locales/{es,en}.json` son planos (clave → texto) y se embeben con `go:embed`. `i18n.New(lang)` carga el idioma y usa el inglés para las claves que falten; una clave desconocida se devuelve tal cual. Los textos con `%s` se formatean con `T(key, args...)`. Una prueba exige que ambos archivos tengan exactamente las mismas claves.

## 10. Presets (`assets/defaults.json`)

| Herramienta | ID | Binario | Repositorio | Comando de actualización |
| --- | --- | --- | --- | --- |
| Astral uv | `uv` | `uv` | `astral-sh/uv` | `uv self update` |
| Bun | `bun` | `bun` | `oven-sh/bun` | `bun upgrade` |
| Deno | `deno` | `deno` | `denoland/deno` | `deno upgrade` |
| Rustup | `rustup` | `rustup` | `rust-lang/rustup` | `rustup update stable` (también actualiza rustup) |
| Fast Node Manager | `fnm` | `fnm` | `Schniz/fnm` | Linux: `cargo install fnm --force`; macOS: `brew upgrade fnm`; Windows: `winget upgrade Schniz.fnm …` |
| Conda | `anaconda` | `conda` | — (modo `command`, sin versión remota) | `conda update -n base -c defaults conda -y` |
| GitHub CLI | `gh` | `gh` | `cli/cli` | macOS: `brew upgrade gh`; Windows: `winget upgrade GitHub.cli …`; Linux: sin comando automático |

## 11. Compilación, pruebas y CI

- `make check` ejecuta `gofmt`, `go vet` y `go test`. `make cli-all` genera en `dist/` los binarios estáticos `tooltrack-linux-amd64`, `tooltrack-windows-amd64.exe`, `tooltrack-darwin-amd64` y `tooltrack-darwin-arm64`, con `main.Version` inyectado.
- `scripts/build.sh` y `scripts/build.ps1` hacen lo mismo sin `make`.
- GitHub Actions (`.github/workflows/ci.yml`): formato, `go vet` y `go test` en Linux, Windows y macOS, `-race` en Linux y macOS, y compilación cruzada de los cuatro binarios. `.gitlab-ci.yml` replica la comprobación en Linux.
- **GUI (fase posterior):** Wails exige CGO y WebView, y no se puede compilar en cruzado para los tres sistemas desde una sola máquina, así que no entra en este Makefile. `internal/app` no depende de la interfaz para que una GUI pueda reutilizarlo.

## 12. Diferencias con la especificación original

| Tema | Original | Implementado |
| --- | --- | --- |
| `HasUpdate` | OR independiente por componente | Comparación lexicográfica Major, Minor, Patch |
| Limpieza de tags | Quitar prefijos conocidos | Buscar el primer `X.Y.Z` (cubre `bun-v1.1.30`) |
| `go:embed` | Desde `internal/i18n` y `internal/config` | Paquete `assets` en la raíz (las rutas con `..` no compilan) |
| Comando de actualización | Un `update_command` | `update_commands` por sistema, con `default` |
| Rutas alternativas | `windows_fallbacks` | `fallbacks` por sistema, con `~` y variables |
| Versión remota en modo `command` | Sin definir | `remote_command`/`remote_regex` opcionales |
| Carpetas `data/` y `logs/` | Junto al proyecto | Carpeta de configuración del usuario (o `$TOOLTRACK_HOME`) |
| Actualizaciones | Paralelas o secuenciales | Secuenciales por defecto |
| Salida de versión | Solo stdout | stdout y stderr |
| Pantalla de carga | Barra de progreso indeterminada | Spinner |
| Estados de una herramienta | 3 | 4 (se añade *Sin verificar*) |
| Nombre | `updater` | `tooltrack` |
| GUI Wails | En el Makefile | Fase posterior |
| Pruebas | No contempladas | Por paquete, con CI en tres sistemas |

## 13. Pendientes conocidos

- Definir un `remote_command` real para conda (hoy aparece como *Sin verificar*).
- Comando de actualización de `gh` en Linux (depende del gestor de paquetes).
- Los scripts de actualización con `.bat`/`.cmd` se envuelven con `cmd.exe /c`; rutas con caracteres especiales de `cmd` (`& ^ |`) no están cubiertas.
- GUI con Wails.
