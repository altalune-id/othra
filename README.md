# Othra

Othra is Altalune's identity provider, on its way to a full OAuth 2.1 / OpenID Connect authorization server with multi-tenant orgs and projects. It is one Go binary that serves:

- an API (Connect RPC)
- an MCP server
- a CLI
- a web console

Today it is the platform base: orgs, projects, members, invites, API keys, webhooks and an admin console, with an API, a CLI and MCP admin tools. Sign-in for apps (OAuth clients, tokens, hosted login) is not built yet.

## Requirements

- Go 1.26.9
- Node 24.15 or newer
- pnpm 10 (`corepack enable` picks the version pinned in `package.json`)
- Docker, for local dependencies and the integration tests

## Set up the repo

1. Clone the repo and install the JavaScript dependencies. This also installs the git hooks.

   ```sh
   git clone <repo-url> othra && cd othra
   corepack enable
   pnpm install
   ```

2. Turn on signed commits and the commit template. Pushes with unsigned commits are refused (see [.husky/README.md](.husky/README.md)).

   ```sh
   git config commit.gpgsign true
   git config commit.template .gitmessage.txt
   ```

3. Start Postgres, Redis, NATS and Mailpit.

   ```sh
   docker compose up -d
   ```

4. Create your config. Point both DSNs at the compose Postgres (`postgres://postgres:postgres@localhost:5432/othra?sslmode=disable`) and generate the two secrets with `openssl rand -hex 32`.

   ```sh
   cp config.example.yml config.yml
   ```

   Every key can also be set from the environment: see `.env.example` and the full list in `config.reference.yml`.

5. Build the embedded UI (console, MCP views, sign-in page assets; the console and sign-in assets also get brotli and gzip copies), then run the server.

   ```sh
   make ui
   make run
   ```

6. Open the one-time install link printed in the log, create the first owner account, and you land in the console at `http://localhost:5150/console/`.

## Provision a production database

`scripts/db/provision.sh` creates the database and three roles on any Postgres (self-hosted, RDS, Neon and others).
It is safe to re-run, and a re-run sets the passwords you pass.

| Role             | Purpose                                             |
| ---------------- | --------------------------------------------------- |
| `othra_owner`    | owns the schema and tables; nobody logs in as it    |
| `othra_migrator` | runs migrations as the owner                        |
| `othra_service`  | used by the running app; reads and writes rows only |

```sh
ADMIN_URL='postgres://admin:...@db.example.com:5432?sslmode=require' \
MIG_PW=gen SVC_PW=gen scripts/db/provision.sh
```

- Run it without the variables to be asked for each value.
- `gen` creates a password and writes it to `othra-db-credentials.env` (mode 0600). Passwords are never printed.
- The script prints the matching `postgres.dsn`, `postgres.migrator.dsn`, `postgres.migrator.role` and `postgres.schema` settings.

## Run with Docker

You don't need Go or Node to run Othra. The image holds the same binary with the UI embedded.

```sh
docker build -t othra .
docker run --rm -p 5150:5150 \
  -e OTHRA_POSTGRES_DSN=postgres://... \
  -e OTHRA_POSTGRES_MIGRATOR_DSN=postgres://... \
  -e OTHRA_HTTP_STATE_SECRET=$(openssl rand -hex 32) \
  -e OTHRA_SECRETS_ENCRYPTION_KEY=$(openssl rand -hex 32) \
  othra
```

- The image runs `serve` by default. Pass any other command after the image name, for example `docker run --rm othra version`.
- To run the whole stack in containers: `docker compose --profile app up -d --build`.

### Queue (NATS)

The queue is off unless `queue.enabled` is true; `queue.driver` picks what carries it (`nats`, the only driver).
`nats.*` is only the connection, and Othra opens it only when a feature uses it. Othra connects as user `othra` in
its own account, `OTHRA`, as on the shared Altalune NATS ([altalune-id/nats](https://github.com/altalune-id/nats)).
Every app uses the same stream names, so the account is what keeps apps apart: never share one between apps.

```sh
OTHRA_QUEUE_ENABLED=true
OTHRA_NATS_URL=nats://<nats-service>.railway.internal:4222
OTHRA_NATS_USER=othra
OTHRA_NATS_PASSWORD=...   # the server's OTHRA_NATS_PASSWORD, at least 16 characters
```

The credentials may also go in the URL (`nats://othra:<password>@host:4222`). `nats.token` is for a NATS server
without accounts; set it or the user and password, not both. Deploy the server on private networking only, with a
volume at `/data` (see the shared server's README). Locally, `docker compose up -d` runs NATS with the accounts in
`docker/nats/nats-server.conf` (`OTHRA` and `OTHER`, dev passwords of 16 characters or more). To run Othra on
the host against it: `OTHRA_NATS_URL=nats://localhost:4222`, `OTHRA_NATS_USER=othra` and
`OTHRA_NATS_PASSWORD=othra-dev-nats-password` (the compose default).

## Everyday commands

| Command                      | What it does                                                         |
| ---------------------------- | -------------------------------------------------------------------- |
| `make check`                 | vet, every linter, unit tests                                        |
| `make cover`                 | unit + integration tests; fails when a package is under 85% coverage |
| `make ui-test`               | UI tests                                                             |
| `make generate`              | regenerate protos, MCP tools, scopes, error codes and templ views    |
| `make fmt` / `pnpm lint:fix` | format Go / fix JS, TS, Vue, JSON, Markdown and CSS                  |
| `make help`                  | every target                                                         |

## Docs

- [AGENTS.md](AGENTS.md): the rules for contributors and coding agents
- [docs/](docs/README.md): architecture and how-to recipes

## License

[MIT](LICENSE)
