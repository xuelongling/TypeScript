// SPDX-License-Identifier: Apache-2.0
// Independent Array public-protocol boundaries. No legacy test group is selected.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// Local probes keep this newly admitted Array boundary independent of old
// String/nullable execution groups and their fixture assumptions.
type arrayBoundaryReader25 struct {
	input  io.Reader
	calls  int
	err    error
	cancel context.CancelFunc
}

func (r *arrayBoundaryReader25) Read(p []byte) (int, error) {
	r.calls++
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.input.Read(p)
	if r.cancel != nil {
		r.cancel()
	}
	return n, err
}

type arrayBoundaryWriter25 struct {
	bytes.Buffer
	calls  int
	err    error
	short  bool
	cancel context.CancelFunc
}

func (w *arrayBoundaryWriter25) Write(p []byte) (int, error) {
	w.calls++
	if w.err != nil {
		return 0, w.err
	}
	if w.short {
		return len(p) - 1, nil
	}
	n, err := w.Buffer.Write(p)
	if w.cancel != nil {
		w.cancel()
	}
	return n, err
}
func arrayBoundaryWire25(t *testing.T, v any) []byte {
	t.Helper()
	return append(arrayFirstCanonical25(t, v), '\n')
}
func arrayBoundaryEnvelope25(t *testing.T) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(arrayFirstFixture25(t)[3].RequestText), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func arrayBoundarySource25(t *testing.T, source string) []byte {
	t.Helper()
	v := arrayBoundaryEnvelope25(t)
	input := v["input"].(map[string]any)
	root := input["rootFile"].(string)
	for _, value := range input["files"].([]any) {
		file := value.(map[string]any)
		if file["path"] == root {
			file["content"] = source
			file["sha256"] = arrayFirstHash25([]byte(source))
		}
	}
	return arrayBoundaryWire25(t, v)
}
func arrayBoundaryFailure25(t *testing.T, err error) *arrayFactsFailure {
	t.Helper()
	var owned *arrayFactsFailure
	if err == nil || !errors.As(err, &owned) {
		t.Fatalf("expected owned public Array failure: %v", err)
	}
	if !strings.HasPrefix(owned.Code, "r03-array-facts-") {
		t.Fatalf("wrong family: %+v", owned)
	}
	return owned
}
func arrayBoundaryCall25(t *testing.T, input []byte) ([]byte, *arrayFactsFailure) {
	t.Helper()
	var output arrayBoundaryWriter25
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), &output)
	if err == nil {
		return append([]byte(nil), output.Bytes()...), nil
	}
	owned := arrayBoundaryFailure25(t, err)
	if output.calls != 0 || output.Len() != 0 {
		t.Fatal("pre-publication failure wrote a partial packet")
	}
	return nil, owned
}

