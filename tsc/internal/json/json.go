//nolint:depguard
package json

import (
	"io"
	"slices"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

var allowInvalid []json.Options = slices.Clip([]json.Options{jsontext.AllowInvalidUTF8(true)})

// JoinOptions combines options using the official JSON implementation.
func JoinOptions(opts ...json.Options) json.Options {
	return json.JoinOptions(opts...)
}

// MarshalToFunc delegates type-specific streaming marshaling to the official API.
func MarshalToFunc[T any](fn func(*Encoder, T) error) *Marshalers {
	return json.MarshalToFunc(fn)
}

// WithMarshalers applies the official type-specific marshaler option.
func WithMarshalers(value *Marshalers) json.Options {
	return json.WithMarshalers(value)
}

func Marshal(in any, opts ...json.Options) (out []byte, err error) {
	if len(opts) == 0 {
		opts = allowInvalid
	} else {
		opts = append(allowInvalid, opts...)
	}
	return json.Marshal(in, opts...)
}

func MarshalEncode(out *jsontext.Encoder, in any, opts ...json.Options) (err error) {
	if len(opts) == 0 {
		opts = allowInvalid
	} else {
		opts = append(allowInvalid, opts...)
	}
	return json.MarshalEncode(out, in, opts...)
}

func MarshalWrite(out io.Writer, in any, opts ...json.Options) (err error) {
	if len(opts) == 0 {
		opts = allowInvalid
	} else {
		opts = append(allowInvalid, opts...)
	}
	return json.MarshalWrite(out, in, opts...)
}

func MarshalIndent(in any, prefix, indent string) (out []byte, err error) {
	if prefix == "" && indent == "" {
		// WithIndentPrefix and WithIndent imply multiline output, so skip them.
		return Marshal(in)
	}
	return Marshal(in, jsontext.WithIndentPrefix(prefix), jsontext.WithIndent(indent))
}

func MarshalIndentWrite(out io.Writer, in any, prefix, indent string) (err error) {
	if prefix == "" && indent == "" {
		// WithIndentPrefix and WithIndent imply multiline output, so skip them.
		return MarshalWrite(out, in)
	}
	return MarshalWrite(out, in, jsontext.WithIndentPrefix(prefix), jsontext.WithIndent(indent))
}

func Unmarshal(in []byte, out any, opts ...json.Options) (err error) {
	return json.Unmarshal(in, out, opts...)
}

func UnmarshalDecode(in *jsontext.Decoder, out any, opts ...json.Options) (err error) {
	return json.UnmarshalDecode(in, out, opts...)
}

func UnmarshalRead(in io.Reader, out any, opts ...json.Options) (err error) {
	return json.UnmarshalRead(in, out, opts...)
}

// AllowInvalidUTF8 applies the official UTF-8 encoding and decoding policy.
func AllowInvalidUTF8(allow bool) json.Options {
	return jsontext.AllowInvalidUTF8(allow)
}

func AllowDuplicateNames(allow bool) json.Options {
	return jsontext.AllowDuplicateNames(allow)
}

func Deterministic(v bool) json.Options {
	return json.Deterministic(v)
}

func WithIndent(indent string) json.Options {
	return jsontext.WithIndent(indent)
}

func NewDecoder(r io.Reader) *jsontext.Decoder {
	return jsontext.NewDecoder(r)
}

type (
	Value           = jsontext.Value
	Kind            = jsontext.Kind
	UnmarshalerFrom = json.UnmarshalerFrom
	Marshalers      = json.Marshalers
	MarshalerTo     = json.MarshalerTo
	Decoder         = jsontext.Decoder
	Encoder         = jsontext.Encoder
)

var (
	BeginObject = jsontext.BeginObject
	EndObject   = jsontext.EndObject
	Null        = jsontext.Null
	BeginArray  = jsontext.BeginArray
	EndArray    = jsontext.EndArray
)
