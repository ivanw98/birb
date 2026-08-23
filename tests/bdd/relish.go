//go:build bdd

package bdd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/ivanw98/birb/internal/models"
)

// bddNamespace roots the deterministic per-username auth-id derivation below.
var bddNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("birb-bdd"))

// Deterministic fixture rows (the DB is truncated per scenario).
const (
	defaultUsername     = "default_user"
	defaultUserID       = "usr_01j9z3x8k2m4n6p8r0s2t4v6w8"
	defaultSightingID   = "sgh_01j9z3x8k2m4n6p8r0s2t4v6w8"
	defaultSightingTime = "2025-06-01T10:00:00Z"

	// bddJoinFailureLimit keeps the rate-limit scenario to a handful of requests.
	bddJoinFailureLimit = 2
)

// authIDForUsername deterministically derives a stable Supabase-style auth uid (a UUID) from a BDD username.
func authIDForUsername(username string) string {
	return uuid.NewSHA1(bddNamespace, []byte(username)).String()
}

func emailForUsername(username string) string {
	return username + "@example.test"
}

// steps binds the step library to the suite harness; per-scenario state lives in World, carried through the context.
type steps struct {
	env *harness
}

// InitializeScenario builds the initializer godog.TestSuite calls, registering the shared step library and per-scenario lifecycle hook over the harness.
func InitializeScenario(env *harness) func(*godog.ScenarioContext) {
	s := &steps{env: env}
	return func(sc *godog.ScenarioContext) {
		sc.Before(s.before)

		// --- Given: identity ---
		sc.Step(`^I am anonymous$`, s.iAmAnonymous)
		sc.Step(`^I am authenticated as "([^"]*)"$`, s.iAmAuthenticatedAs)
		sc.Step(`^I am authenticated as "([^"]*)" with display name "([^"]*)"$`, s.iAmAuthenticatedAsNamed)
		sc.Step(`^the user "([^"]*)" has tier "(free|premium)"$`, s.userHasTier)
		sc.Step(`^the default user exists$`, s.defaultUser)
		sc.Step(`^the user "([^"]*)" exists$`, s.userExists)
		sc.Step(`^the user "([^"]*)" exists with display name "([^"]*)"$`, s.userExistsNamed)

		// --- Given: groups ---
		sc.Step(`^a group "([^"]*)" owned by "([^"]*)" exists$`, s.groupExists)
		sc.Step(`^a group "([^"]*)" owned by "([^"]*)" exists with join code "([^"]*)"$`, s.groupExistsWithCode)
		sc.Step(`^"([^"]*)" is a member of group "([^"]*)"$`, s.userIsMemberOfGroup)
		sc.Step(`^group "([^"]*)" has (\d+) other members$`, s.groupHasNOtherMembers)
		sc.Step(`^"([^"]*)" is a member of (\d+) groups$`, s.userIsMemberOfNGroups)
		sc.Step(`^"([^"]*)" owns (\d+) groups$`, s.userOwnsNGroups)

		// --- Given: feed fixtures ---
		// Time-relative on purpose: the feed's window is seven days from now, and every
		// other fixture in this suite is pinned to 2025-06-01, well outside it.
		sc.Step(`^a sighting by "([^"]*)" observed (\d+) (hour|day)s? ago exists as "([^"]*)"$`, s.sightingAgo)
		sc.Step(`^a sighting by "([^"]*)" observed (\d+) (hour|day)s? from now exists as "([^"]*)"$`, s.sightingAhead)
		sc.Step(`^a sighting by "([^"]*)" at ([-\d.]+), ([-\d.]+) observed (\d+) hours? ago exists as "([^"]*)"$`, s.sightingAtCoords)
		sc.Step(`^"([^"]*)" has (\d+) sightings from the last week$`, s.userHasNSightings)
		sc.Step(`^the sighting "([^"]*)" is soft deleted$`, s.sightingSoftDeleted)
		sc.Step(`^the sighting "([^"]*)" has notes "([^"]*)" and quick note "([^"]*)"$`, s.sightingHasNotes)
		sc.Step(`^a place "([^"]*)" exists at ([-\d.]+), ([-\d.]+)$`, s.placeExists)
		sc.Step(`^a place "([^"]*)" exists at ([-\d.]+), ([-\d.]+) with population (\d+)$`, s.placeExistsWithPopulation)
		sc.Step(`^there are no places$`, s.noPlaces)

		// --- Given: request building ---
		sc.Step(`^I set header "([^"]*)" to "([^"]*)"$`, s.setHeader)
		sc.Step(`^I set query param "([^"]*)" to "([^"]*)"$`, s.setQueryParam)
		sc.Step(`^a seeded bird is saved as "([^"]*)"$`, s.seededBird)
		sc.Step(`^a string of (\d+) "([^"]*)" characters is saved as "([^"]*)"$`, s.saveRepeatedString)
		sc.Step(`^the default sighting exists$`, s.defaultSighting)

		// --- When: actions ---
		sc.Step(`^I make a (GET|POST|PUT|PATCH|DELETE) call to (\S+)$`, s.makeCall)
		sc.Step(`^I make a (GET|POST|PUT|PATCH|DELETE) call to (\S+) with body$`, s.makeCallWithBody)
		sc.Step(`^I make (\d+) failed join attempts$`, s.nFailedJoinAttempts)

		// --- Then: assertions ---
		sc.Step(`^I should receive a (\d+) response$`, s.status)
		sc.Step(`^I should receive a (\d+) JSON response$`, s.statusJSON)
		sc.Step(`^the response body should be$`, s.bodyExact)
		sc.Step(`^the response body should contain$`, s.bodyContains)
		sc.Step(`^the response field "([^"]*)" should be "([^"]*)"$`, s.field)
		sc.Step(`^the response field "([^"]*)" should be absent$`, s.fieldAbsent)
		sc.Step(`^the response field "([^"]*)" should not be "([^"]*)"$`, s.fieldNot)
		sc.Step(`^the response field "([^"]*)" should match "([^"]*)"$`, s.fieldMatch)
		sc.Step(`^the response field "([^"]*)" should have (\d+) items$`, s.fieldLength)
		sc.Step(`^the response should not contain "([^"]*)"$`, s.bodyNotContains)
		sc.Step(`^the response header "([^"]*)" should be "([^"]*)"$`, s.headerEquals)
		sc.Step(`^the response header "([^"]*)" should not be empty$`, s.headerNotEmpty)
		sc.Step(`^I save the response under "([^"]*)"$`, s.saveResponse)
		sc.Step(`^I save the response header "([^"]*)" as "([^"]*)"$`, s.saveResponseHeaderAs)
	}
}

