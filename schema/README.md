# schema

Postgres migrations for Othra, run by the app-agnostic `migrator` package
(goose). Files are templates: `{{.Schema}}`, `{{.TablePrefix}}`, `{{.Role}}`.
`Config.Schema` defaults to `public`; a non-empty `Config.Role` runs `SET ROLE`
on the migration connection.

Roles (`scripts/db/provision.sh`): migrations run as `othra_migrator` with
`SET ROLE othra_owner`, so `othra_owner` (NOLOGIN, BYPASSRLS) owns the schema,
every table, sequence and function, and `goose_db_version`. The app logs in as
`othra_service` (NOBYPASSRLS, no memberships, owns nothing), which gets
`SELECT, INSERT, UPDATE, DELETE` on tables, `USAGE` on sequences and `EXECUTE`
on functions through the owner's default privileges, read-only access to
`goose_db_version`, and no `CREATE` or `TEMP`. Functions are never executable by
`PUBLIC`. `scripts/db/verify.template.sql` checks all of this. Integration tests
get the same split from `pgtest.MigratedRLS` (`othra_test_owner`,
`othra_test_service`).

```mermaid
erDiagram
    orgs ||--o{ projects : has
    orgs ||--o{ api_keys : has
    projects |o--o{ api_keys : "scopes (project keys)"
    api_keys ||--o{ api_key_projects : grants
    projects ||--o{ api_key_projects : "granted to"
    orgs ||--o{ outbox_entries : has
    projects ||--o{ outbox_entries : has
    orgs ||--o{ webhook_endpoints : has
    projects ||--o{ webhook_endpoints : has
    webhook_endpoints ||--o{ webhook_deliveries : logs
    users ||--o{ memberships : has
    orgs ||--o{ memberships : has
    users |o--o{ orgs : "created_by"
    users |o--o{ api_keys : "created_by (owner of personal keys)"
    users ||--o{ sessions : owns
    users ||--o{ password_resets : requests
    orgs ||--o{ invites : has
    users ||--o{ terms_acceptances : accepted
    legal_documents ||--o{ terms_acceptances : "terms, privacy"
    users |o--o{ invites : "invited_by, accepted_by"
    orgs ||--o{ files : has
    projects ||--o{ signing_keys : signs
    projects ||--o{ oauth_clients : registers
    oauth_clients ||--o{ oauth_client_secrets : "authenticates with"
    projects ||--o{ resource_servers : registers
    oauth_clients ||--o{ oauth_client_resources : "linked to"
    resource_servers ||--o{ oauth_client_resources : "linked from"
    projects ||--o{ end_users : "signs in"
    projects ||--o{ browser_sessions : has
    browser_sessions ||--o{ user_sessions : holds
    end_users ||--o{ user_sessions : "signed in as"
    end_users ||--o{ end_user_terms : accepted
    user_sessions |o--o{ end_user_terms : "accepted in"
    projects ||--o{ magic_links : sends
    browser_sessions |o--o{ magic_links : "redeemed by"
    oauth_clients ||--o{ grant_families : "issued to"
    end_users ||--o{ grant_families : "signed in as"
    user_sessions |o--o{ grant_families : "signed in through"
    resource_servers |o--o{ grant_families : "for (M6)"
    oauth_clients ||--o{ authorization_codes : "issued to"
    end_users ||--o{ authorization_codes : "signed in as"
    user_sessions |o--o{ authorization_codes : "signed in through"
    grant_families |o--o{ authorization_codes : "redeemed into"
    resource_servers |o--o{ authorization_codes : "for (M6)"
    grant_families ||--o{ access_tokens : holds
    grant_families ||--o{ refresh_tokens : holds
    resource_servers |o--o{ access_tokens : "audience of"
    end_users ||--o{ scope_consents : approved
    oauth_clients ||--o{ scope_consents : "approved for"
    resource_servers |o--o{ scope_consents : "approved on"
    files |o--o{ projects : "logo_file_id"

    orgs {
        uuid id PK
        text public_id UK
        text slug UK
        text name
        bool is_system "one row at most"
        uuid created_by FK "users, NULL in basic mode"
        timestamptz created_at
        timestamptz updated_at
    }
    projects {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text slug "unique per org"
        text name
        text issuer_handle UK "global; the issuer URL /p/<handle>"
        timestamptz issuer_handle_locked_at "set by the first OAuth client or resource server"
        text display_name
        text primary_color "#rrggbb or empty"
        text support_email
        text terms_url "empty turns the end-user terms gate off"
        text privacy_url
        int terms_version "0 until the first terms link"
        timestamptz terms_published_at
        bool signin_email_link
        uuid logo_file_id FK "(org_id, logo_file_id) -> files, NULL when no logo"
        uuid created_by
        timestamptz created_at
        timestamptz updated_at
    }
    signing_keys {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text kid UK "RFC 7638 thumbprint of the public key"
        text alg "RS256"
        jsonb public_jwk "the published JWK"
        bytea private_key_enc "PKCS#8 DER, sealed with AAD signing_key:<project_id>:<kid>; NULL once retired"
        text status "next, active, retiring or retired"
        timestamptz created_at
        timestamptz activates_at "set while next"
        timestamptz activated_at
        timestamptz retire_after "set while retiring"
        timestamptz retired_at
    }
    oauth_clients {
        uuid id PK
        text public_id UK "also the OAuth client_id"
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text name
        text client_type "confidential (web) or public (native)"
        text application_type "web or native"
        text token_endpoint_auth_method "client_secret_basic or client_secret_post (web), none (native)"
        text_array grant_types "authorization_code, plus refresh_token"
        text_array allowed_scopes
        text_array redirect_uris "1 to 20, canonical, matched exactly"
        text_array post_logout_redirect_uris "web only, up to 20"
        text consent_policy "require or implicit"
        bool pkce_required "false only for a confidential client, which then needs a nonce"
        text registration_kind "registered, or cimd for a shadow of a metadata document"
        text metadata_url UK "the CIMD client_id; per project; cimd only"
        timestamptz metadata_fetched_at "cimd only"
        timestamptz metadata_expires_at "cimd only; refetched after"
        text client_uri "https, from the document"
        timestamptz created_at
        timestamptz updated_at
    }
    oauth_client_secrets {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid project_id FK
        uuid client_id FK "(org_id, project_id, client_id) -> oauth_clients"
        text secret_hash UK "sha256$<hex>; never the secret"
        text hint "last 4 characters"
        timestamptz created_at
        timestamptz expires_at "NULL on the current secret; set on the old one during a rotation's grace"
    }
    resource_servers {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text name
        text identifier "the audience; canonical, unique per project, never changes"
        text_array scopes "1 to 64 RFC 6749 scope tokens"
        text verification_state "pending, verified or failed"
        timestamptz verified_at "set once verified"
        timestamptz last_checked_at
        jsonb last_error "why the latest check failed; NULL after a pass"
        int consecutive_failures
        bool trusted "trust without verification, self-hosted only"
        timestamptz trusted_at "set exactly when trusted"
        uuid trusted_by "the person who set trust"
        bool accept_cimd_clients "CIMD admission; off by default"
        text_array cimd_client_hosts "up to 64 hosts; empty = any"
        text_array cimd_scopes "the pin for CIMD apps; NULL = every scope"
        jsonb scope_descriptions "scope -> what it lets an app do"
        timestamptz created_at
        timestamptz updated_at
    }
    oauth_client_resources {
        uuid client_id PK, FK "(org_id, project_id, client_id) -> oauth_clients"
        uuid resource_server_id PK, FK "(org_id, project_id, resource_server_id) -> resource_servers"
        uuid org_id
        uuid project_id
        text_array scopes "1 to 64 of the server's scopes"
        timestamptz created_at
        timestamptz updated_at
    }
    end_users {
        uuid id PK
        text public_id UK "also the OIDC sub"
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text email "normalized; unique per project"
        timestamptz email_verified_at "set at the first magic-link redeem"
        text name
        timestamptz last_sign_in_at
        timestamptz created_at
        timestamptz updated_at
    }
    browser_sessions {
        uuid id PK
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text secret_hash "SHA-256 hex of the cookie secret; never the secret"
        text user_agent
        timestamptz created_at
        timestamptz last_seen_at "idle expiry counts from here"
        timestamptz absolute_expires_at
        timestamptz ended_at
    }
    user_sessions {
        uuid id PK
        text public_id UK "names the account in the picker form"
        uuid org_id FK
        uuid project_id FK
        uuid browser_session_id FK "(org_id, project_id, browser_session_id) -> browser_sessions"
        uuid end_user_id FK "(org_id, project_id, end_user_id) -> end_users"
        text method "magic_link"
        timestamptz auth_time "for M5 auth_time and max_age"
        timestamptz selected_at "the latest live one is the active account"
        timestamptz created_at
        timestamptz last_seen_at
        timestamptz ended_at "set by sign-out"
    }
    end_user_terms {
        uuid end_user_id PK, FK "(org_id, project_id, end_user_id) -> end_users"
        int terms_version PK "the project's terms_version"
        uuid org_id
        uuid project_id
        text terms_url
        text privacy_url
        timestamptz accepted_at
        uuid user_session_id FK "SET NULL when the session is pruned"
        text user_session_public_id "kept after prune, so the evidence names the session"
        inet ip
        text user_agent
    }
    magic_links {
        uuid id PK
        text public_id UK "carried in the short verify cookie"
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text token_hash "SHA-256 hex of the token; never the token"
        text email "normalized"
        text return_to "a path under the issuer"
        inet request_ip
        timestamptz created_at
        timestamptz expires_at
        timestamptz used_at "set once, atomically, at redeem"
        uuid browser_session_id FK "the browser that redeemed it"
    }
    grant_families {
        uuid id PK
        text public_id UK "the sid claim"
        uuid org_id
        uuid project_id
        uuid client_id FK "(org_id, project_id, client_id) -> oauth_clients"
        uuid end_user_id FK "(org_id, project_id, end_user_id) -> end_users"
        uuid user_session_id FK "SET NULL when the session is pruned"
        text user_session_public_id "kept after prune"
        text_array scopes "the granted scopes; a refresh never widens them"
        timestamptz auth_time "the real sign-in time"
        timestamptz created_at
        timestamptz last_used_at "the refresh idle timeout counts from it"
        timestamptz absolute_expires_at
        timestamptz revoked_at
        text revoked_reason "reuse, code_replay, client_mismatch, end_session, sign_out, revoked, consent_revoked"
        uuid resource_server_id FK "the audience; CASCADE"
    }
    authorization_codes {
        uuid id PK
        uuid org_id
        uuid project_id
        text code_hash "SHA-256 hex of the code; never the code"
        uuid client_id FK "(org_id, project_id, client_id) -> oauth_clients"
        uuid end_user_id FK "(org_id, project_id, end_user_id) -> end_users"
        uuid user_session_id FK "SET NULL when the session is pruned"
        text redirect_uri "exactly as sent to /authorize"
        text_array scopes
        text code_challenge "S256 only; NULL when the client turned PKCE off"
        text code_challenge_method
        text nonce
        timestamptz auth_time
        timestamptz created_at
        timestamptz expires_at
        timestamptz used_at "set once, atomically, at /token"
        uuid family_id FK "the family it was redeemed into; a replay revokes it"
        uuid resource_server_id FK "the requested resource; CASCADE"
    }
    access_tokens {
        uuid id PK
        uuid org_id
        uuid project_id
        uuid family_id FK "(org_id, project_id, family_id) -> grant_families"
        text token_hash "SHA-256 hex of the token as issued (opaque or at+jwt); never the token"
        text_array scopes
        timestamptz created_at
        timestamptz expires_at
        uuid resource_server_id FK "the at+jwt audience; NULL = the opaque userinfo token"
        timestamptz revoked_at "RFC 7009 revocation of this token alone"
    }
    scope_consents {
        uuid id PK
        text public_id UK
        uuid org_id
        uuid project_id
        uuid end_user_id FK "(org_id, project_id, end_user_id) -> end_users"
        uuid client_id FK "(org_id, project_id, client_id) -> oauth_clients"
        uuid resource_server_id FK "NULL = an OIDC-only request"
        text_array scopes "the union of every approval"
        timestamptz granted_at
        timestamptz updated_at "compared with the re-entry marker for prompt=consent"
        inet granted_ip
        text granted_user_agent
    }
    refresh_tokens {
        uuid id PK
        uuid org_id
        uuid project_id
        uuid family_id FK "(org_id, project_id, family_id) -> grant_families"
        text token_hash "SHA-256 hex of the token; never the token"
        timestamptz created_at
        timestamptz expires_at
        timestamptz used_at "set at rotation; a second use revokes the family"
    }
    files {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text owner_kind "orgs, projects or users"
        uuid owner_id "the owner row; no FK, the kind names the table"
        text purpose "logo, avatar, ..."
        text visibility "public or private"
        text object_key UK "{visibility}/{app}/{owner kind}/{owner public_id}/{purpose}/{file public_id}.{ext}"
        text content_type
        bigint size_bytes "declared on upload, checked on completion"
        text original_name
        text status "pending, ready or deleting"
        timestamptz created_at
        timestamptz completed_at
        timestamptz expires_at "an unfinished upload is swept after it"
    }
    api_keys {
        uuid id PK
        uuid org_id FK
        uuid project_id FK "project keys only; (org_id, project_id) -> projects"
        text kind "project, org or personal"
        bool all_projects
        text name
        bytea secret_hash UK
        text secret_hint "last 4 characters; names the key on every surface (no public_id)"
        text_array scopes
        uuid_array resource_ids
        uuid created_by FK "NULL when the user is deleted"
        timestamptz created_at
        timestamptz expires_at
        timestamptz revoked_at
        timestamptz last_used_at
    }
    api_key_projects {
        uuid key_id PK, FK
        uuid project_id PK, FK
        uuid org_id FK
        timestamptz created_at
    }
    outbox_entries {
        uuid id PK
        uuid event_id
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text target
        bytea payload
        int attempt
        timestamptz next_attempt_at
        text status "pending, delivered, failed"
        text last_error
        timestamptz created_at
        timestamptz delivered_at
        text public_id UK "the delivery id on the API and in the delivery-id header"
        text event_public_id "the event id in the envelope and the event-id header"
    }
    webhook_endpoints {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        text url
        text description
        text event_types
        bytea secret_primary "sealed"
        bytea secret_secondary "sealed, during rotation"
        bool active
        timestamptz created_at
        timestamptz updated_at
    }
    webhook_deliveries {
        uuid id PK
        uuid org_id FK
        uuid project_id FK "(org_id, project_id) -> projects"
        uuid endpoint_id FK
        uuid delivery_id
        uuid event_id
        text event_type
        int attempt
        int status_code
        text error
        text response_body
        bool response_truncated
        text response_headers
        int duration_ms
        timestamptz created_at
    }
    users {
        uuid id PK
        text public_id UK
        text email UK "trimmed, lowercase"
        text name
        text avatar_url
        text password_hash "Argon2id PHC, empty when none"
        text idp_issuer "OIDC, set with idp_subject"
        text idp_subject
        text locale
        timestamptz terms_accepted_at
        timestamptz created_at
        timestamptz updated_at
    }
    memberships {
        uuid id PK
        text public_id UK
        uuid org_id FK
        uuid user_id FK
        text role "owner, admin, member"
        timestamptz created_at
    }
    invites {
        uuid id PK
        text public_id UK
        uuid org_id FK
        text email "normalized"
        text role "owner, admin, member"
        text token_hash UK "SHA-256 of the token, NULL only for a draft"
        bool draft "an MCP suggestion a person must confirm"
        uuid invited_by FK "NULL for a non-person operator"
        timestamptz expires_at
        timestamptz accepted_at
        uuid accepted_by FK
        timestamptz revoked_at
        timestamptz created_at
        timestamptz updated_at
    }
    legal_documents {
        uuid id PK
        text slug "terms, privacy"
        text version "the updated header value"
        text title
        text body "the markdown people accepted"
        text sha256
        timestamptz published_at
    }
    terms_acceptances {
        uuid id PK
        text public_id UK
        uuid user_id FK
        uuid terms_document_id FK
        uuid privacy_document_id FK
        text source "install, join, welcome"
        timestamptz accepted_at
    }
    mail_queue {
        uuid id PK "uuid v7, not tenant-scoped"
        bytea payload "sealed mailer.Message JSON, AAD = id, NULL once settled"
        bool sealed "false only in development with the console driver"
        text status "pending, sent, failed"
        int attempt
        timestamptz next_attempt_at
        text last_error "redacted, at most 512 bytes"
        timestamptz created_at
        timestamptz settled_at
    }
    password_resets {
        uuid id PK
        text public_id UK "held in the reset cookie"
        uuid user_id FK
        text token_hash UK "SHA-256 of the emailed token"
        timestamptz expires_at "one hour"
        timestamptz used_at "set once, by a conditional UPDATE"
        timestamptz created_at
    }
    sessions {
        text sid PK "opaque, no public_id"
        uuid user_id FK
        bytea payload "sealed Principal, AAD = sid"
        timestamptz expires_at
        timestamptz created_at
    }
```

