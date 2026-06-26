# WiseLabz — Architecture & Technical Decisions

This document records every significant technical decision made for the WiseLabz project.
It exists so contributors (and future maintainers) can understand **why** the project is structured
the way it is, not just **what** it uses.

---

## Project overview

WiseLabz is a self-hosted, open-source service that automatically generates and maintains
documentation for home labs. Users register their services (Proxmox, Docker/Portainer,
pfSense/OPNsense, self-hosted apps, etc.), connect their APIs, and WiseLabz fetches live data
to generate structured documentation from configurable templates. A diff engine detects changes
since the last sync and either triggers an AI-assisted section update or emits an alert. A web
dashboard displays live service data alongside the generated docs.

**Deployment target:** Docker Compose (self-hosted).  
**Repository:** `github.com/wiselabz/wiselabz`  
**Container registry:** `ghcr.io/wiselabz/wiselabz`

---

## Repository structure

### Monorepo with Go workspaces

All code lives in a single repository. Go workspaces (`go.work`) are configured from day one,
separating the core module from connector packages. This keeps the door open for extracting
connectors into importable standalone libraries later without breaking the internal API.

```
WiseLabz/
├── backend/
│   ├── cmd/server/          # entrypoint — main.go
│   └── internal/
│       ├── connector/       # Connector interface + implementations
│       │   ├── proxmox/
│       │   ├── docker/
│       │   ├── pfsense/
│       │   └── custom/      # example connector for contributors
│       ├── sync/            # scheduler, diff engine
│       ├── doc/             # template engine, renderer
│       ├── api/             # REST handlers + WebSocket
│       ├── auth/            # JWT + OIDC provider
│       └── store/           # repositories, migrations
├── web/
│   └── src/
│       ├── components/      # reusable UI
│       ├── pages/           # dashboard, settings, docs viewer
│       ├── hooks/           # custom React hooks (incl. useWebSocket)
│       ├── store/           # Zustand global state
│       └── api/             # generated REST client
├── deploy/
│   ├── docker-compose.yml
│   └── config.example.yaml
├── docs/                    # ADRs, connector guides, this file
├── .github/
│   └── workflows/           # CI/CD pipelines
├── Makefile
├── CONTRIBUTING.md
└── go.work
```

---

## Technology stack

### Backend

| Concern          | Decision                              | Rationale                                                                                                       |
|------------------|---------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| Language         | Go                                    | performance, single binary, strong stdlib                                                                       |
| HTTP router      | `chi`                                 | idiomatic middleware composition, no magic, lightweight                                                         |
| WebSocket        | `gorilla/websocket`                   | de facto standard in Go, well-maintained                                                                        |
| Auth             | Local JWT + OIDC via `coreos/go-oidc` | single session logic covers both local and external providers                                                   |
| Database queries | `sqlc`                                | type-safe generated code, no ORM, fully auditable SQL                                                           |
| Migrations       | `golang-migrate`                      | simple, supports SQLite and PostgreSQL, low contributor friction                                                |
| Config           | `viper`                               | YAML file for traditional installs; all values overridable via WISELABZ_ env vars, friendly to PaaS deployments |
| Logging          | `slog` (stdlib)                       | no external dependency, JSON output in production                                                               |

### Frontend

| Concern          | Decision                                                  | Rationale                                                                                                                                                            |
|------------------|-----------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Framework        | React + Vite                                              | mature ecosystem, fast HMR in dev                                                                                                                                    |
| State management | Zustand                                                   | minimal boilerplate, works cleanly alongside React Query                                                                                                             |
| Server state     | React Query                                               | caching, background refetch, loading/error states for REST data                                                                                                      |
| Styling          | Tailwind CSS                                              | productive, dark mode built-in, low barrier for contributors                                                                                                         |
| API client       | generated via `orval` (OpenAPI → typed React Query hooks) | contract-first from `docs/openapi.yaml`; frontend/backend types stay in sync, and MSW mocks generated from the same spec let the frontend build ahead of the backend |

### Infrastructure

| Concern            | Decision                                        | Rationale                                                      |
|--------------------|-------------------------------------------------|----------------------------------------------------------------|
| Deployment         | Docker Compose                                  | `docker compose up` is the entire install story                |
| Build              | Vite SPA embedded into Go binary via `go:embed` | single binary, single Docker service, no Nginx needed          |
| Container registry | GHCR (`ghcr.io`)                                | native GitHub integration, free for open-source                |
| CI                 | GitHub Actions                                  | lint + test + build on every PR; multi-arch Docker push on tag |

---

## Build pipeline

The production build uses a multi-stage Dockerfile:

1. **Stage 1 (`web`)** — Node.js Alpine image runs `npm ci` and `npm run build`, producing a
   static SPA in `/web/dist`.
2. **Stage 2 (`builder`)** — Go image copies the SPA output into `internal/web/dist/` (where
   `go:embed` expects it) and compiles a static binary with `CGO_ENABLED=0`.
3. **Stage 3 (`runtime`)** — Minimal scratch or distroless image receives only the compiled
   binary. No Node.js, no Go toolchain reaches the runtime image.

In development, Vite's dev server runs separately with a proxy forwarding `/api` and `/ws`
requests to the Go backend. The `go:embed` path only activates in the production build.

