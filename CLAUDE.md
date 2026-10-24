# CLAUDE.md

Guía para agentes de código que trabajan en este repositorio.

## Qué es este proyecto

`tooltrack` es una herramienta en Go (CLI/TUI con Bubble Tea, GUI opcional con Wails) que detecta la versión instalada de herramientas de desarrollo (uv, bun, deno, rustup, fnm, conda, gh), consulta la última versión disponible y ejecuta su actualización. Objetivos: Windows 11 (x86_64), Linux (x86_64) y macOS (x86_64 y arm64).

Estado: en desarrollo inicial (CLI/TUI funcional, sin versiones publicadas). La especificación técnica está en `spec/spec.md`; si cambias el comportamiento, actualízala.

## Reglas de ramas (estrictas)

- El repo tiene dos ramas: `main` y `develop`.
- Solo se permite agregar cambios a `develop`. Nunca hacer commit, push ni PR contra `main`.
- Trabaja en una rama derivada de `develop` y abre PRs hacia `develop`.

## Idioma

- Responde al usuario (Josh) en español.
- `README.md` (principal) en inglés; `README.es.md` en español. Mantén ambos sincronizados.
- Código e identificadores en inglés.

## Arquitectura

```
cmd/cli            entrypoint CLI/TUI (--version, --home)
internal/app       lógica compartida: carga, Check, Plan, RunUpdates, preferencias
internal/config    config.json, carpeta de usuario ($TOOLTRACK_HOME) y defaults
internal/executor  ejecución segura de comandos (.exe vs .bat/.cmd), Output y Run
internal/i18n      traducciones es/en con respaldo en inglés
internal/logger    logs/updater.log con rotación por tamaño
internal/tool      modelo, checker, cliente GitHub, parser de versiones, store
internal/ui        Bubble Tea (tui.go), views/ (render puro) y styles/
assets/            defaults.json y locales embebidos (paquete en la raíz)
```

La GUI con Wails (`cmd/gui`) es una fase posterior; mantén `internal/app` independiente de la interfaz.

## Decisiones ya tomadas

- `HasUpdate` se calcula con comparación lexicográfica Major, luego Minor, luego Patch. No usar OR independiente entre componentes.
- `//go:embed` no acepta rutas con `..`: los assets se exponen desde un paquete en la raíz del módulo.
- Binarios CLI estáticos: `CGO_ENABLED=0`.
- Tolerar binarios ausentes o sin permisos; nunca hacer panic por eso.
- En Windows, los `.bat`/`.cmd` se invocan envueltos con `cmd.exe /c`.

## Comandos

```sh
make check      # gofmt, go vet y go test
make cli-all    # binarios estáticos en dist/ (scripts/build.sh sin make)
go test -race ./...
```

## Convenciones

- Código formateado con `gofmt`; pasar `go vet` antes de hacer commit.
- Todo paquete nuevo lleva tests. Usa `httptest` para HTTP, un `Checker` con funciones falsas para el sistema, y no dependas de herramientas reales instaladas. Los tests deben pasar en Linux, Windows y macOS (usa `filepath` en vez de rutas con `/` fijas; salta los que necesiten `sh` con `runtime.GOOS`).
- La CI (`.github/workflows/ci.yml`) corre en los tres sistemas; `.gitlab-ci.yml` replica la comprobación en Linux.
- Nunca escribir el token de GitHub en logs ni commitearlo.
- `data/` y `logs/` se generan en runtime y están ignorados por git.
- Preferencias personales del agente van en `CLAUDE.local.md` (ignorado por git).
