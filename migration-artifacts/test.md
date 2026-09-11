# test.md — tests and behavioural oracle for the chi → echo migration

Companion to `truth.md`. Describes what `test.patch` contains, what each test
asserts, how to run them, and what was actually observed.

---

## 1. What `test.patch` contains

| File | Status | Purpose |
|---|---|---|
| `auth/pwdless/api_test.go` | modified | Existing Go unit tests, migrated to echo |
| `.gitignore` | modified | ignore `__pycache__/`, `*.pyc`, `.pytest_cache/` created by the oracle |
| `tests/oracle.feature` | new | Framework-neutral Gherkin behavioural oracle |
| `tests/smoke.py` | new | `pytest + requests` concretisation of the oracle |

`test.patch` is kept separate from `golden.patch` so the tests can be applied to
a candidate implementation without also applying the reference solution.

Apply order (both against a clean checkout of `56903f8`):

```bash
git apply golden.patch   # reference solution — never given to the agent
git apply test.patch     # tests/oracle — applied to whatever is being scored
```

---

## 2. Two layers of testing, and why both exist

**Layer 1 — Go unit tests (`auth/pwdless/api_test.go`).** These are the
repository's own tests. They are *source-framework-coupled by construction*: they
build a router, mount the auth resource on it and drive it through
`httptest.Server`. They cannot survive a framework migration untouched, which is
exactly why they are in `test.patch` rather than being treated as fixed.

**Layer 2 — behavioural oracle (`tests/oracle.feature` + `tests/smoke.py`).**
Framework-neutral. Speaks HTTP to a running container and never imports the
application, reads its source, or runs its unit tests. This is the layer that
decides whether behaviour was preserved, and the only layer that is valid to
score a candidate migration with.

---

## 3. Changes to the existing Go tests

Only framework plumbing changed. Every test case, input, expected status,
expected error and invocation assertion is byte-for-byte the original.

```diff
-	"github.com/go-chi/chi/v5"
+	"github.com/labstack/echo/v4"
+	"github.com/dhax/go-base/render"

-	r := chi.NewRouter()
-	r.Use(logging.NewStructuredLogger(logging.NewLogger()))
-	r.Mount("/", auth.Router())
-	ts = httptest.NewServer(r)
+	e := echo.New()
+	e.HideBanner = true
+	e.HidePort = true
+	e.HTTPErrorHandler = render.ErrorHandler
+	e.Use(logging.NewStructuredLogger(logging.NewLogger()))
+	auth.RegisterRoutes(e.Group(""))
+	ts = httptest.NewServer(e)

-	_, tokenString, _ := auth.TokenAuth.JwtAuth.Encode(claims)
+	tokenString, _ := auth.TokenAuth.Encode(claims)
```

No test was weakened, skipped, or removed. Coverage is unchanged: 4 test
functions, 16 subtests.

| Test | Subtests |
|---|---|
| `TestAuthResource_login` | missing, inexistent, disabled, valid |
| `TestAuthResource_token` | invalid, expired, deleted_account, disabled, valid |
| `TestAuthResource_refresh` | not_found, expired, disabled, valid |
| `TestAuthResource_logout` | notfound, expired, valid |

---

## 4. The behavioural oracle

### 4.1 Design constraint that shaped it

`POST /auth/login` returns `200 {}`. The login token is delivered by email — to
stdout when SMTP is unconfigured — so it **never crosses the HTTP boundary**.
An oracle restricted to HTTP therefore cannot complete the passwordless flow.

Rather than scrape the container log (which would stop being a pure HTTP
oracle), `smoke.py` **mints its own HS256 access token** using the known
`AUTH_JWT_SECRET`. This is sound because `auth/jwt/authenticator.go` validates
signature, expiry and claim shape only — it performs no database lookup — so a
correctly signed token is indistinguishable from a login-issued one.

Claims required (all mandatory; omitting any yields 401):

```json
{"id": <int>, "sub": "<string>", "roles": ["<string>"], "iat": <unix>, "exp": <unix>}
```

### 4.2 What is asserted

| # | Test | Asserts |
|---|---|---|
| 1 | `test_healthz_returns_ok` | 200, body exactly `ok` |
| 2 | `test_root_serves_the_spa` | 200, SPA marker present |
| 3 | `test_unknown_path_falls_back_to_the_spa` | 200 **not 404**, and body identical to `/` |
| 4 | `test_protected_route_without_token_is_unauthorized` | 401, `{"status":"Unauthorized","error":"token unauthorized"}` |
| 5 | `test_protected_route_with_foreign_signature_is_unauthorized` | 401 for a token signed with a different key |
| 6 | `test_login_with_registered_email_is_accepted` | 200, body exactly `{}` |
| 7 | `test_login_with_unregistered_email_is_rejected` | 401, error `email not registered` |
| 8 | `test_login_with_malformed_email_is_rejected` | 401, error `invalid email address` |
| 9 | `test_account_returns_the_authenticated_account` | 200, id/email/name/active/roles of account 1 |
| 10 | `test_profile_returns_the_default_theme` | 200, `theme == "default"` |
| 11 | `test_admin_lists_all_accounts` | 200, `count == 2`, both seeded emails |
| 12 | `test_admin_index_greets` | 200, body exactly `Hello Admin` |
| 13 | `test_non_admin_is_forbidden_from_admin_routes` | **403** for a `roles:["user"]` token |
| 14–17 | `test_resource_paths_answer_with_and_without_trailing_slash` | 200 for `/api/account`, `/api/account/`, `/api/profile`, `/api/profile/` |

