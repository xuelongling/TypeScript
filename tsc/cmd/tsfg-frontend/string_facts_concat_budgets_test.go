// SPDX-License-Identifier: Apache-2.0
// Public Source boundaries and helper-only policies deliberately carry different credit.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func stringConcatRepeatedUnits24e(unit int32, count int) []int32 {
	result := make([]int32, count)
	for i := range result {
		result[i] = unit
	}
	return result
}
func stringConcatMany24e(count int) (string, []stringConcatExpectedCase24e) {
	var source strings.Builder
	cases := []stringConcatExpectedCase24e{}
	for i := 0; i < count; i++ {
		text, tc := stringConcatSimple24e(fmt.Sprintf("f%02d", i), "\"\"", "\"\"", []int32{}, []int32{})
		cases = append(cases, stringConcatShift24e(tc, stringConcatSourceUnits24e(source.String())))
		source.WriteString(text)
	}
	return source.String(), cases
}

func TestStringFactsV2Budgets(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		leftCount, rightCount int
		accepted              bool
	}{
		{"right16384", 0, 16384, true}, {"left16384", 16384, 0, true}, {"right16385", 0, 16385, false}, {"left16385", 16385, 0, false}, {"both16384Concat32768", 16384, 16384, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, one := stringConcatSimple24e("bounded", "\""+strings.Repeat("x", tc.leftCount)+"\"", "\""+strings.Repeat("y", tc.rightCount)+"\"", stringConcatRepeatedUnits24e(120, tc.leftCount), stringConcatRepeatedUnits24e(121, tc.rightCount))
			if len(source) >= 65536 {
				t.Fatal("literal isolation lost Source budget")
			}
			input := stringConcatRequest24e(t, source)
			cases := []stringConcatExpectedCase24e{one}
			stringConcatOfficial24e(t, input, cases)
			if !tc.accepted {
				f := stringConcatFailure24e(t, input, "limit")
				if f.Detail != "value UTF16 code units" {
					t.Fatalf("literal limit was masked: %+v", f)
				}
				return
			}
			expected := stringConcatExpected24e(t, input, cases)
			stringConcatSuccess24e(t, input, expected)
		})
	}
	t.Run("supplementaryWidthTwo", func(t *testing.T) {
		units := []int32{}
		for i := 0; i < 8192; i++ {
			units = append(units, 55296, 56320)
		}
		for _, extra := range []bool{false, true} {
			t.Run(fmt.Sprint(extra), func(t *testing.T) {
				value := strings.Repeat("\U00010000", 8192)
				expectedUnits := append([]int32{}, units...)
				if extra {
					value += "x"
					expectedUnits = append(expectedUnits, 120)
				}
				source, one := stringConcatSimple24e("wide", "\"\"", "\""+value+"\"", []int32{}, expectedUnits)
				input := stringConcatRequest24e(t, source)
				cases := []stringConcatExpectedCase24e{one}
				if len(source) >= 65536 {
					t.Fatal("wide value isolation lost Source budget")
				}
				stringConcatOfficial24e(t, input, cases)
				if extra {
					stringConcatFailure24e(t, input, "limit")
					return
				}
				expected := stringConcatExpected24e(t, input, cases)
				stringConcatSuccess24e(t, input, expected)
			})
		}
	})
	t.Run("source65536Then65537", func(t *testing.T) {
		base, one := stringConcatSimple24e("sourceCap", "\"\"", "\"\"", []int32{}, []int32{})
		for _, size := range []int{65536, 65537} {
			t.Run(fmt.Sprint(size), func(t *testing.T) {
				prefix := "/*" + strings.Repeat("p", size-len(base)-4) + "*/"
				source := prefix + base
				if len(source) != size {
					t.Fatal("manual Source byte premise")
				}
				input := stringConcatRequest24e(t, source)
				if size == 65537 {
					f := stringConcatFailure24e(t, input, "limit")
					if f.Detail != "source UTF8 bytes" {
						t.Fatalf("earlier Source limit expected: %+v", f)
					}
					return
				}
				cases := []stringConcatExpectedCase24e{stringConcatShift24e(one, len(prefix))}
				stringConcatOfficial24e(t, input, cases)
				expected := stringConcatExpected24e(t, input, cases)
				stringConcatSuccess24e(t, input, expected)
			})
		}
	})
	t.Run("functions64Then65", func(t *testing.T) {
		for _, count := range []int{64, 65} {
			t.Run(fmt.Sprint(count), func(t *testing.T) {
				source, cases := stringConcatMany24e(count)
				if len(source) > 65536 {
					t.Fatal("small function corpus exceeded Source cap")
				}
				input := stringConcatRequest24e(t, source)
				stringConcatOfficial24e(t, input, cases)
				if count == 65 {
					stringConcatFailure24e(t, input, "limit")
					return
				}
				expected := stringConcatExpected24e(t, input, cases)
				stringConcatSuccess24e(t, input, expected)
			})
		}
	})
	t.Run("inputByteGuard", func(t *testing.T) {
		r := &stringConcatReadProbe24e{data: bytes.Repeat([]byte{' '}, 2097153)}
		w := &stringConcatWriteProbe24e{}
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v2"}, r, w)
		var f *stringFactsFailure
		if !errors.As(err, &f) || f.Kind != "limit" || w.calls != 0 || r.delivered != 2097153 {
			t.Fatalf("bounded max+1 raw input guard: %v delivered=%d writes=%d", err, r.delivered, w.calls)
		}
	})
	t.Run("publicOutputGuardWithLongVirtualOwnerPaths", func(t *testing.T) {
		source, cases := stringConcatMany24e(64)
		root := "/" + strings.Repeat("p", 40000)
		main, dts := root+"/corpus.ts", root+"/minimal-scalars-v2.d.ts"
		input := stringConcatMutate24e(t, stringConcatRequest24e(t, source), func(r map[string]any) {
			i := r["input"].(map[string]any)
			i["sourceRoot"], i["rootFile"] = root, main
			i["declarations"].(map[string]any)["paths"] = []string{dts}
			f := i["files"].([]any)
			f[0].(map[string]any)["path"], f[1].(map[string]any)["path"] = main, dts
		})
		if len(input) >= 2097152 || len(main) >= 1048576 || 64*len(main) <= 2097152 || len(source) >= 65536 {
			t.Fatal("independent output-only budget premises")
		}
		// Paths exist only in the in-memory official VFS. This creates no host path.
		stringConcatOfficial24e(t, input, cases)
		stringConcatFailure24e(t, input, "limit")
	})
	t.Run("aggregate65536Then65537HelperOnly", func(t *testing.T) {
		// Four repeated equal owners count four times. Public Source cannot isolate
		// this endpoint: required syntax makes Source bytes exceed its cap first.
		value := strings.Repeat("x", 16384)
		values := []string{value, value, value, value}
		if total, err := stringFactsTotalUnits(values); err != nil || total != 65536 {
			t.Fatalf("actual aggregate helper inclusive limit: %d %v", total, err)
		}
		if _, err := stringFactsTotalUnits(append(values, "y")); err == nil {
			t.Fatal("actual aggregate helper accepted65537")
		}
	})
	t.Run("policyBoundariesHelperOnly", func(t *testing.T) {
		policy := stringFactsPolicy(false)
		for _, depth := range []int{64, 65} {
			data := []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
			err := genericJSONPreflight(data, policy)
			if (err == nil) != (depth == 64) {
				t.Fatalf("real depth helper boundary %d: %v", depth, err)
			}
		}
		for _, tokens := range []int{200000, 200001} {
			n := tokens - 2
			data := []byte("[" + strings.Repeat("0,", n-1) + "0]")
			err := genericJSONPreflight(data, policy)
			if (err == nil) != (tokens == 200000) {
				t.Fatalf("real token helper boundary %d: %v", tokens, err)
			}
		}
		for _, count := range []int{1048576, 1048577} {
			data := []byte("\"" + strings.Repeat("x", count) + "\"")
			err := genericJSONPreflight(data, policy)
			if (err == nil) != (count == 1048576) {
				t.Fatalf("real string-byte helper boundary %d: %v", count, err)
			}
		}
		frame := []byte("[\"" + strings.Repeat("x", 1048572) + "\",\"" + strings.Repeat("y", 1048572) + "\"]\n")
		if len(frame) != 2097152 || genericJSONPreflight(frame, policy) != nil || genericJSONPreflight(append(bytes.Clone(frame), ' '), policy) == nil {
			t.Fatal("real raw preflight inclusive byte guard")
		}
	})
	t.Run("publisherFrameBoundaryHelperOnly", func(t *testing.T) {
		// Natural v1/v2 shared publisher, not a fabricated public DTO/Source input.
		for _, extra := range []int{0, 1} {
			packet := []string{strings.Repeat("x", 1048572), strings.Repeat("y", 1048572+extra)}
			w := &stringConcatWriteProbe24e{}
			err := publishStringFactsPacket(t.Context(), w, packet)
			if extra == 0 {
				if err != nil || w.calls != 1 || w.Len() != 2097152 || w.Bytes()[w.Len()-1] != '\n' {
					t.Fatalf("real publisher exact inclusive frame: %v calls=%d bytes=%d", err, w.calls, w.Len())
				}
			} else {
				var f *stringFactsFailure
				if !errors.As(err, &f) || f.Kind != "limit" || w.calls != 0 {
					t.Fatalf("real publisher +1 frame guard: %v calls=%d", err, w.calls)
				}
			}
		}
	})
}