func TestArrayFactsBoundaryWireAndClosure(t *testing.T) {
	base := []byte(arrayFirstFixture25(t)[3].RequestText)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"noLF", bytes.TrimSuffix(base, []byte{'\n'})},
		{"extraLF", append(append([]byte(nil), base...), '\n')},
		{"leadingSpace", append([]byte(" "), base...)},
		{"duplicateKey", bytes.Replace(base, []byte("{"), []byte("{\"family\":\"tsfg.array-facts-request-candidate\","), 1)},
		{"brokenJSON", []byte("{\n")},
		{"byteLimit", bytes.Repeat([]byte(" "), 2097153)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := arrayBoundaryCall25(t, tc.data)
			if err == nil || err.RuleID != "r01-input-boundary" || err.Phase != "input" {
				t.Fatalf("input boundary: %+v", err)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknownTop", func(v map[string]any) { v["extra"] = true }},
		{"version", func(v map[string]any) { v["version"] = 1 }},
		{"retiredVersion5", func(v map[string]any) { v["version"] = 5 }},
		{"foreignFamily", func(v map[string]any) { v["family"] = "foreign" }},
		{"unknownInput", func(v map[string]any) { v["input"].(map[string]any)["extra"] = true }},
		{"profile", func(v map[string]any) {
			v["input"].(map[string]any)["declarations"].(map[string]any)["version"] = "minimal-scalars-v2"
		}},
		{"thirdSource", func(v map[string]any) {
			in := v["input"].(map[string]any)
			in["files"] = append(in["files"].([]any), map[string]any{"path": "/src/extra.ts", "content": "", "sha256": arrayFirstHash25(nil)})
		}},
		{"staleSourceSHA", func(v map[string]any) {
			files := v["input"].(map[string]any)["files"].([]any)
			files[0].(map[string]any)["content"] = "changed"
		}},
		{"forgedPinnedDTS", func(v map[string]any) {
			in := v["input"].(map[string]any)
			for _, x := range in["files"].([]any) {
				f := x.(map[string]any)
				if f["path"] != "/src/main.ts" {
					f["content"] = f["content"].(string) + "// extra\n"
					f["sha256"] = arrayFirstHash25([]byte(f["content"].(string)))
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := arrayBoundaryEnvelope25(t)
			tc.change(v)
			_, err := arrayBoundaryCall25(t, arrayBoundaryWire25(t, v))
			if err == nil || err.RuleID != "r01-input-boundary" || err.Phase != "input" {
				t.Fatalf("closed input: %+v", err)
			}
		})
	}
}

func TestArrayFactsBoundaryExactIndexAndGrammar(t *testing.T) {
	source := func(index string) string {
		return "export function boundary(): f64 { const values: f64[] = [1]; return values[" + index + "]; }"
	}
	for _, tc := range []struct{ name, token, decimal string }{
		{"decimal", "1.0", "1"}, {"exponent", "1e0", "1"}, {"hex", "0x1", "1"}, {"zeroHugeExponent", "0e" + strings.Repeat("9", 1000), "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := arrayBoundaryCall25(t, arrayBoundarySource25(t, source(tc.token)))
			if err != nil {
				t.Fatalf("legal exact index: %+v", err)
			}
			var packet arrayFirstPackage25
			if e := json.Unmarshal(data, &packet); e != nil {
				t.Fatal(e)
			}
			if len(packet.Payload.Functions) != 1 {
				t.Fatal("whole function collection")
			}
			fn := packet.Payload.Functions[0]
			read := fn.Body[len(fn.Body)-1].Value
			if read == nil || read.Kind != "checkedElementAccess" || read.Index == nil || read.Index.Token != tc.token || read.Index.Decimal != tc.decimal {
				t.Fatalf("original exact checked index: %+v", read)
			}
			if !bytes.Equal(data, append(arrayFirstCanonical25(t, json.RawMessage(data)), '\n')) {
				t.Fatal("whole canonical packet")
			}
		})
	}
	for _, tc := range []struct{ name, token string }{
		{"fractionRoundsToInteger", "1.0000000000000001"}, {"hugePositiveExponent", "1e" + strings.Repeat("9", 1000)},
		{"hugeNegativeExponent", "1e-" + strings.Repeat("9", 1000)}, {"maxNotArrayIndex", "4294967295"}, {"negative", "-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := arrayBoundaryCall25(t, arrayBoundarySource25(t, source(tc.token)))
			if err == nil || err.Code != "r03-array-facts-capability" || err.RuleID != "r01-ts-baseline" || err.Phase != "capability" {
				t.Fatalf("exact index capability: %+v", err)
			}
			if len(err.Diagnostics) == 0 {
				t.Fatal("capability lost original source anchor")
			}
		})
	}
	for _, tc := range []struct{ name, initializer string }{
		{"spread", "...[1]"}, {"as", "as"}, {"nonnull", "nonnull"}, {"holes", "holes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := "export function boundary(): f64 { const values: f64[] = "
			switch tc.name {
			case "spread":
				s += "[...[1]]"
			case "as":
				s += "[1] as f64[]"
			case "nonnull":
				s += "[1]!"
			case "holes":
				s += "[1,,2]"
			}
			s += "; return values[0]; }"
			_, err := arrayBoundaryCall25(t, arrayBoundarySource25(t, s))
			if err == nil || len(err.Diagnostics) == 0 {
				t.Fatal("excluded syntax produced a packet or lost diagnostics")
			}
			if tc.name == "holes" {
				found := false
				for _, d := range err.Diagnostics {
					found = found || d.UpstreamCode != 0
				}
				if !found {
					t.Fatal("official hole type diagnostic was replaced")
				}
			} else if err.Code != "r03-array-facts-capability" {
				t.Fatalf("private grammar rejection: %+v", err)
			}
		})
	}
	t.Run("officialTypePriority", func(t *testing.T) {
		_, err := arrayBoundaryCall25(t, arrayBoundarySource25(t, "export function boundary(): f64 { const values: f64[] = ['bad']; return values[0]; }"))
		found := false
		if err != nil {
			for _, d := range err.Diagnostics {
				found = found || d.UpstreamCode == 2322
			}
		}
		if !found {
			t.Fatalf("missing original2322: %+v", err)
		}
	})
	t.Run("sourceByteLimit", func(t *testing.T) {
		_, err := arrayBoundaryCall25(t, arrayBoundarySource25(t, strings.Repeat(" ", 65537)))
		if err == nil || err.Code != "r03-array-facts-limit" {
			t.Fatalf("source budget: %+v", err)
		}
	})
}

