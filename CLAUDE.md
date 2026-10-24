# CLAUDE.md

Guía para agentes de código que trabajan en este repositorio.

## Qué es este proyecto

`tooltrack` es una herramienta en Go (CLI/TUI con Bubble Tea, GUI opcional con Wails) que detecta la versión instalada de herramientas de desarrollo (uv, bun, deno, rustup, fnm, conda, gh), consulta la última versión disponible y ejecuta su actualización. Objetivos: Windows 11 (x86_64), Linux (x86_64) y macOS (x86_64 y arm64).

Estado: en desarrollo inicial. La especificación técnica vivirá en `spec/spec.md` cuando el plan esté cerrado.

## Reglas de ramas (estrictas)

- El repo tiene dos ramas: `main` y `develop`.
- Solo se permite agregar cambios a `develop`. Nunca hacer commit, push ni PR contra `main`.
- Trabaja en una rama derivada de `develop` y abre PRs hacia `develop`.

## Idioma

- Responde al usuario (Josh) en español.
- `README.md` (principal) en inglés; `README.es.md` en español. Mantén ambos sincronizados.
- Código e identificadores en inglés.

## Arquitectura prevista

```
cmd/cli            entrypoint CLI/TUI
cmd/gui            entrypoint GUI (Wails, fase posterior)
internal/app       ciclo de vida y máquina de estados
internal/config    config.json y defaults
internal/executor  ejecución segura de comandos (.exe vs .bat/.cmd)
internal/i18n      traducciones es/en
internal/logger    logs/updater.log con rotación
internal/tool      modelo, checker, cliente GitHub, parser de versiones, store
internal/ui        Bubble Tea, vistas y estilos Lip Gloss
assets/            defaults.json y locales embebidos con go:embed
```

## Decisiones ya tomadas

- `HasUpdate` se calcula con comparación lexicográfica Major, luego Minor, luego Patch. No usar OR independiente entre componentes.
- `//go:embed` no acepta rutas con `..`: los assets se exponen desde un paquete en la raíz del módulo.
- Binarios CLI estáticos: `CGO_ENABLED=0`.
- Tolerar binarios ausentes o sin permisos; nunca hacer panic por eso.
- En Windows, los `.bat`/`.cmd` se invocan envueltos con `cmd.exe /c`.

## Comandos

```sh
go build ./...
go vet ./...
go test ./...
gofmt -l .
```

## Convenciones

- Código formateado con `gofmt`; pasar `go vet` antes de hacer commit.
- Todo paquete nuevo lleva tests (parser de versiones, checker, `BuildCommand`, config y cliente GitHub como mínimo). Usa `httptest` para HTTP y no dependas de herramientas reales instaladas.
- Nunca escribir el token de GitHub en logs ni commitearlo.
- `data/` y `logs/` se generan en runtime y están ignorados por git.
- Preferencias personales del agente van en `CLAUDE.local.md` (ignorado por git).