func (s *steps) before(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	if err := s.env.resetDB(ctx); err != nil {
		return ctx, fmt.Errorf("reset db before scenario: %w", err)
	}
	return withWorld(ctx, newWorld()), nil
}

// --- Given: identity ---

func (s *steps) iAmAnonymous(ctx context.Context) error {
	w := worldFrom(ctx)
	delete(w.headers, "Authorization")
	w.clearCurrentUserVars()
	return nil
}

func (s *steps) iAmAuthenticatedAs(ctx context.Context, username string) error {
	return s.authenticateAs(ctx, username, nil)
}

// iAmAuthenticatedAsNamed puts a display name in the token. The JIT upsert rewrites
// display_name from the token on every request, so a name set only by a fixture INSERT is
// erased the moment that user calls the API.
func (s *steps) iAmAuthenticatedAsNamed(ctx context.Context, username, displayName string) error {
	w := worldFrom(ctx)
	name := w.interpolate(displayName)
	return s.authenticateAs(ctx, username, &name)
}

func (s *steps) authenticateAs(ctx context.Context, username string, displayName *string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	w.clearCurrentUserVars()

	authID := authIDForUsername(username)
	token, err := s.env.signer.sign(authID, emailForUsername(username), displayName)
	if err != nil {
		return fmt.Errorf("sign token for %q: %w", username, err)
	}
	w.headers["Authorization"] = "Bearer " + token
	w.vars["current_user.username"] = username
	w.vars["current_user.auth_id"] = authID
	return nil
}

