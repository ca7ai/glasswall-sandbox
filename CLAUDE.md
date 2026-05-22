# CLAUDE.md

Guidelines for working on `glasswall` (GlassWall Sandbox).

## Project Overview
`glasswall` is a lightweight, local-first agentic execution sandbox. It runs untrusted commands inside isolated environments (using native macOS `sandbox-exec` profiles and ephemeral workspace mirroring) to track filesystem diffs, network activity, and execution results. All runs are cataloged in a local SQLite database and summarized in token-optimized formats for AI agents.

## Tech Stack
- **Language:** Go (1.26.3+)
- **Database:** SQLite (modernc.org/sqlite or go-sqlite3)
- **Framework:** Cobra CLI
- **Sandboxing Engine:** Native macOS `sandbox-exec` + virtualized workspace diffing

## Build & Run Commands
- Initialize dependencies: `go mod tidy`
- Build the binary: `go build -o glasswall ./cmd/glasswall`
- Run local tests: `go test ./...`
- Linting: `golangci-lint run`

## CLI Interface
- `glasswall run "<command>"`: Spawns the command inside the sandbox, tracks and diffs all file operations and prints an agent-native summary.
- `glasswall runs`: Lists history of all sandboxed runs from SQLite.
- `glasswall diff <run-id>`: Shows the visual diff of modified/created files for a run.
- `glasswall clean`: Cleans up temporary sandbox workspaces.
