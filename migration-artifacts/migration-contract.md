# Migration Contract: chi -> echo (go-base @ 56903f8)

| # | Source (chi)                          | Target (echo)                                   | Files |
|---|---------------------------------------|-------------------------------------------------|-------|
| 1 | go-chi/chi/v5 router                  | labstack/echo/v4                                | go.mod |
| 2 | chi.NewRouter / Mount / Route         | echo.New / Group                                | api/api.go, api/app/*, api/admin/*, auth/pwdless/api.go |
| 3 | func(w,r) handlers                    | func(c echo.Context) error                      | all handlers |
| 4 | chi.URLParam(r,"x")                   | c.Param("x")                                    | api/app/account.go, api/admin/accounts.go |
| 5 | chi/v5/middleware Recoverer/RequestID/Timeout | echo middleware Recover/RequestID/ContextTimeout | api/api.go |
| 6 | chi/v5/middleware RequestLogger+LogEntry | echo.MiddlewareFunc + c.Set/c.Get             | logging/logger.go |
| 7 | go-chi/render Render/Respond/Bind/Status | c.JSON + returned errors + HTTPErrorHandler   | 12 files, 87 call sites |
| 8 | go-chi/jwtauth/v5 Verifier/FromContext | jwx/v2 jwt.Parse + custom echo middleware      | auth/jwt/tokenauth.go, authenticator.go |
| 9 | go-chi/cors                           | echo middleware.CORSWithConfig                  | api/api.go |
|10 | go-chi/docgen                         | NO EQUIVALENT -> remove cmd/gendoc.go, routes.md| cmd/gendoc.go, routes.md |
|11 | context.WithValue(r.Context(), k, v)  | c.Set(k, v) / c.Get(k)                          | api/app/*, api/admin/*, auth/jwt/* |
|12 | http.Server{Handler: chiMux}          | echo instance e.Start / e.Shutdown              | api/server.go |
|13 | chi.NewRouter() in test               | echo.New() in test                              | auth/pwdless/api_test.go |
|14 | README "chi router" references        | echo references                                 | README.md |

## OUT OF SCOPE (must not change)
- database/*, models/*, email/*  (bun / SMTP, framework-neutral)
- cmd/root.go, cmd/serve.go, cmd/migrate.go, main.go  (cobra/viper)
- database/migrations/*  (schema + seed data the oracle depends on)
- go.mod `go 1.25.0` directive
- Dockerfile / docker-compose.yml (contain no framework-specific config)

## OBSERVABLE BOUNDARY THAT MUST NOT CHANGE
GET  /healthz            200 "ok" text/plain
GET  /                   200 index.html
GET  /<unknown>          200 index.html (NOT 404)
GET  /api/account        401 {"status":"Unauthorized","error":"token unauthorized"}
POST /auth/login         200 {}
GET  /api/account  +JWT  200 {id,created_at,updated_at,last_login,email,name,active,roles,token[]}
GET  /api/profile  +JWT  200 {updated_at,theme}
GET  /admin/accounts+JWT 200 {accounts:[...],count:N}
GET  /admin/        +JWT 200 "Hello Admin"
GET  /admin/accounts +user-role JWT  403
Trailing-slash: both /api/account and /api/account/ must work (chi Mount registered both)