// userHasTier provisions the user via a real request, then updates their tier directly in the database, since tier is DB-side state that cannot be set via a request alone.
func (s *steps) userHasTier(ctx context.Context, username, tier string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	authID := authIDForUsername(username)
	email := emailForUsername(username)

	token, err := s.env.signer.sign(authID, email, nil)
	if err != nil {
		return fmt.Errorf("sign token for %q: %w", username, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.env.baseURL+"/api/me", nil)
	if err != nil {
		return fmt.Errorf("build provisioning request for %q: %w", username, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.env.client.Do(req)
	if err != nil {
		return fmt.Errorf("provision %q via GET /api/me: %w", username, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provision %q via GET /api/me: status %d: %s", username, resp.StatusCode, body)
	}

	if _, err := s.env.db.ExecContext(ctx, `UPDATE users SET tier = $1 WHERE email = $2`, tier, email); err != nil {
		return fmt.Errorf("set tier %q for %q: %w", tier, username, err)
	}
	return nil
}

// --- Given: request building ---

func (s *steps) setHeader(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	w.headers[name] = w.interpolate(value)
	return nil
}

func (s *steps) setQueryParam(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	w.query[name] = w.interpolate(value)
	return nil
}

// defaultUser inserts the fixture user directly; kept separate from
// authentication so token-only auth (first-call provisioning) stays testable.
func (s *steps) defaultUser(ctx context.Context) error {
	w := worldFrom(ctx)
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO users (id, auth_id, email)
		VALUES ($1, $2, $3)`,
		defaultUserID, authIDForUsername(defaultUsername), emailForUsername(defaultUsername)); err != nil {
		return fmt.Errorf("insert default user: %w", err)
	}
	w.vars["default_user.id"] = defaultUserID
	return nil
}

// defaultSighting inserts the default sighting, owned by the default user.
func (s *steps) defaultSighting(ctx context.Context) error {
	w := worldFrom(ctx)
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO sightings (id, user_id, observed_at, observed_at_offset_minutes, client_updated_at, quick_note)
		VALUES ($1, $2, $3, 0, $3, 'default sighting')`,
		defaultSightingID, defaultUserID, defaultSightingTime); err != nil {
		return fmt.Errorf("insert default sighting (add 'Given the default user exists' first): %w", err)
	}

	w.vars["default_sighting.id"] = defaultSightingID
	w.vars["default_sighting.observedAt"] = defaultSightingTime
	w.vars["default_sighting.clientUpdatedAt"] = defaultSightingTime
	return nil
}

func (s *steps) seededBird(ctx context.Context, varName string) error {
	w := worldFrom(ctx)
	var id string
	if err := s.env.db.GetContext(ctx, &id, `SELECT id FROM birds ORDER BY taxonomic_order LIMIT 1`); err != nil {
		return fmt.Errorf("select seeded bird: %w", err)
	}
	w.vars[varName] = id
	return nil
}

// saveRepeatedString saves count copies of str as a variable, so scenarios can build bodies that exceed a length limit without bloating the feature file.
func (s *steps) saveRepeatedString(ctx context.Context, count int, str, varName string) error {
	w := worldFrom(ctx)
	w.vars[varName] = strings.Repeat(str, count)
	return nil
}

// --- When: actions ---

func (s *steps) makeCall(ctx context.Context, method, path string) error {
	return s.doCall(ctx, method, path, "")
}

func (s *steps) makeCallWithBody(ctx context.Context, method, path string, body *godog.DocString) error {
	return s.doCall(ctx, method, path, body.Content)
}

// doCall sends one HTTP request against the running server, interpolating the path, query, and body and attaching pending headers, then stashes the response on World.
func (s *steps) doCall(ctx context.Context, method, rawPath, rawBody string) error {
	w := worldFrom(ctx)
	path := w.interpolate(rawPath)

	target := s.env.baseURL + path
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

	resp, err := s.env.client.Do(req)
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

func (s *steps) status(ctx context.Context, status int) error {
	w := worldFrom(ctx)
	if w.status != status {
		return fmt.Errorf("expected status %d, got %d: %s", status, w.status, string(w.body))
	}
	return nil
}

func (s *steps) statusJSON(ctx context.Context, status int) error {
	w := worldFrom(ctx)
	if err := s.status(ctx, status); err != nil {
		return err
	}
	ct := w.header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasSuffix(mediaType, "/json") {
		return fmt.Errorf("expected a JSON response, got Content-Type=%q", ct)
	}
	return nil
}

func (s *steps) bodyExact(ctx context.Context, doc *godog.DocString) error {
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

func (s *steps) bodyContains(ctx context.Context, doc *godog.DocString) error {
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

func (s *steps) field(ctx context.Context, path, value string) error {
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

// fieldNot asserts a field is present but holds something other than the given value.
// Presence is required on purpose: "nextCursor should not be null" must fail if the
// field is missing entirely, not pass by absence.
func (s *steps) fieldNot(ctx context.Context, path, value string) error {
	w := worldFrom(ctx)
	actual, ok := getPath(w.json, path)
	if !ok {
		return fmt.Errorf("field %q not found in response: %s", path, string(w.body))
	}
	if fieldsEqual(actual, coerceExpected(w.interpolate(value))) {
		return fmt.Errorf("field %q should not be %s: %s", path, value, string(w.body))
	}
	return nil
}

// fieldAbsent asserts a dotted path is not present in the response, e.g. the omitted `deleted` field on a live sighting.
func (s *steps) fieldAbsent(ctx context.Context, path string) error {
	w := worldFrom(ctx)
	if v, ok := getPath(w.json, path); ok {
		return fmt.Errorf("field %q should be absent, got %#v: %s", path, v, string(w.body))
	}
	return nil
}

// fieldMatch asserts a response field matches a regex, e.g. a server-generated id's shape ("^usr_[a-z0-9]{26}$") that cannot be hard-coded.
func (s *steps) fieldMatch(ctx context.Context, path, pattern string) error {
	w := worldFrom(ctx)
	actual, ok := getPath(w.json, path)
	if !ok {
		return fmt.Errorf("field %q not found in response: %s", path, string(w.body))
	}
	re, err := regexp.Compile(w.interpolate(pattern))
	if err != nil {
		return fmt.Errorf("field %q: invalid pattern %q: %w", path, pattern, err)
	}
	str := stringify(actual)
	if !re.MatchString(str) {
		return fmt.Errorf("field %q: %q does not match pattern %q", path, str, pattern)
	}
	return nil
}

func (s *steps) headerEquals(ctx context.Context, name, value string) error {
	w := worldFrom(ctx)
	got := w.header.Get(name)
	want := w.interpolate(value)
	if got != want {
		return fmt.Errorf("header %q: expected %q, got %q", name, want, got)
	}
	return nil
}

func (s *steps) headerNotEmpty(ctx context.Context, name string) error {
	w := worldFrom(ctx)
	if w.header.Get(name) == "" {
		return fmt.Errorf("header %q is empty", name)
	}
	return nil
}

func (s *steps) saveResponse(ctx context.Context, prefix string) error {
	w := worldFrom(ctx)
	w.vars[prefix] = string(w.body)
	flattenInto(w.vars, prefix, w.json)
	return nil
}

// fieldLength asserts an array's length; a path of "$" means the response root, for endpoints that return a bare array.
func (s *steps) fieldLength(ctx context.Context, path string, want int) error {
	w := worldFrom(ctx)
	actual := w.json
	if path != "$" {
		var ok bool
		if actual, ok = getPath(w.json, path); !ok {
			return fmt.Errorf("field %q not found in response: %s", path, string(w.body))
		}
	}
	arr, ok := actual.([]any)
	if !ok {
		return fmt.Errorf("field %q is not an array, got %#v", path, actual)
	}
	if len(arr) != want {
		return fmt.Errorf("field %q: expected %d items, got %d: %s", path, want, len(arr), string(w.body))
	}
	return nil
}

// bodyNotContains asserts a raw substring is absent anywhere in the response.
// It is the only assertion that catches a private field leaking somewhere the
// scenario did not think to look.
func (s *steps) bodyNotContains(ctx context.Context, needle string) error {
	w := worldFrom(ctx)
	want := w.interpolate(needle)
	if strings.Contains(string(w.body), want) {
		return fmt.Errorf("response should not contain %q: %s", want, string(w.body))
	}
	return nil
}

// saveResponseHeaderAs saves a response header into a variable, for round-tripping values like ETag into a later request.
func (s *steps) saveResponseHeaderAs(ctx context.Context, name, varName string) error {
	w := worldFrom(ctx)
	w.vars[varName] = w.header.Get(name)
	return nil
}

// --- group fixtures: rows are written straight to the database so a scenario
// arranges membership without exercising the endpoints it is there to test ---

// joinCodeAlphabet mirrors the server's confusable-free set (no 0, O, 1, I, L, U).
const joinCodeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

// idForName derives a stable prefixed id from a fixture name, so a scenario can
// reference a row whose id it never sees. Hex digits are a subset of the [a-z0-9]
// the CHECK constraints allow.
func idForName(prefix, name string) string {
	sum := sha256.Sum256([]byte(prefix + name))
	return prefix + hex.EncodeToString(sum[:])[:26]
}

func userIDForUsername(username string) string {
	// Keep the two user fixtures in agreement: "the default user exists" hard-codes its id.
	if username == defaultUsername {
		return defaultUserID
	}
	return idForName(models.PrefixUser, username)
}

// joinCodeForName derives a deterministic, well-formed join code for a fixture group.
func joinCodeForName(name string) string {
	sum := sha256.Sum256([]byte("joincode:" + name))
	out := make([]byte, 8)
	for i := range out {
		out[i] = joinCodeAlphabet[int(sum[i])%len(joinCodeAlphabet)]
	}
	return string(out)
}

// upsertUser writes a fixture user. Idempotent on auth_id so a group fixture can
// conjure its owner without the scenario having to declare them first, and so the
// JIT provisioning upsert later finds the same row.
func (s *steps) upsertUser(ctx context.Context, username string, displayName *string) (string, error) {
	id := userIDForUsername(username)
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO users (id, auth_id, email, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (auth_id) DO UPDATE
		SET display_name = COALESCE(EXCLUDED.display_name, users.display_name)`,
		id, authIDForUsername(username), emailForUsername(username), displayName); err != nil {
		return "", fmt.Errorf("insert user %q: %w", username, err)
	}
	return id, nil
}

func (s *steps) insertGroup(ctx context.Context, groupName, ownerUsername, joinCode string) (string, error) {
	ownerID, err := s.upsertUser(ctx, ownerUsername, nil)
	if err != nil {
		return "", err
	}
	id := idForName(models.PrefixGroup, groupName)
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO groups (id, name, owner_user_id, join_code)
		VALUES ($1, $2, $3, $4)`,
		id, groupName, ownerID, joinCode); err != nil {
		return "", fmt.Errorf("insert group %q: %w", groupName, err)
	}
	if err := s.addMember(ctx, id, ownerID); err != nil {
		return "", err
	}
	return id, nil
}

func (s *steps) addMember(ctx context.Context, groupID, userID string) error {
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (group_id, user_id) DO NOTHING`,
		groupID, userID); err != nil {
		return fmt.Errorf("add member %q to group %q: %w", userID, groupID, err)
	}
	return nil
}

// --- Given: users ---

func (s *steps) userExists(ctx context.Context, username string) error {
	return s.saveUser(ctx, username, nil)
}

func (s *steps) userExistsNamed(ctx context.Context, username, displayName string) error {
	return s.saveUser(ctx, username, &displayName)
}

func (s *steps) saveUser(ctx context.Context, username string, displayName *string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	id, err := s.upsertUser(ctx, username, displayName)
	if err != nil {
		return err
	}
	w.vars["user."+username+".id"] = id
	return nil
}

// --- Given: groups ---

func (s *steps) groupExists(ctx context.Context, groupName, ownerUsername string) error {
	return s.saveGroup(ctx, groupName, ownerUsername, "")
}

func (s *steps) groupExistsWithCode(ctx context.Context, groupName, ownerUsername, joinCode string) error {
	return s.saveGroup(ctx, groupName, ownerUsername, joinCode)
}

func (s *steps) saveGroup(ctx context.Context, groupName, ownerUsername, joinCode string) error {
	w := worldFrom(ctx)
	groupName = w.interpolate(groupName)
	ownerUsername = w.interpolate(ownerUsername)
	if joinCode == "" {
		joinCode = joinCodeForName(groupName)
	} else {
		joinCode = strings.ToUpper(w.interpolate(joinCode))
	}

	id, err := s.insertGroup(ctx, groupName, ownerUsername, joinCode)
	if err != nil {
		return err
	}
	w.vars["user."+ownerUsername+".id"] = userIDForUsername(ownerUsername)
	w.vars["group."+groupName+".id"] = id
	w.vars["group."+groupName+".joinCode"] = joinCode
	return nil
}

func (s *steps) userIsMemberOfGroup(ctx context.Context, username, groupName string) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)
	groupName = w.interpolate(groupName)

	userID, err := s.upsertUser(ctx, username, nil)
	if err != nil {
		return err
	}
	w.vars["user."+username+".id"] = userID
	return s.addMember(ctx, idForName(models.PrefixGroup, groupName), userID)
}

// --- Given: cap fixtures ---

// groupHasNOtherMembers fills a group towards the members-per-group cap.
func (s *steps) groupHasNOtherMembers(ctx context.Context, groupName string, n int) error {
	w := worldFrom(ctx)
	groupName = w.interpolate(groupName)
	groupID := idForName(models.PrefixGroup, groupName)

	for i := range n {
		userID, err := s.upsertUser(ctx, fmt.Sprintf("%s_member_%d", groupName, i), nil)
		if err != nil {
			return err
		}
		if err := s.addMember(ctx, groupID, userID); err != nil {
			return err
		}
	}
	return nil
}

// userIsMemberOfNGroups fills a user towards the memberships-per-user cap, using
// groups owned by somebody else so the owned-groups cap stays untouched.
func (s *steps) userIsMemberOfNGroups(ctx context.Context, username string, n int) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)

	userID, err := s.upsertUser(ctx, username, nil)
	if err != nil {
		return err
	}
	w.vars["user."+username+".id"] = userID

	for i := range n {
		name := fmt.Sprintf("%s_membership_%d", username, i)
		groupID, err := s.insertGroup(ctx, name, fmt.Sprintf("%s_host_%d", username, i), joinCodeForName(name))
		if err != nil {
			return err
		}
		if err := s.addMember(ctx, groupID, userID); err != nil {
			return err
		}
	}
	return nil
}

// userOwnsNGroups fills a user towards the owned-groups cap.
func (s *steps) userOwnsNGroups(ctx context.Context, username string, n int) error {
	w := worldFrom(ctx)
	username = w.interpolate(username)

	for i := range n {
		name := fmt.Sprintf("%s_owned_%d", username, i)
		if _, err := s.insertGroup(ctx, name, username, joinCodeForName(name)); err != nil {
			return err
		}
	}
	return nil
}

// --- Given: feed fixtures ---

// insertSighting writes one fixture sighting.
func (s *steps) insertSighting(ctx context.Context, w *World, author, name string, offset time.Duration, lat, lon *float64) error {
	authorID, err := s.upsertUser(ctx, author, nil)
	if err != nil {
		return err
	}
	id := idForName(models.PrefixSighting, name)
	observedAt := time.Now().Add(offset)

	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO sightings (id, user_id, observed_at, observed_at_offset_minutes, client_updated_at, latitude, longitude)
		VALUES ($1, $2, $3, 0, now(), $4, $5)`,
		id, authorID, observedAt, lat, lon); err != nil {
		return fmt.Errorf("insert sighting %q: %w", name, err)
	}

	w.vars["sighting."+name+".id"] = id
	w.vars["user."+author+".id"] = authorID
	return nil
}

func unitDuration(n int, unit string) time.Duration {
	if unit == "day" {
		return time.Duration(n) * 24 * time.Hour
	}
	return time.Duration(n) * time.Hour
}

func (s *steps) sightingAgo(ctx context.Context, author string, n int, unit, name string) error {
	w := worldFrom(ctx)
	return s.insertSighting(ctx, w, w.interpolate(author), name, -unitDuration(n, unit), nil, nil)
}

func (s *steps) sightingAhead(ctx context.Context, author string, n int, unit, name string) error {
	w := worldFrom(ctx)
	return s.insertSighting(ctx, w, w.interpolate(author), name, unitDuration(n, unit), nil, nil)
}

func (s *steps) sightingAtCoords(ctx context.Context, author, lat, lon string, n int, name string) error {
	w := worldFrom(ctx)
	latitude, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return fmt.Errorf("latitude %q is not a number: %w", lat, err)
	}
	longitude, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		return fmt.Errorf("longitude %q is not a number: %w", lon, err)
	}
	return s.insertSighting(ctx, w, w.interpolate(author), name, -unitDuration(n, "hour"), &latitude, &longitude)
}

// userHasNSightings fills a feed past a page boundary. Each is an hour older than the
// last so the keyset order is total - walk is deterministic.
func (s *steps) userHasNSightings(ctx context.Context, author string, n int) error {
	w := worldFrom(ctx)
	author = w.interpolate(author)
	for i := range n {
		name := fmt.Sprintf("%s_bulk_%d", author, i)
		if err := s.insertSighting(ctx, w, author, name, -unitDuration(i+1, "hour"), nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *steps) sightingSoftDeleted(ctx context.Context, name string) error {
	w := worldFrom(ctx)
	id := idForName(models.PrefixSighting, w.interpolate(name))
	if _, err := s.env.db.ExecContext(ctx, `UPDATE sightings SET deleted_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("soft delete sighting %q: %w", name, err)
	}
	return nil
}

// sightingHasNotes exists so the privacy scenarios can prove the projection drops text
// the caller never should see.
func (s *steps) sightingHasNotes(ctx context.Context, name, notes, quickNote string) error {
	w := worldFrom(ctx)
	id := idForName(models.PrefixSighting, w.interpolate(name))
	if _, err := s.env.db.ExecContext(ctx, `UPDATE sightings SET notes = $2, quick_note = $3 WHERE id = $1`,
		id, notes, quickNote); err != nil {
		return fmt.Errorf("set notes on sighting %q: %w", name, err)
	}
	return nil
}

// geonamesIDForName keeps fixture places unique without colliding.
func geonamesIDForName(name string) int {
	sum := sha256.Sum256([]byte("geonames:" + name))
	return int(binary.BigEndian.Uint32(sum[:4])>>1) + 1
}

func (s *steps) placeExists(ctx context.Context, name, lat, lon string) error {
	return s.savePlace(ctx, name, lat, lon, 500)
}

func (s *steps) placeExistsWithPopulation(ctx context.Context, name, lat, lon string, population int) error {
	return s.savePlace(ctx, name, lat, lon, population)
}

func (s *steps) savePlace(ctx context.Context, name, lat, lon string, population int) error {
	w := worldFrom(ctx)
	name = w.interpolate(name)
	if _, err := s.env.db.ExecContext(ctx, `
		INSERT INTO places (id, geonames_id, name, latitude, longitude, population, feature_code)
		VALUES ($1, $2, $3, $4, $5, $6, 'PPL')`,
		idForName(models.PrefixPlace, name), geonamesIDForName(name), name, lat, lon, population); err != nil {
		return fmt.Errorf("insert place %q: %w", name, err)
	}
	return nil
}

func (s *steps) noPlaces(ctx context.Context) error {
	if _, err := s.env.db.ExecContext(ctx, `TRUNCATE places`); err != nil {
		return fmt.Errorf("truncate places: %w", err)
	}
	return nil
}

// --- When: bulk actions ---

// nFailedJoinAttempts burns n rate-limit slots with codes that cannot match a group.
func (s *steps) nFailedJoinAttempts(ctx context.Context, n int) error {
	for i := range n {
		body := fmt.Sprintf(`{"code": "%s"}`, joinCodeForName(fmt.Sprintf("miss_%d", i)))
		if err := s.doCall(ctx, "POST", "/api/groups/join", body); err != nil {
			return fmt.Errorf("failed join attempt %d: %w", i, err)
		}
	}
	return nil
}

// --- World: the per-scenario state that the steps above read and mutate ---

// errAt formats a path-prefixed assertion failure, e.g. "$.results[0]: …".
func errAt(path, format string, args ...any) error {
	return fmt.Errorf("%s: %s", path, fmt.Sprintf(format, args...))
}

// World is per-scenario state (pending request config, identity, last response, and interpolation variables), reset fresh by the Before hook so scenarios never see each other's state.
type World struct {
	// pending request state: headers (including Authorization) persist across calls in a scenario; query is one-shot, cleared after every call (see afterCall).
	headers map[string]string
	query   map[string]string

	// last response.
	status int
	header http.Header
	body   []byte
	json   any // body decoded into map[string]any/[]any/scalars; nil if empty or not JSON
	// saved variables for {{ dotted.path }} interpolation, populated by the "authenticated as", "seeded bird", and "save the response" steps.
	vars map[string]any
}

func newWorld() *World {
	return &World{
		headers: map[string]string{},
		query:   map[string]string{},
		vars:    map[string]any{},
	}
}

// afterCall clears one-shot request state (query params) after a request is sent; headers persist for the rest of the scenario.
func (w *World) afterCall() {
	w.query = map[string]string{}
}

// clearCurrentUserVars drops every "current_user.*" variable so re-authenticating (or going anonymous) never leaks the previous identity's saved fields.
func (w *World) clearCurrentUserVars() {
	for k := range w.vars {
		if strings.HasPrefix(k, "current_user.") {
			delete(w.vars, k)
		}
	}
}

// --- context plumbing ---

type worldKey struct{}

func withWorld(ctx context.Context, w *World) context.Context {
	return context.WithValue(ctx, worldKey{}, w)
}

func worldFrom(ctx context.Context) *World {
	w, ok := ctx.Value(worldKey{}).(*World)
	if !ok {
		panic("bdd: no World in context; step registered outside the scenario Before hook")
	}
	return w
}

// --- {{ dotted.path }} interpolation ---

var templateRe = regexp.MustCompile(`\{\{\s*([\w.-]+)\s*\}\}`)

// interpolate replaces every {{ dotted.path }} placeholder in s with the string form of the matching variable, leaving unmatched placeholders verbatim.
func (w *World) interpolate(s string) string {
	return templateRe.ReplaceAllStringFunc(s, func(m string) string {
		key := templateRe.FindStringSubmatch(m)[1]
		if v, ok := w.vars[key]; ok {
			return stringify(v)
		}
		return m
	})
}

// stringify renders a JSON-decoded value (string/bool/float64/nil/etc.) as
// text for interpolation into a path, header, query param, or docstring.
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// --- dotted-path JSON field access (supports array indices) ---

// getPath walks a JSON-decoded value (map[string]any/[]any/scalars) along a dotted path, e.g. "results.0.status", returning ok=false if the path runs past the shape.
func getPath(obj any, path string) (any, bool) {
	cur := obj
	for _, part := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			nv, ok := v[part]
			if !ok {
				return nil, false
			}
			cur = nv
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// coerceExpected parses an already-interpolated expected string as JSON when possible, so numbers/bools/null compare by native type, falling back to the raw string for bare words like "free" or "created".
func coerceExpected(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

// fieldsEqual compares an actual decoded JSON value against a coerced
// expected value with the same rules encoding/json would use on both sides.
func fieldsEqual(actual, expected any) bool {
	return reflect.DeepEqual(actual, expected)
}

// --- subset / exact body matching ---

// assertSubset checks that every key in expected is present with a recursively-matching value in actual (unlisted keys ignored); arrays must match length and compare element-wise, and scalars (including null) compare by equality.
func assertSubset(expected, actual any, path string) error {
	switch ev := expected.(type) {
	case map[string]any:
		av, ok := actual.(map[string]any)
		if !ok {
			return errAt(path, "expected an object, got %T (%v)", actual, actual)
		}
		for k, v := range ev {
			nv, ok := av[k]
			if !ok {
				return errAt(path, "missing key %q", k)
			}
			if err := assertSubset(v, nv, path+"."+k); err != nil {
				return err
			}
		}
		return nil
	case []any:
		av, ok := actual.([]any)
		if !ok {
			return errAt(path, "expected an array, got %T (%v)", actual, actual)
		}
		if len(ev) != len(av) {
			return errAt(path, "length %d != %d", len(ev), len(av))
		}
		for i := range ev {
			if err := assertSubset(ev[i], av[i], path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
		return nil
	default:
		if !fieldsEqual(actual, expected) {
			return errAt(path, "%#v != %#v", expected, actual)
		}
		return nil
	}
}

// --- flattening a response body into dotted/indexed variables ---

// flattenInto stores every leaf (non-object, non-array) value under its dotted/indexed path relative to parent; container nodes are not stored themselves.
func flattenInto(vars map[string]any, parent string, v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			flattenInto(vars, joinPath(parent, k), vv)
		}
	case []any:
		for i, vv := range x {
			flattenInto(vars, joinPath(parent, strconv.Itoa(i)), vv)
		}
	default:
		if parent != "" {
			vars[parent] = x
		}
	}
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}
