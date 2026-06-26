# Copilot Instructions for Opengist

## Architecture Overview
- **Core Concept**: Opengist is a Git-powered pastebin. Every "gist" is a bare Git repository stored on disk.
- **Backend**: Go (Golang) using the [Echo](https://echo.labstack.com/) web framework.
- **Frontend**: Vite-based build system with Tailwind CSS and TypeScript. Templates use Go's `html/template`.
- **Database**: GORM is used for metadata (users, gist descriptions, topics), while the actual content resides in Git repos.
- **Git Storage**: Repositories are stored in `~/.opengist/repos` (by default). See `internal/git/commands.go`.

## Key Patterns & Conventions
- **Routing**: Defined in `internal/web/server/router.go`. Handlers are grouped by feature in `internal/web/handlers/`.
- **Context**: Use `*context.Context` (found in `internal/web/context`) in handlers instead of the raw `echo.Context`. It provides helper methods like `ctx.Html()`, `ctx.SetData()`, and `ctx.ErrorRes()`.
- **Templating**: 
    - Templates are in `templates/`.
    - Data is passed via `ctx.SetData(key, value)`.
    - Custom functions for templates are defined in `internal/web/server/renderer.go`.
- **Database**: Models are in `internal/db/`. Use GORM conventions.
- **Git Operations**: Wrap Git CLI commands in `internal/git/commands.go`. Avoid direct `os/exec` for git unless necessary.

## Developer Workflow
- **Build All**: `make` (installs deps, builds frontend, then backend).
- **Frontend Only**: `npm run build` or `npx vite -c public/vite.config.js build`.
- **Backend Only**: `go build -tags fs_embed .`. Note the `fs_embed` tag for production builds.
- **Development/Watch Mode**: `make watch` runs both frontend and backend in watch mode.
- **Testing**: `make test` (defaulting to sqlite). Use `OPENGIST_TEST_DB=postgres make test` to test other DBs.
- **Translations**: Checked via `make check-tr`.

## Critical Files
- `internal/web/server/router.go`: The source of truth for all web endpoints.
- `internal/web/context/context.go`: The extended Echo context used everywhere.
- `internal/db/gist.go`: The main Gist model and its associated logic.
- `internal/git/commands.go`: Logic for interacting with the underlying Git repositories.
- `public/vite.config.js`: Frontend build configuration.

## Common Tasks
- **Adding a Route**: Add to `router.go`, create a handler in `internal/web/handlers/`, and a template in `templates/pages/`.
- **Adding a Template Function**: Register it in `internal/web/server/renderer.go`'s `setFuncMap`.
- **Modifying the Schema**: Update models in `internal/db/`. GORM handles migrations automatically for most changes (see `internal/db/migration.go`).
