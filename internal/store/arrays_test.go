package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringArrayValue(t *testing.T) {
	cases := []struct {
		name string
		in   StringArray
		want string
	}{
		{"empty", StringArray{}, "{}"},
		{"nil", nil, "{}"},
		{"single", StringArray{"a"}, `{"a"}`},
		{"multi", StringArray{"a", "b"}, `{"a","b"}`},
		{"path", StringArray{"uid/sgh_1/p.jpg"}, `{"uid/sgh_1/p.jpg"}`},
		{"escapes", StringArray{`a"b\c`}, `{"a\"b\\c"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := c.in.Value()
			require.NoError(t, err)
			assert.Equal(t, c.want, v)
		})
	}
}

func TestStringArrayScan(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want StringArray
	}{
		{"nil", nil, nil},
		{"empty", "{}", StringArray{}},
		{"empty bytes", []byte("{}"), StringArray{}},
		{"unquoted", "{a,b}", StringArray{"a", "b"}},
		{"quoted", `{"a b","c,d"}`, StringArray{"a b", "c,d"}},
		{"escaped", `{"a\"b\\c"}`, StringArray{`a"b\c`}},
		{"path", `{uid/sgh_1/p.jpg}`, StringArray{"uid/sgh_1/p.jpg"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var a StringArray
			require.NoError(t, a.Scan(c.in))
			assert.Equal(t, c.want, a)
		})
	}
}

func TestStringArrayScanRoundTrip(t *testing.T) {
	orig := StringArray{"uid/sgh_abc/1.jpg", "weird, \"name\"\\x", "unicode—é"}
	v, err := orig.Value()
	require.NoError(t, err)
	var back StringArray
	require.NoError(t, back.Scan(v))
	assert.Equal(t, orig, back)
}

func TestStringArrayScanErrors(t *testing.T) {
	var a StringArray
	assert.Error(t, a.Scan(123), "unsupported type")
	assert.Error(t, a.Scan("no braces"), "missing braces")
	assert.Error(t, a.Scan(`{"unterminated}`), "unterminated quote")
}
