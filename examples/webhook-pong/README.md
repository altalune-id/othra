# webhook-pong

Localhost webhook receiver for testing: verifies `X-Othra-Signature`, logs each delivery, and always replies `200 PONG!!!`.

## Run

```bash
make webhook-pong                              # builds ./bin/webhook-pong
set -a; . examples/webhook-pong/.env; set +a   # loads OTHRA_WEBHOOK_SECRET
./bin/webhook-pong
```

Env: `OTHRA_WEBHOOK_SECRET` (signing secret; unset ⇒ signatures not verified, prints a warning), `ADDR` (default `127.0.0.1:9099`).

## Point a webhook at it

Console → **Project → Webhooks → New endpoint**, URL `http://127.0.0.1:9099/`, subscribe to `invite.created` / `member.added`, and copy the secret (shown once) into `.env`. Then invite a member (or press **Send test**, which delivers `webhook.ping`) to trigger a delivery.

> Non-production only: dev allows `http://` + loopback. Production requires `https` and blocks private hosts (SSRF).

## Signature

`X-Othra-Signature` = `v1=` + `HMAC-SHA256(secret, X-Othra-Timestamp + "." + rawBody)`, timestamp in unix seconds (±5 min). During rotation both signatures are sent, space-separated.
