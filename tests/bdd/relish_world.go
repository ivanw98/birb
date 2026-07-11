//go:build bdd

// relish_world.go holds the per-scenario World state that the step definitions in relish.go read and mutate.
package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// errAt formats a path-prefixed assertion failure, e.g. "$.results[0]: …".
func errAt(path, format string, args ...any) error {
	return fmt.Errorf("%s: %s", path, fmt.Sprintf(format, args...))
}

// World is per-scenario state — pending request config, identity, last response, and interpolation variables — reset fresh by the Before hook so scenarios never see each other's state.
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
