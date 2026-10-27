// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCommonProgramCounterBoundary37v(t *testing.T) {
	cases := []struct {
		name string
		edit func(*commonProgramRequest)
	}{
		{"old-schema", func(r *commonProgramRequest) {
			r.SchemaHash = "sha256:a8f649d600a0f57be48c60415e856b4849898f96f07ecdf443926c555d0642cf"
		}},
		{"wrong-capability", func(r *commonProgramRequest) { r.Input.HostImports[0].CapabilityID = 32 }},
		{"wrong-configured-name", func(r *commonProgramRequest) { r.Input.HostImports[0].Name = "other" }},
		{"wrong-configured-parameter", func(r *commonProgramRequest) { r.Input.HostImports[0].ParameterTypes[0] = "u32" }},
		{"wrong-configured-return", func(r *commonProgramRequest) { r.Input.HostImports[0].ReturnType = "u32" }},
		{"non-declaration-path", func(r *commonProgramRequest) { r.Input.HostImports[0].SourcePath = "/src/main.ts" }},
		{"duplicate-host", func(r *commonProgramRequest) {
			r.Input.HostImports = append(r.Input.HostImports, r.Input.HostImports[0])
		}},
		{"unconfigured-declaration", func(r *commonProgramRequest) { r.Input.HostImports = []commonProgramHostImport{} }},
		{"wrong-original-parameter", func(r *commonProgramRequest) { r.Input.Files[0].Content = "declare function record(value:u32):i32;\n" }},
		{"wrong-original-return", func(r *commonProgramRequest) { r.Input.Files[0].Content = "declare function record(value:i32):u32;\n" }},
		{"overloaded-declaration", func(r *commonProgramRequest) { r.Input.Files[0].Content += r.Input.Files[0].Content }},
		{"optional-parameter", func(r *commonProgramRequest) { r.Input.Files[0].Content = "declare function record(value?:i32):i32;\n" }},
		{"missing-annotation", func(r *commonProgramRequest) { r.Input.Files[0].Content = "declare function record(value:i32);\n" }},
		{"definition-not-declaration", func(r *commonProgramRequest) {
			r.Input.Files[0].Content = "declare function record(value:i32):i32 {return value;}\n"
		}},
		{"extra-ambient-declaration", func(r *commonProgramRequest) { r.Input.Files[0].Content += "declare function other(value:i32):i32;\n" }},
		{"host-function-value", func(r *commonProgramRequest) {
			r.Input.Files[1].Content = strings.Replace(r.Input.Files[1].Content, "return record(value);", "const f:(x:i32)=>i32=record; return f(value);", 1)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r commonProgramRequest
			if err := json.Unmarshal(counterHostRequest37v(t), &r); err != nil {
				t.Fatal(err)
			}
			c.edit(&r)
			for i := range r.Input.Files {
				r.Input.Files[i].SHA256 = arrayFactsSHA([]byte(r.Input.Files[i].Content))
			}
			data, err := arrayFactsCanonical(r)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(append(data, '\n')), &out); err == nil || out.Len() != 0 {
				t.Fatal("invalid original Host input must reject without a partial Program")
			}
		})
	}
}
func counterHostMutations37v(t *testing.T, programData []byte, check func(commonProgramPayload, []byte) error) {
	t.Helper()
	cases := []struct {
		name string
		edit func(*commonProgramPayload)
	}{
		{"host-ID", func(p *commonProgramPayload) { p.HostImports[0].ID = "wrong" }},
		{"host-capability", func(p *commonProgramPayload) { p.HostImports[0].CapabilityID = 32 }},
		{"host-original-parameter", func(p *commonProgramPayload) { p.HostImports[0].ParameterTypes[0] = "u32" }},
		{"host-name-range", func(p *commonProgramPayload) { p.HostImports[0].NameRange.Start++ }},
		{"host-declaration-range", func(p *commonProgramPayload) { p.HostImports[0].DeclarationRange.Start++ }},
		{"selected-signature", func(p *commonProgramPayload) { p.HostImports[0].Signature.DeclarationRange.Start++ }},
		{"call-callee", func(p *commonProgramPayload) { p.Functions[0].Body[0].Value.Callee = "wrong" }},
		{"call-original-range", func(p *commonProgramPayload) { p.Functions[0].Body[0].Value.Range.Start++ }},
		{"call-operand-kind", func(p *commonProgramPayload) { (*p.Functions[0].Body[0].Value.Arguments)[0].Kind = "undefinedLiteral" }},
		{"call-operand-order", func(p *commonProgramPayload) { (*p.Functions[0].Body[0].Value.Arguments)[0].Range.Start++ }},
		{"fabricated-owned-host", func(p *commonProgramPayload) { p.Functions = append(p.Functions, arrayGenericFunction{Name: "record"}) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var packet commonProgramPackage
			if err := json.Unmarshal(programData, &packet); err != nil {
				t.Fatal(err)
			}
			c.edit(&packet.Payload)
			raw, err := json.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			if err = check(packet.Payload, raw); err == nil {
				t.Fatal("independent original Program oracle accepted altered facts")
			}
		})
	}
}
