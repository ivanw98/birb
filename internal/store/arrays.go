package store

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// StringArray adapts a Postgres text[] to/from a Go []string at the database/sql boundary; callers must read with a ::text cast and write with a ::text[] cast.
type StringArray []string

// Value renders the slice as a Postgres array literal, double-quoting every element; a nil/empty slice becomes `{}`.
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		// Escape backslash and double-quote per Postgres array literal rules.
		for _, r := range s {
			if r == '\\' || r == '"' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

// Scan parses a Postgres array literal (text form) into the slice, accepting string or []byte with quoted or unquoted elements.
func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("StringArray: cannot scan %T", src)
	}
	parsed, err := parsePGTextArray(s)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// parsePGTextArray parses the subset of Postgres array literal syntax we emit
// and that Postgres emits for text[]: `{}`, `{a,b}`, `{"a b","c,d"}`, with
// backslash escapes inside quotes.
func parsePGTextArray(s string) (StringArray, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, fmt.Errorf("StringArray: malformed array literal %q", s)
	}
	body := s[1 : len(s)-1]
	if strings.TrimSpace(body) == "" {
		return StringArray{}, nil
	}

	var (
		out     StringArray
		cur     strings.Builder
		inQuote bool
		escaped bool
		hadElem bool
	)
	flush := func() {
		out = append(out, cur.String())
		cur.Reset()
		hadElem = false
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case escaped:
			cur.WriteByte(c)
			escaped = false
			hadElem = true
		case c == '\\':
			escaped = true
			hadElem = true
		case c == '"':
			inQuote = !inQuote
			hadElem = true
		case c == ',' && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
			hadElem = true
		}
	}
	if inQuote || escaped {
		return nil, fmt.Errorf("StringArray: unterminated element in %q", s)
	}
	_ = hadElem
	flush()
	return out, nil
}
