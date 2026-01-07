# Repository Guidelines

## Project Structure & Module Organization
- `cmd/bambu-cli/` — CLI entry point and command handlers (`main.go`).
- `internal/printer/` — MQTT/FTPS/camera protocol clients, payload builders, and printer-specific helpers.
- `internal/config/` — config loading, merging, and XDG/project path helpers.
- `internal/output/` and `internal/ui/` — output formatting and confirmation/TTY utilities.
- `README.md` — usage and configuration overview.

## Build, Test, and Development Commands
- `go build -o bambu-cli ./cmd/bambu-cli` — build the CLI binary.
- `go run ./cmd/bambu-cli --help` — run locally without installing.
- `go test ./...` — run all Go tests (currently no test files; add as you go).

## Coding Style & Naming Conventions
- Follow standard Go formatting (`gofmt`) and idioms.
- Use lowerCamelCase for local variables, UpperCamelCase for exported types/functions.
- Keep CLI flags kebab-case (e.g., `--access-code-file`).
- Prefer small, focused functions; keep protocol-specific logic in `internal/printer/`.

## Testing Guidelines
- Framework: Go’s built-in `testing` package.
- Name test files `*_test.go` and place alongside the code under test.
- Cover protocol parsing and payload generation first (pure functions are easiest to test).
- Run tests with `go test ./...`.

## Commit & Pull Request Guidelines
- No git history is present in this repo, so no established commit convention.
- Use clear, imperative commit messages (e.g., “Add camera snapshot command”).
- PRs should include: summary, rationale, test command(s) run, and sample output for CLI changes.

## Security & Configuration Tips
- Never accept access codes via CLI flags; use `--access-code-file` or `--access-code-stdin`.
- Default config lives at `~/.config/bambu/config.json` with optional project config `./.bambu.json`.
- Network defaults: MQTT 8883, FTPS 990, Camera 6000. Use `bambu-cli doctor` to verify reachability.
