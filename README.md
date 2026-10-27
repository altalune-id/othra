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

3. Start Postgres, Redis, NATS, Garage and Mailpit. On its first start the compose Postgres creates the Othra roles (see [Database roles](#database-roles)). To start only the database: `podman compose up -d postgres`.

   ```sh
   docker compose up -d
   ```

   A Postgres volume created before the roles existed is not provisioned again. Drop it once with `docker compose down -v` (this deletes the dev data), or run `scripts/db/provision.sh` against it.

4. Create your config. `config.example.yml` already points at the compose Postgres as `othra_service` and `othra_migrator`; generate the two secrets with `openssl rand -hex 32`.

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

## Database roles

Othra's tenant tables use Postgres row-level security, so the app connects as a role that cannot bypass it.
`scripts/db/provision.sh` creates the database and three roles on any Postgres (self-hosted, RDS, Neon and others).

| Role             | Attributes         | Used by                                              | Purpose                                                                                   |
| ---------------- | ------------------ | ---------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `othra_owner`    | NOLOGIN, BYPASSRLS | `postgres.migrator.role` (the migrator's `SET ROLE`) | owns the schema, the tables and the `SECURITY DEFINER` lookups; nobody logs in as it      |
| `othra_migrator` | LOGIN, NOBYPASSRLS | `postgres.migrator.dsn`                              | runs migrations as the owner; use a direct (not pooled) connection                        |
| `othra_service`  | LOGIN, NOBYPASSRLS | `postgres.dsn` (and `postgres.reader.dsn`)           | the running app; reads and writes rows under row-level security, owns nothing, no members |

```sh
ADMIN_URL='postgres://admin:...@db.example.com:5432?sslmode=require' \
MIG_PW=gen SVC_PW=gen scripts/db/provision.sh
```

- The admin needs `CREATEROLE` and either `SUPERUSER` or `BYPASSRLS`, because it hands `BYPASSRLS` to `othra_owner`.
- Run it without the variables to be asked for each value. It is safe to re-run, and a re-run sets the passwords you pass and repairs role attributes and grants.
- `gen` creates a password and writes it to `othra-db-credentials.env` (mode 0600). Passwords are never printed.
- `SCHEMA` and `TABLE_PREFIX` must match `postgres.schema` and `postgres.tablePrefix`.
- It ends by running `scripts/db/verify.template.sql`, which fails loudly on any drift; it prints the command to run that check again later.
- The app logs in as `othra_service` directly; Othra never runs `SET ROLE` on the app pool, because behind a transaction-mode pooler it would not stick. `postgres.role` is optional: when set, it names the role the DSN must log in as, and boot refuses any other.
- At boot, after migrating, Othra refuses to start if its role can bypass row-level security (superuser, BYPASSRLS, a table or schema owner, or a member of one) or if an org-owned table is not forced. In development only, `db.rls.allowBypass: true` turns the role check into a warning.
- Local dev: the compose Postgres runs the same script on its first start (`docker/postgres/init`), with dev-only passwords from `compose.yaml`.

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

### File storage (S3)

Uploads are off unless `storage.driver` is set. With `storage.driver: s3`, the browser uploads straight to an
S3-compatible bucket with a presigned PUT; Othra never receives the file. `s3.*` is only the connection, and one
bucket can serve several Altalune apps (`storage.app` is the second key segment).

Cloudflare R2:

```sh
OTHRA_STORAGE_DRIVER=s3
OTHRA_STORAGE_PUBLIC_BASE_URL=https://files.altalune.id     # the bucket's custom domain
OTHRA_STORAGE_IMAGE_TRANSFORM=cloudflare                    # optional: resized images via /cdn-cgi/image/
OTHRA_S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
OTHRA_S3_REGION=auto
OTHRA_S3_BUCKET=altalune-files
OTHRA_S3_ACCESS_KEY_ID=...        # an R2 API token scoped to the bucket
OTHRA_S3_SECRET_ACCESS_KEY=...
```

- **CORS:** the bucket needs a rule for each console origin, with methods `PUT`, `GET`, `HEAD`, allowed header
  `content-type` and exposed header `etag`. Without it the browser's preflight fails and uploads never start.
- **Browser endpoint:** presigned URLs are signed for one exact host. Set `s3.browserEndpoint` only when browsers
  reach the S3 API on another host than Othra does (compose: Othra calls `http://garage:3900`, the browser
  `http://localhost:3900`). It must be exactly the host the browser uses, or every upload gets 403.
- **Public reads:** files are read from `storage.publicBaseURL`, never through Othra. On R2, attach the custom
  domain, disable the `r2.dev` URL, and add a WAF custom rule that blocks paths starting with `/private/` on that
  domain; private files are only ever read through short-lived signed URLs.

Locally, `docker compose up -d` runs Garage (`dxflrs/garage`) and `garage-init`, which creates the bucket
`altalune-files`, a fixed dev key and the CORS rule for `http://localhost:${OTHRA_PORT:-5150}`. Public files are
served at `http://altalune-files.localhost:3902`. To run Othra on the host against it:

```sh
OTHRA_STORAGE_DRIVER=s3
OTHRA_STORAGE_PUBLIC_BASE_URL=http://altalune-files.localhost:3902
OTHRA_S3_ENDPOINT=http://localhost:3900
OTHRA_S3_REGION=garage
OTHRA_S3_BUCKET=altalune-files
OTHRA_S3_ACCESS_KEY_ID=GK000000000000000000000001
OTHRA_S3_SECRET_ACCESS_KEY=0000000000000000000000000000000000000000000000000000000000000001
```

### Signing keys

Every project signs its tokens with its own RS256 keys, sealed at rest with `secrets.encryptionKey`, which is
therefore required in every mode. A rotation publishes the new key first and makes it sign after
`keys.rotation.prePublish` (10 minutes); the old key stays published for `keys.rotation.overlap` (7 days). Project
editors rotate from the console or with `othra project keys rotate <project>`; `--now` makes the new key sign at
once, and verifiers that cached the old key set reject new tokens until they fetch it again.

Changing `secrets.encryptionKey` makes every sealed private key unreadable (the published public keys stay
servable). After a key change, run `othra project keys rotate <project> --now` for every project.

### Console sign-in and break-glass

Admins sign in at `/login` with a password (self-hosted, when `auth.password.enabled` is on) or with Continue with
Othra, which goes through the deployment's own `/p/system` issuer. Cloud has no passwords.

Locked out? Both commands need database access, like `othra serve`:

- `othra admin set-password --email <email> --password-stdin` sets a console password and signs out every session.
  It is refused when passwords are off.
- `othra admin sign-in-link <email>` prints a one-time sign-in link for an existing console account, or for the first
  owner an install with passwords off invited. It works once, for the magic-link TTL (15 minutes by default), and needs
  no email server. It needs `http.baseURL`. Treat it like a password.

Use one public URL (`http.baseURL`) per deployment. Every `othra serve` boot points the console's sign-in back at it, so
instances with different URLs would keep undoing each other. Operator commands never change it.

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
