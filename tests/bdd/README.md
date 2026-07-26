# birb BDD acceptance suite

Gherkin/godog acceptance tests that exercise the real application stack: the
production Chi router (via `internal/httpapi.NewRouter`), a real Postgres
database, and real signed JWTs verified end-to-end by `auth.JWKSVerifier`.

The suite runs against an `httptest.Server` using the same
handler/service/store graph that `cmd/api/main.go` wires together in
production. No mocks or separately compiled test binary are involved.

The entire package is guarded by `//go:build bdd`, so it is excluded from
normal `go build ./...` and `go test ./...` runs. It only compiles and runs
when the `bdd` build tag is supplied.

## Running it

1. Start a disposable Postgres instance:

   ```sh
   docker run -d --name birb-bdd-pg \
     -e POSTGRES_USER=birb \
     -e POSTGRES_PASSWORD=birb \
     -e POSTGRES_DB=birb_bdd \
     -p 55432:5432 \
     postgres:16-alpine
   ```

2. Run the acceptance suite:

   ```sh
   BIRB_TEST_DATABASE_URL="postgres://birb:birb@localhost:55432/birb_bdd?sslmode=disable" \
     go test -tags bdd ./tests/bdd/... -v
   ```

   Database migrations (`db/migrations/*.sql`, via `goose`) are applied
   automatically and are idempotent, so the same container can be reused
   across runs.

   If `BIRB_TEST_DATABASE_URL` is not set, the suite skips execution with a
   clear message instead of failing.

3. Tear down the database:

   ```sh
   docker rm -f birb-bdd-pg
   ```

## Layout

```text
tests/bdd/
  features/*.feature   Gherkin feature files
  relish.go            Step definitions
  relish_world.go      Per-scenario state, variable interpolation, JSON matching
  suite_test.go        Test harness (database, migrations, JWKS, HTTP server)
```

### Test isolation

Each scenario runs against a clean application state. A Godog `Before` hook
truncates the `sightings` and `users` tables before every scenario while
preserving the seeded `birds` reference data. Scenarios therefore remain
independent even though they share a single application instance and database
connection pool.

## Step reference

The step library provides reusable building blocks for authentication, HTTP
requests, response assertions, and variable capture.

| Step | Effect |
| --- | --- |
| `Given I am anonymous` | Clears the Authorization header and authentication state. |
| `Given I am authenticated as "alice"` | Generates a signed JWT for a deterministic UUIDv5 identity, sets the `Authorization` header, and populates `current_user.*` variables. |
| `Given the user "alice" has tier "free"` / `"premium"` | Creates the user through the public API, then updates the stored tier directly in Postgres. |
| `Given I set header "X" to "Y"` | Adds a request header that persists for the remainder of the scenario. |
| `Given I set query param "X" to "Y"` | Adds a query parameter to the next request only. |
| `Given a seeded bird is saved as "birdId"` | Saves the identifier of a migration-seeded bird for later use. |
| `When I make a GET/POST/PUT/PATCH/DELETE call to /path` | Sends the request. |
| `When I make a ... call to /path with body` | Sends a request with a JSON body. |
| `Then I should receive a 200 response` | Asserts only the response status. |
| `Then I should receive a 200 JSON response` | Asserts response status and JSON content type. |
| `Then the response body should be` | Exact JSON equality. |
| `Then the response body should contain` | Recursive JSON subset match. |
| `Then the response field "results.0.status" should be "created"` | Compares a value at a dotted JSON path. |
| `Then the response field "id" should match "^usr_[a-z0-9]{26}$"` | Validates a response field against a regular expression. |
| `Then the response header "ETag" should be "..."` | Exact response header assertion. |
| `Then the response header "ETag" should not be empty` | Asserts that a response header is present. |
| `Then I save the response under "widget"` | Stores the response body as reusable variables. |
| `Then I save the response header "ETag" as "etag"` | Stores a response header as a reusable variable. |

## Custom steps

The suite includes two custom assertion helpers in addition to the standard
request and response steps:

### Response field regex matching

```gherkin
Then the response field "id" should match "^usr_[a-z0-9]{26}$"
```

Useful when asserting the format of server-generated values whose exact value
cannot be known ahead of time.

### Response header capture

```gherkin
Then I save the response header "ETag" as "etag"
```

Allows values such as `ETag` to be reused in later requests (for example,
`If-None-Match` conditional requests).

## Router extraction

To allow the acceptance suite to construct the production HTTP stack,
the router creation logic was extracted from `cmd/api/main.go` into
`internal/httpapi.NewRouter()`. This is a mechanical refactoring only:
middleware, routes, and mount order remain unchanged, and both production and
tests now use the same router construction code.
