package canon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, src string) any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("fixture %q: %v", src, err)
	}
	return v
}

func TestRFC8785Vectors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// RFC 8785 section 3.2.3 sample with the floating point array removed.
		{
			name: "literals and string escapes",
			in:   `{"string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/", "literals": [null, true, false]}`,
			want: `{"literals":[null,true,false],"string":"€$\u000f\nA'B\"\\\\\"/"}`,
		},
		// RFC 8785 section 3.2.3 key ordering sample (UTF-16 code unit order:
		// the emoji's surrogate pair sorts before U+FB33).
		{
			name: "utf16 key ordering",
			in: `{
  "\u20ac": "Euro Sign",
  "\r": "Carriage Return",
  "\u000a": "Newline",
  "1": "One",
  "\u0080": "Control\u007f",
  "\ud83d\ude02": "Smiley",
  "\u00f6": "Latin Small Letter O With Diaeresis",
  "\ufb33": "Hebrew Letter Dalet With Dagesh",
  "</script>": "Browser Challenge"
}`,
			want: "{\"\\n\":\"Newline\",\"\\r\":\"Carriage Return\",\"1\":\"One\",\"</script>\":\"Browser Challenge\",\"\u0080\":\"Control\u007f\",\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\",\"\U0001F602\":\"Smiley\",\"\uFB33\":\"Hebrew Letter Dalet With Dagesh\"}",
		},
		{
			name: "integers incl. large and negative zero",
			in:   `{"a": 0, "b": -0, "c": 9007199254740993, "d": -12, "e": 1.0, "f": 1e3}`,
			want: `{"a":0,"b":0,"c":9007199254740993,"d":-12,"e":1,"f":1000}`,
		},
		{
			name: "nested arrays preserve order",
			in:   `[3, [2, [1, {}]], "x"]`,
			want: `[3,[2,[1,{}]],"x"]`,
		},
		{
			name: "no html or slash escaping",
			in:   `{"h": "<a href=\"/\">&amp;</a>"}`,
			want: `{"h":"<a href=\"/\">&amp;</a>"}`,
		},
		{
			name: "control characters use lowercase hex",
			in:   `"\u0001\u001f\u001F\u007f"`,
			want: "\"\\u0001\\u001f\\u001f\u007f\"",
		},
		{
			name: "keys that are prefixes sort shorter first",
			in:   `{"ab": 1, "a": 2, "abc": 3, "": 4}`,
			want: `{"":4,"a":2,"ab":1,"abc":3}`,
		},
		{
			name: "empty containers",
			in:   `{"a": {}, "b": []}`,
			want: `{"a":{},"b":[]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Bytes(mustJSON(t, tc.in))
			if err != nil {
				t.Fatalf("Bytes: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestFloatsRejected(t *testing.T) {
	cases := []struct {
		name string
		in   any
	}{
		{"float64", 1.5},
		{"float32", float32(0.25)},
		{"json.Number fraction", json.Number("2.5")},
		{"json.Number exponent fraction", json.Number("1.25e1")},
		{"json.Number garbage", json.Number("abc")},
		{"nested in map", map[string]any{"x": 0.1}},
		{"nested in array", []any{1, 2.5}},
		{"struct field", struct {
			F float64 `json:"f"`
		}{F: 3.14}},
		{"too large whole float", 1e300},
		{"above 2^53", float64(1<<53 + 2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Bytes(tc.in)
			if !errors.Is(err, ErrFloat) {
				t.Fatalf("err = %v, want ErrFloat", err)
			}
		})
	}
}

func TestGoNativeValues(t *testing.T) {
	type inner struct {
		B bool   `json:"b"`
		S string `json:"s,omitempty"`
	}
	type outer struct {
		Z     int               `json:"z"`
		A     []string          `json:"a"`
		In    inner             `json:"in"`
		Ptr   *inner            `json:"ptr"`
		M     map[string]uint16 `json:"m"`
		Skip  string            `json:"-"`
		Whole float64           `json:"whole"`
	}
	v := outer{Z: -7, A: []string{"y", "x"}, In: inner{B: true}, M: map[string]uint16{"k2": 2, "k1": 1}, Skip: "no", Whole: 42}
	got, err := Bytes(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":["y","x"],"in":{"b":true},"m":{"k1":1,"k2":2},"ptr":null,"whole":42,"z":-7}`
	if string(got) != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}

	prims := []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{true, "true"},
		{int8(-1), "-1"},
		{uint64(1<<64 - 1), "18446744073709551615"},
		{json.Number("-0"), "0"},
		{float64(-0.0), "0"},
		{"plain", `"plain"`},
		{(*inner)(nil), "null"},
		{[]string{"b", "a"}, `["b","a"]`},
	}
	for _, p := range prims {
		got, err := Bytes(p.in)
		if err != nil {
			t.Fatalf("%v: %v", p.in, err)
		}
		if string(got) != p.want {
			t.Fatalf("%v: got %s want %s", p.in, got, p.want)
		}
	}
}

func TestInvalidUTF8Rejected(t *testing.T) {
	bad := string([]byte{0xff, 'a'})
	if _, err := Bytes(bad); !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("string: %v", err)
	}
	if _, err := Bytes(map[string]any{bad: 1}); !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("key: %v", err)
	}
}

func TestHashIsStableAndOrderIndependent(t *testing.T) {
	a := map[string]any{"b": 1, "a": []any{"x", nil}}
	b := map[string]any{"a": []any{"x", nil}, "b": 1}
	ha, err := Hash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hash differs: %s vs %s", ha, hb)
	}
	sum := sha256.Sum256([]byte(`{"a":["x",null],"b":1}`))
	if want := hex.EncodeToString(sum[:]); ha != want {
		t.Fatalf("hash %s want %s", ha, want)
	}
	if _, err := Hash(1.5); !errors.Is(err, ErrFloat) {
		t.Fatalf("Hash float: %v", err)
	}
}