Every foreign key to `orgs`, `projects`, `api_keys`, `webhook_endpoints`,
`oauth_clients` and `users` cascades on delete, except `orgs.created_by`,
`api_keys.created_by`, `invites.invited_by` and `invites.accepted_by`, which are
set to NULL. `oauth_client_resources` → `resource_servers` is `NO ACTION`: a
linked resource server can't be deleted, but a project delete removes both.
`end_user_terms.user_session_id` and `magic_links.browser_session_id` are set to
NULL when their session is pruned; the end user and browser session FKs cascade.
The grant tables cascade from their client, end user and resource server (a deleted resource server ends every
grant for it); `user_session_id` is set to NULL, and so is `authorization_codes.family_id`, so a used code outlives
its family until it is pruned. Access and refresh tokens cascade with their family. Scope consents cascade from
their end user, client and resource server.

## Constraints and indexes

Rules the diagram can't show. Names omit `{{.TablePrefix}}`.

| Table                                                                   | Constraint / index                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Rule                                                                                                                                                                                  |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `orgs`                                                                  | `orgs_one_system` (unique, `WHERE is_system`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | at most one system org; it marks the instance as installed                                                                                                                            |
| `projects`                                                              | `UNIQUE (org_id, slug)`, `projects_org_id_id_key UNIQUE (org_id, id)`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | slug unique per org; the pair is the target of org-scoped FKs                                                                                                                         |
| `projects`                                                              | `projects_issuer_handle_key` (unique), `projects_issuer_handle_format`, `projects_primary_color_format`, `projects_terms_version_nonnegative` (checks)                                                                                                                                                                                                                                                                                                                                                                                                                                   | one issuer URL per handle across every org; the store turns the unique violation into `PRJ005`                                                                                        |
| `api_keys`, `outbox_entries`, `webhook_endpoints`, `webhook_deliveries` | `<table>_org_project_fkey`: `(org_id, project_id) → projects (org_id, id)`, `ON DELETE CASCADE`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | a row's project always belongs to the row's org; a NULL `project_id` (org and personal keys) is allowed                                                                               |
| `api_keys`                                                              | `api_keys_kind_shape` (check)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | `project` keys have a `project_id` and no `all_projects`; `org` and `personal` keys have no `project_id`                                                                              |
| `api_keys`                                                              | `api_keys_created_by_fkey` (`ON DELETE SET NULL`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | `created_by` is a real user; deleting the user keeps the key but clears the owner, and a personal key without an owner never authenticates                                            |
| `api_keys`                                                              | `api_keys_personal_idx` (`WHERE kind = 'personal'`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | lists one person's personal keys in an org                                                                                                                                            |
| `api_keys`                                                              | `api_keys_secret_hash_key` (unique)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | keys are found by the hash of their secret                                                                                                                                            |
| `api_keys`                                                              | `api_keys_live_hint_key` (unique `(org_id, secret_hint)`, `WHERE revoked_at IS NULL`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | a live key is named by its hint within its org; minting tries a new secret when the hint is taken; revoked keys may share one                                                         |
| `api_key_projects`                                                      | FKs on `(org_id, key_id)` and `(org_id, project_id)`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | a key can only be granted projects of its own org                                                                                                                                     |
| `outbox_entries`                                                        | `UNIQUE (org_id, event_id, target)`, status check                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | one entry per event and target, per org                                                                                                                                               |
| `outbox_entries`                                                        | `outbox_entries_public_id_key` (unique)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | a delivery is found by its public id; all entries of one event share `event_public_id`                                                                                                |
| `webhook_endpoints`                                                     | `webhook_endpoints_secondary_nonempty` (check)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | the secondary secret is NULL or non-empty                                                                                                                                             |
| `users`                                                                 | `users_email_normalized` (check), `email` unique                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | email is stored trimmed and lowercase                                                                                                                                                 |
| `users`                                                                 | `users_idp_pair` (check), `users_idp_idx` (unique, `WHERE idp_issuer IS NOT NULL`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | issuer and subject are set together and identify one user                                                                                                                             |
| `memberships`                                                           | `UNIQUE (org_id, user_id)`, role check                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | one membership per person per org                                                                                                                                                     |
| `invites`                                                               | `invites_live_idx` (unique, `WHERE accepted_at IS NULL AND revoked_at IS NULL`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | one live invite per org and email; a re-invite updates that row                                                                                                                       |
| `invites`                                                               | `invites_token_or_draft` (check), `token_hash` unique                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | every invite but a draft has a token; tokens are found by their hash                                                                                                                  |
| `invites`                                                               | `email` check                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | email is stored trimmed and lowercase                                                                                                                                                 |
| `legal_documents`                                                       | `legal_documents_slug_version_key` (unique)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | one snapshot per document version; built-in documents only (a custom terms or privacy URL keeps no history)                                                                           |
| `terms_acceptances`                                                     | `terms_acceptances_user_idx`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | one person's acceptances, oldest first                                                                                                                                                |
| `mail_queue`                                                            | `mail_queue_settled_erased`, `mail_queue_pending_payload` (checks)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | a pending row holds its payload; a sent or failed row holds none                                                                                                                      |
| `mail_queue`                                                            | `mail_queue_due_idx` (`WHERE status = 'pending'`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | the worker's claim of due mail                                                                                                                                                        |
| `password_resets`                                                       | `token_hash` unique, `password_resets_user_idx` (`WHERE used_at IS NULL`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | tokens are found by their hash; a new request deletes the person's unused tokens                                                                                                      |
| `sessions`                                                              | `sessions_expires_idx`, `sessions_user_idx`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | sweep of expired rows; delete all sessions of a user                                                                                                                                  |
| `signing_keys`                                                          | `signing_keys_org_project_fkey` (`ON DELETE CASCADE`), `signing_keys_kid_key` (unique), checks on `alg`, `status`, a sealed key on every key but a retired one (`signing_keys_sealed_while_live`), `activates_at` while next and `retire_after` while retiring                                                                                                                                                                                                                                                                                                                           | a key belongs to a project of its own org and goes with it                                                                                                                            |
| `signing_keys`                                                          | `signing_keys_one_active`, `signing_keys_one_next` (unique `(project_id, alg)`, `WHERE status = 'active'` / `'next'`), `signing_keys_project_status_idx`                                                                                                                                                                                                                                                                                                                                                                                                                                 | one signer and at most one waiting key per project and algorithm                                                                                                                      |
| `oauth_clients`                                                         | `oauth_clients_org_project_fkey` (`ON DELETE CASCADE`), `oauth_clients_org_project_id_key UNIQUE (org_id, project_id, id)`                                                                                                                                                                                                                                                                                                                                                                                                                                                               | a client belongs to a project of its own org; the triple is the target of the secret and link FKs                                                                                     |
| `oauth_clients`                                                         | `oauth_clients_kind_check`, `oauth_clients_grant_types_check`, `oauth_clients_consent_policy_check`, `oauth_clients_pkce_public`, `oauth_clients_name_length`, `oauth_clients_redirect_uris_count`, `oauth_clients_post_logout_uris_check` (checks), `oauth_clients_project_idx`                                                                                                                                                                                                                                                                                                         | web = confidential + `client_secret_basic` or `client_secret_post`, native = public + `none` and always PKCE; `authorization_code` always; 1–20 redirect URIs; sign-out URIs web only |
| `oauth_clients`                                                         | `oauth_clients_registration_kind_check`, `oauth_clients_metadata_url_key UNIQUE (project_id, metadata_url)`, `oauth_clients_metadata_url_https`, `oauth_clients_client_uri_https`, `oauth_clients_cimd_metadata`, `oauth_clients_cimd_public` (checks), `oauth_clients_metadata_expiry_idx` (`WHERE registration_kind = 'cimd'`)                                                                                                                                                                                                                                                         | a CIMD shadow has its metadata URL and fetch times and is always public, native, `none` and consent-required; one shadow per URL in a project                                         |
| `oauth_client_secrets`                                                  | `oauth_client_secrets_client_fkey` (`ON DELETE CASCADE`), `oauth_client_secrets_hash_key` (unique), `oauth_client_secrets_hash_format`, `oauth_client_secrets_hint_length` (checks)                                                                                                                                                                                                                                                                                                                                                                                                      | a secret goes with its client; it is found by its `sha256$` hash                                                                                                                      |
| `oauth_client_secrets`                                                  | `oauth_client_secrets_one_current` (unique `(client_id)`, `WHERE expires_at IS NULL`), `oauth_client_secrets_client_idx`                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | one current secret per client; the service keeps at most one old one alongside it                                                                                                     |
| `resource_servers`                                                      | `resource_servers_org_project_fkey` (`ON DELETE CASCADE`), `resource_servers_org_project_id_key UNIQUE (org_id, project_id, id)`, `resource_servers_identifier_key UNIQUE (project_id, identifier)`                                                                                                                                                                                                                                                                                                                                                                                      | a server belongs to a project of its own org; one server per identifier in a project                                                                                                  |
| `resource_servers`                                                      | checks on name and identifier length, 1–64 scopes, `verification_state`, `verified_at` once verified, `last_error` an object, `consecutive_failures >= 0`, `trusted_at` exactly when `trusted`, `resource_servers_https_or_trusted` (a non-https identifier only while trusted); `resource_servers_recheck_idx` (`WHERE NOT trusted`)                                                                                                                                                                                                                                                    | the scheduled re-check finds stale untrusted servers per project                                                                                                                      |
| `resource_servers`                                                      | `resource_servers_cimd_hosts_count` (≤ 64), `resource_servers_cimd_scopes_count` (1–64 when pinned), `resource_servers_scope_descriptions_object` (checks)                                                                                                                                                                                                                                                                                                                                                                                                                               | CIMD admission per server; NULL `cimd_scopes` follows the catalog                                                                                                                     |
| `oauth_client_resources`                                                | PK `(client_id, resource_server_id)`, `oauth_client_resources_client_fkey` (`ON DELETE CASCADE`), `oauth_client_resources_resource_server_fkey` (`NO ACTION`), `oauth_client_resources_scopes_count` (1–64), `oauth_client_resources_resource_server_idx`                                                                                                                                                                                                                                                                                                                                | one link per pair; both ends share the link's org and project, so a link across projects is impossible                                                                                |
| `end_users`                                                             | `end_users_org_project_fkey` (`ON DELETE CASCADE`), `end_users_org_project_id_key UNIQUE (org_id, project_id, id)`, `end_users_email_key UNIQUE (project_id, email)`, `end_users_email_normalized`, `end_users_name_length` (checks)                                                                                                                                                                                                                                                                                                                                                     | one end user per email in a project; the same email is a different end user in another project; emails are stored trimmed and lowercase                                               |
| `browser_sessions`                                                      | `browser_sessions_org_project_fkey` (`ON DELETE CASCADE`), `browser_sessions_secret_key UNIQUE (project_id, secret_hash)`, `browser_sessions_secret_format`, `browser_sessions_user_agent_length`, `browser_sessions_expiry` (checks), `browser_sessions_expiry_idx`                                                                                                                                                                                                                                                                                                                     | a cookie is found by the hash of its secret within its project; the prune job finds expired rows                                                                                      |
| `user_sessions`                                                         | `user_sessions_browser_session_fkey`, `user_sessions_end_user_fkey` (`ON DELETE CASCADE`), `user_sessions_method_check`, `user_sessions_one_live` (unique `(browser_session_id, end_user_id)`, `WHERE ended_at IS NULL`), `user_sessions_end_user_idx`                                                                                                                                                                                                                                                                                                                                   | one live session per account on a browser; signing in again as the same person refreshes it                                                                                           |
| `end_user_terms`                                                        | PK `(end_user_id, terms_version)`, `end_user_terms_end_user_fkey` (`ON DELETE CASCADE`), `end_user_terms_user_session_fkey` (`ON DELETE SET NULL (user_session_id)`), `end_user_terms_version_positive`, `end_user_terms_user_agent_length`                                                                                                                                                                                                                                                                                                                                              | one acceptance per end user and terms version; rows are only ever inserted                                                                                                            |
| `magic_links`                                                           | `magic_links_org_project_fkey` (`ON DELETE CASCADE`), `magic_links_browser_session_fkey` (`ON DELETE SET NULL (browser_session_id)`), `magic_links_token_key UNIQUE (project_id, token_hash)`, `magic_links_token_format`, `magic_links_email_normalized`, `magic_links_return_to_path`, `magic_links_expiry` (checks), `magic_links_one_live` (unique `(project_id, email)`, `WHERE used_at IS NULL`), `magic_links_expiry_idx`                                                                                                                                                         | a link is found by its token hash within its project; one unused link per email; `return_to` is a path                                                                                |
| `grant_families`                                                        | `grant_families_client_fkey`, `grant_families_end_user_fkey`, `grant_families_resource_server_fkey` (`ON DELETE CASCADE`), `grant_families_user_session_fkey` (`ON DELETE SET NULL` of the column), `grant_families_org_project_id_key`, `grant_families_revoked_check`, `grant_families_revoked_reason_check`, `grant_families_expiry` (checks), `grant_families_client_end_user_idx`, `grant_families_user_session_idx`, `grant_families_expiry_idx`                                                                                                                                   | one family per sign-in at a client; `public_id` is the `sid`; a revocation always names its reason                                                                                    |
| `authorization_codes`                                                   | `authorization_codes_client_fkey`, `authorization_codes_end_user_fkey`, `authorization_codes_resource_server_fkey` (`ON DELETE CASCADE`), `authorization_codes_user_session_fkey`, `authorization_codes_family_fkey` (`ON DELETE SET NULL` of the column), `authorization_codes_hash_key UNIQUE (project_id, code_hash)`, `authorization_codes_hash_format`, `authorization_codes_redirect_uri_length`, `authorization_codes_pkce_check`, `authorization_codes_nonce_length`, `authorization_codes_expiry`, `authorization_codes_used_family` (checks), `authorization_codes_expiry_idx` | a code is found by its hash within its project; a stored challenge is a 43-character base64url S256 value; only a used code names a family                                            |
| `access_tokens`                                                         | `access_tokens_family_fkey`, `access_tokens_resource_server_fkey` (`ON DELETE CASCADE`), `access_tokens_hash_key UNIQUE (project_id, token_hash)`, `access_tokens_hash_format`, `access_tokens_expiry` (checks), `access_tokens_expiry_idx`                                                                                                                                                                                                                                                                                                                                              | a token (opaque or at+jwt) is found by its hash within its project and dies with its family or resource server                                                                        |
| `refresh_tokens`                                                        | `refresh_tokens_family_fkey` (`ON DELETE CASCADE`), `refresh_tokens_hash_key UNIQUE (project_id, token_hash)`, `refresh_tokens_hash_format`, `refresh_tokens_expiry` (checks), `refresh_tokens_one_live` (unique `(family_id)`, `WHERE used_at IS NULL`), `refresh_tokens_expiry_idx`                                                                                                                                                                                                                                                                                                    | one live refresh token per family; rotation marks the old one used                                                                                                                    |
| `scope_consents`                                                        | `scope_consents_end_user_fkey`, `scope_consents_client_fkey`, `scope_consents_resource_server_fkey` (`ON DELETE CASCADE`), `scope_consents_grant_key UNIQUE NULLS NOT DISTINCT (project_id, end_user_id, client_id, resource_server_id)`, `scope_consents_scopes_count` (1–72), `scope_consents_updated`, `scope_consents_user_agent_length` (checks), `scope_consents_client_idx`, `scope_consents_resource_server_idx`                                                                                                                                                                 | one remembered consent per end user, client and resource server, an OIDC-only one included; approving again adds scopes                                                               |
| `files`                                                                 | `files_org_id_id_key UNIQUE (org_id, id)`, `files_object_key_key` (unique), checks on `owner_kind`, `visibility`, `status` and `size_bytes > 0`                                                                                                                                                                                                                                                                                                                                                                                                                                          | one object per row; the pair is the target of `projects_logo_file_fkey`                                                                                                               |
| `files`                                                                 | `files_status_expires_at_idx`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | the sweep of expired pending uploads and of files marked deleting                                                                                                                     |
| `projects`                                                              | `projects_logo_file_fkey`: `(org_id, logo_file_id) → files (org_id, id)`, `ON DELETE SET NULL (logo_file_id)`                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | a logo is a file of the project's own org; deleting the file clears the logo                                                                                                          |

## Row level security

Every org-owned table has `ENABLE` and `FORCE ROW LEVEL SECURITY` and one `FOR ALL` policy with `USING` and
`WITH CHECK`, created in the table's own migration. Policies read the scope only through the helpers
`current_org_id()`, `current_project_id()` and `current_user_id()` in `002_orgs.sql`
(`NULLIF(current_setting('app.current_…', true), '')::uuid`, so a missing scope means no rows, never a cast error).
`tenant_policy_guard_test.go` fails on an inline `current_setting`, an org-owned table without its policy or a
project-owned table whose policy skips the project, and
`rls_isolation_integration_test.go` proves each table as the service role.

| Table                                                                                                                                                                                                                                                                | Policy                                                                                                                                                                                                                                                              |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `projects`, `api_keys`, `api_key_projects`, `outbox_entries`, `webhook_endpoints`, `webhook_deliveries`, `invites`, `files`                                                                                                                                          | `org_id = current_org_id()`                                                                                                                                                                                                                                         |
| `signing_keys`, `oauth_clients`, `oauth_client_secrets`, `resource_servers`, `oauth_client_resources`, `end_users`, `browser_sessions`, `user_sessions`, `magic_links`, `grant_families`, `authorization_codes`, `access_tokens`, `refresh_tokens`, `scope_consents` | `org_id = current_org_id() AND project_id = current_project_id()`: a unit of work joined with a project narrows to it                                                                                                                                               |
| `end_user_terms`                                                                                                                                                                                                                                                     | read (`FOR SELECT`) and append (`FOR INSERT`) only, both on `org_id = current_org_id() AND project_id = current_project_id()`: consent is evidence, so no policy allows `UPDATE` or `DELETE`; the foreign-key cascades still remove it with its end user or project |
| `memberships`                                                                                                                                                                                                                                                        | read: `org_id = current_org_id() OR user_id = current_user_id()`; write: `org_id = current_org_id()`                                                                                                                                                                |
| `orgs`                                                                                                                                                                                                                                                               | read: `id = current_org_id()` or an org the user is a member of; write: `id = current_org_id()`                                                                                                                                                                     |

The `orgs` policy reads `memberships`, so `002_orgs.sql` only enables and forces row level security on `orgs`, and
`007_memberships.sql` creates its policy. `users`, `sessions`, `password_resets`, `terms_acceptances`, `mail_queue`,
`legal_documents` and `goose_db_version` have no org and no row level security.

## Lookup functions

`027_tenant_lookups.sql` holds the `SECURITY DEFINER` functions for reads that must run before a tenant scope
exists, or across every org. Each one runs as the owner with `search_path = <schema>, pg_catalog, pg_temp`,
returns public columns only (never a hash), and is closed to `PUBLIC`; the service role gets `EXECUTE` through
the owner's default privileges. `lookups_integration_test.go` checks all of this.

| Function                                 | Returns                                                                | Used by                                                |
| ---------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------ |
| `list_org_ids()`                         | every org id and creation time                                         | the outbox dispatcher, which then runs each org scoped |
| `resolve_system_org()`                   | the system org                                                         | sign-in refresh, install                               |
| `resolve_org_by_slug(text)`              | one org                                                                | slug routes and the system slug check                  |
| `resolve_org_by_public_id(text)`         | one org                                                                | public id resolution on every surface                  |
| `list_orgs_for_user(uuid)`               | the orgs a person is a member of                                       | `ListOrgs` for a person                                |
| `resolve_invite_by_token_hash(text)`     | one invite, without `token_hash`                                       | the invite landing page and join                       |
| `resolve_api_key_by_secret_hash(bytea)`  | one API key and its projects, without the hash                         | API key authentication                                 |
| `resolve_project_by_issuer_handle(text)` | one project's issuer: ids, handle, branding, terms and sign-in setting | issuer resolution before sign-in (M2 discovery)        |
| `issuer_handle_exists(text)`             | whether any org uses a handle                                          | the issuer handle availability check                   |

## External migrations

Run another tool's migrations after ours on the same DB:

```go
schema.New(cfg).WithUpgrade("whatsmeow", meow.Upgrade).Up(ctx, db)
```

The external tool owns its tables under a reserved prefix; ours must not use it.
`MigrationsFS()` exposes the raw SQL for a guard test.

## Add a migration

1. `NNN_<name>.sql` at the next number, with a working `Down`.
2. Bump `VERSION`.
3. Add or update the `schema/entity` binding for the table.
4. In the same commit, update this README: the diagram (tables, columns,
   relationships) and the constraints table (new unique, check, partial or
   foreign-key rules).