func TestArrayFactsBoundaryIOCancellationAndSelection(t *testing.T) {
	input := []byte(arrayFirstFixture25(t)[3].RequestText)
	sentinel := errors.New("independent Array IO sentinel")
	t.Run("preCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r, w := &arrayBoundaryReader25{input: bytes.NewReader(input)}, &arrayBoundaryWriter25{}
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v6"}, r, w)
		if !errors.Is(err, context.Canceled) || r.calls != 0 || w.calls != 0 {
			t.Fatalf("pre-cancel: %v", err)
		}
	})
	t.Run("cancelDuringRead", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r, w := &arrayBoundaryReader25{input: bytes.NewReader(input), cancel: cancel}, &arrayBoundaryWriter25{}
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v6"}, r, w)
		if !errors.Is(err, context.Canceled) || w.calls != 0 {
			t.Fatalf("read cancellation: %v", err)
		}
	})
	t.Run("readFailure", func(t *testing.T) {
		w := &arrayBoundaryWriter25{}
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, &arrayBoundaryReader25{err: sentinel}, w)
		if !errors.Is(err, sentinel) || w.calls != 0 {
			t.Fatal("reader error identity/zero write")
		}
	})
	t.Run("writeFailure", func(t *testing.T) {
		w := &arrayBoundaryWriter25{err: sentinel}
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), w)
		if !errors.Is(err, sentinel) || w.calls != 1 {
			t.Fatal("writer error identity/single attempt")
		}
	})
	t.Run("shortWrite", func(t *testing.T) {
		w := &arrayBoundaryWriter25{short: true}
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), w)
		if !errors.Is(err, io.ErrShortWrite) || w.calls != 1 {
			t.Fatal("short write retried or hidden")
		}
	})
	t.Run("postWriteCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		w := &arrayBoundaryWriter25{cancel: cancel}
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v6"}, bytes.NewReader(input), w)
		if !errors.Is(err, context.Canceled) || w.calls != 1 || w.Len() == 0 {
			t.Fatal("completed sink write must remain recorded")
		}
	})
	for _, args := range [][]string{{"--array-facts-io-v6", "extra"}, {"--array-facts-io-v1"}, {"--array-facts-io-v2"}, {"--array-facts-io-v3"}, {"--array-facts-io-v4"}, {"--array-facts-io-v5"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			r, w := &arrayBoundaryReader25{input: bytes.NewReader(input)}, &arrayBoundaryWriter25{}
			if err := runFrontendCommand(t.Context(), args, r, w); err == nil || r.calls != 0 || w.calls != 0 {
				t.Fatal("invalid argv touched IO")
			}
		})
	}
}
