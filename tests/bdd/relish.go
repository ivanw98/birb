//go:build bdd

package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// bddNamespace roots the deterministic per-username auth-id derivation below.
var bddNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("birb-bdd"))

// authIDForUsername deterministically derives a stable Supabase-style auth uid (a UUID) from a BDD username.
func authIDForUsername(username string) string {
	return uuid.NewSHA1(bddNamespace, []byte(username)).String()
}

func emailForUsername(username string) string {
	return username + "@example.test"
}

// InitializeScenario registers the shared step library and per-scenario lifecycle hook, called once by godog.TestSuite.
func InitializeScenario(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if err := resetDB(ctx); err != nil {
			return ctx, fmt.Errorf("reset db before scenario: %w", err)
		}
		return withWorld(ctx, newWorld()), nil
	})

	// --- Given: identity ---
	sc.Step(`^I am anonymous$`, stepIAmAnonymous)
	sc.Step(`^I am authenticated as "([^"]*)"$`, stepIAmAuthenticatedAs)
	sc.Step(`^the user "([^"]*)" has tier "(free|premium)"$`, stepUserHasTier)

	// --- Given: request building ---
	sc.Step(`^I set header "([^"]*)" to "([^"]*)"$`, stepSetHeader)
	sc.Step(`^I set query param "([^"]*)" to "([^"]*)"$`, stepSetQueryParam)
	sc.Step(`^a seeded bird is saved as "([^"]*)"$`, stepSeededBird)
	sc.Step(`^a string of (\d+) "([^"]*)" characters is saved as "([^"]*)"$`, stepSaveRepeatedString)

	// --- When: actions ---
	sc.Step(`^I make a (GET|POST|PUT|PATCH|DELETE) call to (\S+)$`, stepMakeCall)
	sc.Step(`^I make a (GET|POST|PUT|PATCH|DELETE) call to (\S+) with body$`, stepMakeCallWithBody)

	// --- Then: assertions ---
	sc.Step(`^I should receive a (\d+) response$`, stepStatus)
	sc.Step(`^I should receive a (\d+) JSON response$`, stepStatusJSON)
	sc.Step(`^the response body should be$`, stepBodyExact)
	sc.Step(`^the response body should contain$`, stepBodyContains)
	sc.Step(`^the response field "([^"]*)" should be "([^"]*)"$`, stepField)
	sc.Step(`^the response field "([^"]*)" should match "([^"]*)"$`, stepFieldMatch)
	sc.Step(`^the response header "([^"]*)" should be "([^"]*)"$`, stepHeaderEquals)
	sc.Step(`^the response header "([^"]*)" should not be empty$`, stepHeaderNotEmpty)
	sc.Step(`^I save the response under "([^"]*)"$`, stepSaveResponse)
	sc.Step(`^I save the response header "([^"]*)" as "([^"]*)"$`, stepSaveResponseHeaderAs)
}

// --- Given: identity ---

func stepIAmAnonymous(ctx context.Context) error {
	w := worldFrom(ctx)
	delete(w.headers, "Authorization")
	w.clearCurrentUserVars()
	return nil
}

func stepIAmAuthenticatedAs(ctx context.Context, username string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	w.clearCurrentUserVars()

	authID := authIDForUsername(username)
	token, err := env.signer.sign(authID, emailForUsername(username), nil)
	if err != nil {
		return fmt.Errorf("sign token for %q: %w", username, err)
	}
	w.headers["Authorization"] = "Bearer " + token
	w.vars["current_user.username"] = username
	w.vars["current_user.auth_id"] = authID
	return nil
}

