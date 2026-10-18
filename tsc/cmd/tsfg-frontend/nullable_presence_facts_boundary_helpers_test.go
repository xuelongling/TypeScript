// SPDX-License-Identifier: Apache-2.0
// Boundary expectations use independent frozen values and standard-library JSON only.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

//go:embed testdata/nullable-presence-v4-boundary/*.json
var nullablePresenceBoundaryData embed.FS

const nullablePresenceBoundaryS4 = "sha256:4205bc18e817d983dd0f91d5b1c1d92b99cff5f6e32eca419d22530bfdcf41ac"
const nullablePresenceBoundaryS3 = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"

type nullablePresenceBoundaryWire struct{ Request, Packet string }
type nullablePresenceBoundaryPositive struct {
	Key, Source, Legacy string
	Wires               map[string]nullablePresenceBoundaryWire
}

type nullablePresenceBoundaryReader struct {
	reader           io.Reader
	calls, delivered int
	after            func()
}

func (r *nullablePresenceBoundaryReader) Read(p []byte) (int, error) {
	r.calls++
	n, err := r.reader.Read(p)
	r.delivered += n
	if r.after != nil {
		r.after()
	}
	return n, err
}

type nullablePresenceBoundaryWriter struct {
	writes   int
	attempts [][]byte
	retained []byte
	hook     func([]byte) (int, error)
}

func (w *nullablePresenceBoundaryWriter) Write(p []byte) (int, error) {
	w.writes++
	w.retained = p
	w.attempts = append(w.attempts, bytes.Clone(p))
	if w.hook != nil {
		return w.hook(p)
	}
	return len(p), nil
}

func nullablePresenceBoundaryRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := nullablePresenceBoundaryData.ReadFile("testdata/nullable-presence-v4-boundary/" + name)
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := nullablePresenceBoundaryPins[name]
	if !ok || len(data) != pin.bytes || nullablePresenceFirstDigest(data) != pin.sha {
		t.Fatalf("independent boundary fixture changed: %s", name)
	}
	return data
}
func nullablePresenceBoundaryPositives(t *testing.T) []nullablePresenceBoundaryPositive {
	t.Helper()
	var rows []nullablePresenceBoundaryPositive
	if err := json.Unmarshal(nullablePresenceBoundaryRead(t, "positive.json"), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatal("complete independent positive corpus changed")
	}
	return rows
}
func nullablePresenceBoundaryObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// These test graphs have ASCII keys, safe integer fields and no U+2028/U+2029.
// Encoding sorted map keys with HTML escaping disabled is their independent JCS recipe.
func nullablePresenceBoundaryJSON(t *testing.T, value any) []byte {
	t.Helper()
	var out bytes.Buffer
	e := json.NewEncoder(&out)
	e.SetEscapeHTML(false)
	if err := e.Encode(value); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n"))
}
func nullablePresenceBoundaryDigest(data []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}
func nullablePresenceBoundaryRequest(t *testing.T, source string, version int) []byte {
	t.Helper()
	v := nullablePresenceBoundaryObject(t, nullablePresenceFirstRead(t, "mask2-null.request.json"))
	v["version"] = version
	if version == 3 {
		v["schemaHash"] = nullablePresenceBoundaryS3
	}
	input := v["input"].(map[string]any)
	input["files"].([]any)[0].(map[string]any)["content"] = source
	return nullablePresenceBoundaryJSON(t, v)
}
func nullablePresenceBoundaryInput(t *testing.T, raw []byte) request {
	t.Helper()
	v := nullablePresenceBoundaryObject(t, raw)
	var r request
	if err := json.Unmarshal(nullablePresenceBoundaryJSON(t, v["input"]), &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func nullablePresenceBoundaryCommand(ctx context.Context, args []string, raw []byte) (*nullablePresenceBoundaryReader, *nullablePresenceBoundaryWriter, error) {
	r := &nullablePresenceBoundaryReader{reader: bytes.NewReader(raw)}
	w := &nullablePresenceBoundaryWriter{}
	err := runFrontendCommand(ctx, args, r, w)
	return r, w, err
}
func nullablePresenceBoundarySuccess(t *testing.T, flag string, raw, expected []byte) {
	t.Helper()
	before := bytes.Clone(raw)
	r, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{flag}, raw)
	if err != nil {
		t.Fatalf("public success rejected: %v", err)
	}
	if r.delivered != len(raw) || w.writes != 1 || len(w.attempts) != 1 || !bytes.Equal(w.attempts[0], expected) {
		t.Fatalf("complete public packet differs: reads=%d bytes=%d writes=%d actual=%s expected=%s", r.calls, r.delivered, w.writes, bytes.Join(w.attempts, nil), expected)
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("public command mutated caller input bytes")
	}
	if bytes.HasSuffix(w.attempts[0], []byte("\n")) {
		t.Fatal("private packet acquired a final LF")
	}
}
func nullablePresenceBoundaryError(t *testing.T, err error, kind string, where *span, cause int) *nullablePresenceFactsError {
	t.Helper()
	var got *nullablePresenceFactsError
	if err == nil || !errors.As(err, &got) || got.Kind != kind || got.Detail == "" {
		t.Fatalf("structured v4 error: got=%#v want kind=%s", err, kind)
	}
	want := fmt.Sprintf("[r03-nullable-facts-%s] %s", got.Kind, got.Detail)
	if where == nil {
		if got.File != "" || got.Range != (span{}) {
			t.Fatalf("global error invented owner: %#v", got)
		}
	} else {
		if got.File != "/src/main.ts" || got.Range != *where {
			t.Fatalf("actual Source anchor differs: got=%s:%+v want=%+v", got.File, got.Range, *where)
		}
		want += fmt.Sprintf(" at %s:%d:%d", got.File, got.Range.Start, got.Range.End)
	}
	if err.Error() != want || errors.Unwrap(got) != got.Cause {
		t.Fatal("structured error format/cause chain changed")
	}
	if cause == 0 && got.Cause != nil || cause == 1 && got.Cause == nil {
		t.Fatalf("cause contract differs: %#v", got)
	}
	if strings.Contains(got.Detail, "CALLER_SECRET_20K") || strings.Contains(got.Detail, "function choose(") || strings.Contains(got.Detail, "function relay(") {
		t.Fatal("Detail exposed caller or Source text")
	}
	return got
}
func nullablePresenceBoundaryReject(t *testing.T, raw []byte, kind string, where *span, cause int) *nullablePresenceFactsError {
	t.Helper()
	before := bytes.Clone(raw)
	_, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v4"}, raw)
	if w.writes != 0 || !bytes.Equal(raw, before) {
		t.Fatal("rejection published partial success or mutated input")
	}
	return nullablePresenceBoundaryError(t, err, kind, where, cause)
}
func nullablePresenceBoundaryLegacy(t *testing.T, raw []byte, expected []byte) response {
	t.Helper()
	r := nullablePresenceBoundaryInput(t, raw)
	var out bytes.Buffer
	if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(nullablePresenceBoundaryJSON(t, r)), &out); err != nil {
		t.Fatal(err)
	}
	var facts response
	if err := json.Unmarshal(out.Bytes(), &facts); err != nil {
		t.Fatal(err)
	}
	if expected != nil {
		nullablePresenceFirstEqualJSON(t, out.Bytes(), expected)
	}
	return facts
}
func nullablePresenceBoundaryOfficialAndLegacy(t *testing.T, raw []byte) response {
	t.Helper()
	r := nullablePresenceBoundaryInput(t, raw)
	nullablePresenceFirstDiagnostics(t, nullablePresenceFirstProgram(t, r))
	facts := nullablePresenceBoundaryLegacy(t, raw, nil)
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
		t.Fatalf("legacy prerequisite rejected new short control: %+v", facts)
	}
	return facts
}