---

## Development workflow

### Branching

| Prefix      | Use                                   |
|-------------|---------------------------------------|
| `feat/`     | new features                          |
| `fix/`      | bug fixes                             |
| `docs/`     | documentation only                    |
| `chore/`    | tooling, dependencies, config         |
| `refactor/` | code changes with no behaviour change |

Pull requests are required for all work, including between the two core maintainers.
This creates a reviewable decision trail that external contributors can follow.

### Commit conventions

All commits follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
feat(connector): add Proxmox VM snapshot support
fix(auth): handle expired refresh token on WS reconnect
docs(connector): add guide for writing a custom connector
chore(ci): pin golangci-lint to v1.57
```

### Commit hooks (`lefthook`)

Two hooks are enforced locally via `lefthook`:

- **`pre-commit`** — runs `gofmt`, `golangci-lint`, and fast unit tests
- **`commit-msg`** — validates the Conventional Commits format

Both hooks also run in CI, so a passing local environment guarantees a passing pipeline.

### Code quality

- Go: `gofmt` + `golangci-lint` with an explicit `.golangci.yml` config checked into the repo
- TypeScript: ESLint + Prettier
- Both block merges if they fail — no exceptions

---

## Authentication design

WiseLabz supports two authentication modes, sharing the same session logic:

**Local mode** — username/password stored in the database (bcrypt). On login, issues a
short-lived JWT access token and a longer-lived refresh token. No external dependency required.

**OIDC mode** — the user sets `auth.oidc.issuer_url` in `config.yaml` pointing to their
provider (Authentik, Authelia, Keycloak, etc.). The login flow delegates to the provider;
WiseLabz receives and validates the ID token via `coreos/go-oidc`. The session mechanism
(JWT issuance, refresh) is identical to local mode.

Switching between modes requires only a config change and restart — no code path diverges.

### OIDC provider configuration (decided 2026-06-25: file-defined, app toggles only)

OIDC providers are defined **entirely in config file / env** — `issuer_url`,
`client_id`, and `client_secret` never transit the HTTP API, never live in the app
database, and are never editable through the UI. This is deliberate: an API that could
rewrite `issuer_url` would be an auth-bypass vector (an attacker repointing the issuer
to an IdP they control logs in as admin), and storing the secret in the DB only adds an
exfiltration surface without removing the env secret you need to encrypt it.

The in-app Settings → Auth page is therefore **read-only for providers plus a few safe
toggles**:
- View detected providers (issuer, clientId, `secretConfigured: yes/no`) — never the secret.
- Enable/disable a provider (`PUT /api/auth/providers/{id}/enabled`) — persisted as a DB flag; the definition stays in config.
- Toggle local login and set access/refresh token TTLs (`PUT /api/auth/config`).

This supersedes the frontend plan's original §8.6 ("add/edit OIDC provider via UI"),
which has been amended accordingly. Endpoint contract: `docs/openapi.yaml`
(`/auth/config`, `/auth/providers/{providerId}/enabled`).

---

## Connector interface

Every service integration implements the same interface. The core knows nothing about
Proxmox or Docker specifically — only about `Connector`.

```go
type Connector interface {
    Name()     string
    Fetch(ctx context.Context) (ServiceSnapshot, error)
    Validate() error
}
```

Each connector lives in its own package under `internal/connector/`. Contributors adding a
new integration only need to implement this interface and register the connector — no changes
to the core sync or doc engine are required.

---

## API design

- **REST** — all data reads and state mutations. Schema exported as OpenAPI, used to generate the TypeScript REST client in `web/src/api/`.
The API itself is implemented entirely in Go.
- **WebSocket** — live updates only (service status changes, diff engine alerts, sync progress).
  No mutations over WS. A single `/ws` endpoint streams typed JSON events to the dashboard.

---

## Database

- **SQLite** — default for development and small single-user installs. Zero setup, embedded.
- **PostgreSQL** — recommended for multi-user or higher-reliability deployments. Configured via
  `config.yaml`.

`sqlc` generates type-safe Go code from plain `.sql` query files. Migrations are managed by `golang-migrate` and run automatically on startup.
Both SQLite and PostgreSQL dialects are supported.  
Database connection is configured via environment variables (recommended for PaaS and container platforms like Dokploy, Coolify, and Portainer)
or via `config.yaml` for traditional installs. Environment variables always take precedence over the config file. The `WISELABZ_` prefix is used
for all env overrides (e.g. `WISELABZ_DB_DSN`, `WISELABZ_DB_DRIVER`).  
`golang-migrate` and run automatically on startup. Both SQLite and PostgreSQL dialects are
supported.

---

## AI module

The AI doc generation feature is **opt-in and provider-agnostic**. When enabled, the diff
engine passes changed service data and the current doc section to the configured LLM API.
The user can choose their provider (Anthropic, OpenAI, or a local model via Ollama) in
`config.yaml`. When disabled, the diff engine emits alerts instead of auto-updating sections.

---

## ADR index

Architectural Decision Records are stored in `docs/adr/`. Each significant decision that
required evaluation of alternatives gets its own numbered ADR file (e.g. `0001-monorepo.md`).
This file records the _outcome_ of each decision; the ADRs record the _reasoning_.

---

_Last updated: 2026-06-25_