Tests 3, 13 and 14–17 are the discriminating ones. Test 3 catches the SPA
catch-all being turned into a 404 — the single most common way a Go router
migration breaks this app. Test 13 forces the middleware chain (verify →
authenticate → authorize) to be ported in the right order rather than merely
having the routes exist. Tests 14–17 catch the trailing-slash forms that chi's
`Mount` answered on being silently dropped.

### 4.3 Fixture dependency

Tests 9–13 depend on the seed data in
`database/migrations/2_bootstrap_users.up.sql`, so the `migrate` step is
mandatory before the oracle runs:

```
id=1  admin@example.com  "Admin Example"  active  roles={admin}
id=2  user@example.com   "User Example"   active  roles={user}
```

---

## 5. How to run

### 5.1 Go unit tests

```bash
go test -v ./...
```

### 5.2 Behavioural oracle

```bash
# 1. bring up the stack (secret must be >=32 chars and not a known weak value)
export AUTH_JWT_SECRET="$(openssl rand -base64 64 | tr -d '\n')"
docker compose up -d postgres
docker compose run --rm server ./main migrate     # required: seeds the fixtures
docker compose up -d server

# 2. run the oracle
pip install pytest requests
BASE_URL=http://localhost:3000 AUTH_JWT_SECRET="$AUTH_JWT_SECRET" \
  pytest -v tests/smoke.py
```

`smoke.py` reads `BASE_URL` (default `http://localhost:3000`) and requires
`AUTH_JWT_SECRET` to match the value the server was started with.

---

## 6. Results actually observed

### 6.1 Go unit tests, migrated tree

```
--- PASS: TestAuthResource_login (0.00s)
--- PASS: TestAuthResource_token (0.00s)
--- PASS: TestAuthResource_refresh (0.00s)
--- PASS: TestAuthResource_logout (0.00s)
PASS
ok  	github.com/dhax/go-base/auth/pwdless	0.011s
```

**16/16 subtests pass.**

### 6.2 Oracle against the migrated (echo) implementation

```
17 passed in 0.09s
```

### 6.3 Oracle against the original (chi) implementation

The same unmodified `smoke.py`, run against a container built from the pinned
baseline `56903f8` on a separate Compose project:

```
17 passed in 0.09s
```

**This is the oracle's own validation.** A behavioural oracle that only passes on
the implementation it was written against proves nothing. Passing identically on
both `f_s` and `f_t` establishes that it measures behaviour, not framework.

### 6.4 Oracle self-correction during authoring

Two tests initially failed on **both** implementations, asserting
`"<!DOCTYPE html>"` in the SPA body. Inspection of `public/index.html` showed the
file opens with `<html>` and has no doctype — the assertion was simply wrong, and
both implementations were serving the correct 139-byte page.

The fix made the assertion *stricter*, not weaker: it now matches a real marker
string from the file and additionally requires the fallback body to be identical
to the one served at `/`. Recorded here because it is a change to a test, and
changes to tests should never be silent.

---

## 7. Coverage gaps

Stated explicitly rather than left implicit:

- **`/auth/refresh` and `/auth/logout`** are not reachable by the HTTP oracle —
  their refresh-token claim is a database-stored UUID that a minted token cannot
  satisfy. Covered by the Go unit tests instead.
- **Write paths are not covered by the oracle.** `PUT`/`DELETE` on
  `/api/account`, `/api/profile`, `/admin/accounts` and
  `/api/account/token/:tokenID` are exercised by the oracle for routing and
  authorization only, not for mutation semantics — a candidate could pass
  `smoke.py` with a broken `update` handler. They *were* verified for this
  migration by a separate 21-step differential run against both implementations
  (`truth.md` §6.7, 20/21 identical, including read-back confirming persistence),
  but that run is a migration check, not part of the scored oracle. **If this
  task is used to score candidates, extend `smoke.py` with the write paths
  first.**
- **CORS is not exercised by the oracle**, though it *was* active in every run
  (`docker-compose.yml:23` sets `ENABLE_CORS: "true"`) and was separately
  compared across 7 interactions — see `truth.md` §6.8. Preflight responses
  differ on the wire between chi and echo while remaining browser-equivalent, so
  asserting raw preflight headers in `smoke.py` would be a mistake.
- **The `jwt` cookie token source is not exercised** — only the `Authorization`
  header path.
- **TLS is not exercised.**
- **`Content-Type` is not asserted**, deliberately: echo emits
  `charset=UTF-8` where chi emitted `charset=utf-8`, and charset is
  case-insensitive per RFC 7231. Asserting it would fail a correct migration.
