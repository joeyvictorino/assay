// Package canon produces RFC 8785 (JSON Canonicalization Scheme) output for
// the subset of JSON that assay signs and hashes: objects, arrays, strings,
// booleans, null and integers.
//
// Non-integer numbers are rejected. Every signature and hash in the system is
// computed over these bytes, so two independent implementations must agree
// byte for byte; floating point formatting is the one place where that is
// hard to guarantee, so it is simply not allowed.
//
// Ported from the canonical_bytes helper in sovereign-assist (same owner),
// tightened to RFC 8785 string and key ordering rules.
package canon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// ErrFloat is returned when a non-integer number is encountered.
var ErrFloat = errors.New("canon: non-integer numbers are not canonicalizable")

// ErrInvalidUTF8 is returned when a string is not valid UTF-8.
var ErrInvalidUTF8 = errors.New("canon: string is not valid UTF-8")

// maxSafeInt is 2^53; whole floats beyond it cannot be trusted to carry an
// exact integer, so they are rejected rather than silently rounded.
const maxSafeInt = 1 << 53

// Bytes returns the canonical JSON encoding of v.
//
// v may be nil, bool, string, any Go integer type, json.Number, a whole
// float32/float64, map[string]any, []any, or any other value that
// encoding/json can marshal (structs, typed maps and slices), which is
// round-tripped through encoding/json first so that struct tags apply.
func Bytes(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, v, 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Hash returns the lowercase hex SHA-256 of the canonical bytes of v.
func Hash(v any) (string, error) {
	b, err := Bytes(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

const maxDepth = 256

func encode(buf *bytes.Buffer, v any, depth int) error {
	if depth > maxDepth {
		return errors.New("canon: nesting too deep")
	}
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case string:
		return encodeString(buf, x)
	case json.Number:
		return encodeNumberString(buf, string(x))
	case int:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int8:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int16:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int32:
		buf.WriteString(strconv.FormatInt(int64(x), 10))
		return nil
	case int64:
		buf.WriteString(strconv.FormatInt(x, 10))
		return nil
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(x), 10))
		return nil
	case uint64:
		buf.WriteString(strconv.FormatUint(x, 10))
		return nil
	case float32:
		return encodeFloat(buf, float64(x))
	case float64:
		return encodeFloat(buf, x)
	case map[string]any:
		return encodeObject(buf, x, depth)
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encode(buf, e, depth+1); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	}
	// Anything else (structs, typed maps and slices, pointers, Marshalers)
	// goes through encoding/json so that struct tags and MarshalJSON apply,
	// then is decoded into the generic shape with numbers preserved as text.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		buf.WriteString("null")
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("canon: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return fmt.Errorf("canon: %w", err)
	}
	return encode(buf, generic, depth+1)
}

func encodeObject(buf *bytes.Buffer, m map[string]any, depth int) error {
	type kv struct {
		key   string
		units []uint16
	}
	keys := make([]kv, 0, len(m))
	for k := range m {
		if !utf8.ValidString(k) {
			return ErrInvalidUTF8
		}
		keys = append(keys, kv{key: k, units: utf16.Encode([]rune(k))})
	}
	// RFC 8785 section 3.2.3: sort by UTF-16 code units.
	sort.Slice(keys, func(i, j int) bool {
		return lessUTF16(keys[i].units, keys[j].units)
	})
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, k.key); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := encode(buf, m[k.key], depth+1); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func lessUTF16(a, b []uint16) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

const hexDigits = "0123456789abcdef"

// encodeString follows RFC 8785 section 3.2.2.2: escape only the quotation
// mark, the reverse solidus and control characters below U+0020. Everything
// else, including non-ASCII, is emitted literally as UTF-8.
func encodeString(buf *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return ErrInvalidUTF8
	}
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\t':
			buf.WriteString(`\t`)
		case '\n':
			buf.WriteString(`\n`)
		case '\f':
			buf.WriteString(`\f`)
		case '\r':
			buf.WriteString(`\r`)
		default:
			if c < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexDigits[c>>4])
				buf.WriteByte(hexDigits[c&0xf])
			} else {
				buf.WriteByte(c)
			}
		}
	}
	buf.WriteByte('"')
	return nil
}

func encodeFloat(buf *bytes.Buffer, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || math.Abs(f) > maxSafeInt {
		return ErrFloat
	}
	// -0 canonicalizes to 0 (RFC 8785 section 3.2.2.3).
	buf.WriteString(strconv.FormatInt(int64(f), 10))
	return nil
}

// encodeNumberString handles json.Number text. Plain integer literals of any
// size are emitted as-is (after normalizing -0); anything with a fraction or
// exponent is only accepted if it denotes a whole number within 2^53.
func encodeNumberString(buf *bytes.Buffer, s string) error {
	if s == "" {
		return ErrFloat
	}
	if n, ok := new(big.Int).SetString(s, 10); ok {
		buf.WriteString(n.String())
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ErrFloat
	}
	return encodeFloat(buf, f)
}
