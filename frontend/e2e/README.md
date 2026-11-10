# End-to-end tests

Playwright drives the real screens against a real backend. The flows are the
ones a regression would hurt most: sign-in, invite and activation, password
reset, and saving a role's permissions.

## What the run needs

- A backend with a migrated database, reachable at `E2E_API_URL`
  (default `http://127.0.0.1:9990`), started with
  `AUTH_MANUAL_TOKEN_DELIVERY=1` so it hands activation and reset codes back
  instead of mailing them. Never point this at production.
- An admin account on it, made with `admin bootstrap`:
  `E2E_ADMIN_EMAIL` and `E2E_ADMIN_PASSWORD`.
- Chromium for Playwright, once per machine: `pnpm exec playwright install chromium`.

Every run invites new people with unique addresses, so a database can be reused.

A run makes about nine auth calls, and the backend allows ten per minute and IP
(`API_RATE_LIMIT_AUTH`). Wait a minute between runs, or raise that limit on the
test backend.

## Running

```bash
E2E_ADMIN_EMAIL=admin@example.test E2E_ADMIN_PASSWORD='…' pnpm e2e
```

The config starts `next dev` on port 3101 (`E2E_PORT`) pointed at the backend,
or reuses one already running there.