// stepUserHasTier provisions the user via a real request, then updates their tier directly in the database, since tier is DB-side state that cannot be set via a request alone.
func stepUserHasTier(ctx context.Context, username, tier string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	authID := authIDForUsername(username)
	email := emailForUsername(username)

	token, err := env.signer.sign(authID, email, nil)
	if err != nil {
		return fmt.Errorf("sign token for %q: %w", username, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, env.baseURL+"/api/me", nil)
	if err != nil {
		return fmt.Errorf("build provisioning request for %q: %w", username, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := env.client.Do(req)
	if err != nil {
		return fmt.Errorf("provision %q via GET /api/me: %w", username, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provision %q via GET /api/me: status %d: %s", username, resp.StatusCode, body)
	}

	if _, err := env.db.ExecContext(ctx, `UPDATE users SET tier = $1 WHERE email = $2`, tier, email); err != nil {
		return fmt.Errorf("set tier %q for %q: %w", tier, username, err)
	}
	return nil
}

// --- Given: request building ---

func stepSetHeader(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	w.headers[name] = w.interpolate(value)
	return nil
}

func stepSetQueryParam(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	w.query[name] = w.interpolate(value)
	return nil
}

func stepSeededBird(ctx context.Context, varName string) error {
	w := worldFrom(ctx)
	var id string
	if err := env.db.GetContext(ctx, &id, `SELECT id FROM birds ORDER BY taxonomic_order LIMIT 1`); err != nil {
		return fmt.Errorf("select seeded bird: %w", err)
	}
	w.vars[varName] = id
	return nil
}

// stepSaveRepeatedString saves count copies of s as a variable, so scenarios can build bodies that exceed a length limit without bloating the feature file.
func stepSaveRepeatedString(ctx context.Context, count int, s, varName string) error {
	w := worldFrom(ctx)
	w.vars[varName] = strings.Repeat(s, count)
	return nil
}

// --- When: actions ---

func stepMakeCall(ctx context.Context, method, path string) error {
	return doCall(ctx, method, path, "")
}

func stepMakeCallWithBody(ctx context.Context, method, path string, body *godog.DocString) error {
	return doCall(ctx, method, path, body.Content)
}

// doCall sends one HTTP request against the running server, interpolating the path, query, and body and attaching pending headers, then stashes the response on World.
func doCall(ctx context.Context, method, rawPath, rawBody string) error {
	w := worldFrom(ctx)
	path := w.interpolate(rawPath)

	target := env.baseURL + path
	if len(w.query) > 0 {
		vals := url.Values{}
		for k, v := range w.query {
			vals.Set(k, v)
		}
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + vals.Encode()
	}

	var bodyBytes []byte
	var bodyReader io.Reader
	if rawBody != "" {
		bodyBytes = []byte(w.interpolate(rawBody))
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}

	hasContentType := false
	for name, value := range w.headers {
		req.Header.Set(name, value)
		if strings.EqualFold(name, "Content-Type") {
			hasContentType = true
		}
	}
	if len(bodyBytes) > 0 && !hasContentType {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := env.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body for %s %s: %w", method, path, err)
	}

	w.status = resp.StatusCode
	w.header = resp.Header
	w.body = respBody
	w.json = nil
	if len(respBody) > 0 {
		var decoded any
		if json.Unmarshal(respBody, &decoded) == nil {
			w.json = decoded
		}
	}
	w.afterCall()
	return nil
}

// --- Then: assertions ---

func stepStatus(ctx context.Context, status int) error {
	w := worldFrom(ctx)
	if w.status != status {
		return fmt.Errorf("expected status %d, got %d: %s", status, w.status, string(w.body))
	}
	return nil
}

func stepStatusJSON(ctx context.Context, status int) error {
	w := worldFrom(ctx)
	if err := stepStatus(ctx, status); err != nil {
		return err
	}
	ct := w.header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasSuffix(mediaType, "/json") {
		return fmt.Errorf("expected a JSON response, got Content-Type=%q", ct)
	}
	return nil
}

func stepBodyExact(ctx context.Context, doc *godog.DocString) error {
	w := worldFrom(ctx)
	raw := w.interpolate(doc.Content)
	var expected any
	if err := json.Unmarshal([]byte(raw), &expected); err != nil {
		return fmt.Errorf("expected body is not valid JSON: %w", err)
	}
	if !reflect.DeepEqual(expected, w.json) {
		expectedPretty, _ := json.MarshalIndent(expected, "", "  ")
		actualPretty, _ := json.MarshalIndent(w.json, "", "  ")
		return fmt.Errorf("body mismatch:\n--- expected ---\n%s\n--- actual ---\n%s", expectedPretty, actualPretty)
	}
	return nil
}

func stepBodyContains(ctx context.Context, doc *godog.DocString) error {
	w := worldFrom(ctx)
	raw := w.interpolate(doc.Content)
	var expected any
	if err := json.Unmarshal([]byte(raw), &expected); err != nil {
		return fmt.Errorf("expected body is not valid JSON: %w", err)
	}
	if err := assertSubset(expected, w.json, "$"); err != nil {
		return fmt.Errorf("%w (actual body: %s)", err, string(w.body))
	}
	return nil
}

func stepField(ctx context.Context, path, value string) error {
	w := worldFrom(ctx)
	actual, ok := getPath(w.json, path)
	if !ok {
		return fmt.Errorf("field %q not found in response: %s", path, string(w.body))
	}
	expected := coerceExpected(w.interpolate(value))
	if !fieldsEqual(actual, expected) {
		return fmt.Errorf("field %q: expected %#v, got %#v", path, expected, actual)
	}
	return nil
}

// stepFieldMatch asserts a response field matches a regex, e.g. a server-generated id's shape ("^usr_[a-z0-9]{26}$") that cannot be hard-coded.
func stepFieldMatch(ctx context.Context, path, pattern string) error {
	w := worldFrom(ctx)
	actual, ok := getPath(w.json, path)
	if !ok {
		return fmt.Errorf("field %q not found in response: %s", path, string(w.body))
	}
	re, err := regexp.Compile(w.interpolate(pattern))
	if err != nil {
		return fmt.Errorf("field %q: invalid pattern %q: %w", path, pattern, err)
	}
	s := stringify(actual)
	if !re.MatchString(s) {
		return fmt.Errorf("field %q: %q does not match pattern %q", path, s, pattern)
	}
	return nil
}

func stepHeaderEquals(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	got := w.header.Get(name)
	want := w.interpolate(value)
	if got != want {
		return fmt.Errorf("header %q: expected %q, got %q", name, want, got)
	}
	return nil
}

func stepHeaderNotEmpty(ctx context.Context, name string) error {
	w := worldFrom(ctx)
	if w.header.Get(name) == "" {
		return fmt.Errorf("header %q is empty", name)
	}
	return nil
}

func stepSaveResponse(ctx context.Context, prefix string) error {
	w := worldFrom(ctx)
	w.vars[prefix] = string(w.body)
	flattenInto(w.vars, prefix, w.json)
	return nil
}

// stepSaveResponseHeaderAs saves a response header into a variable, for round-tripping values like ETag into a later request.
func stepSaveResponseHeaderAs(ctx context.Context, name, varName string) error {
	w := worldFrom(ctx)
	w.vars[varName] = w.header.Get(name)
	return nil
}
