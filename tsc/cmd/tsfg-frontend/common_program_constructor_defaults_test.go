// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func constructorDefaultRequest23aa(t *testing.T, source string, applications []string, version int) []byte {
	t.Helper()
	var request commonProgramRequest
	if err := json.Unmarshal(genericArrayRequest25f(t, source), &request); err != nil {
		t.Fatal(err)
	}
	schema := commonProgramConstructorSchema
	if version == 2 {
		schema = commonProgramSchema
	}
	request.Family, request.Version, request.SchemaHash = "tsfg.program-facts-request-candidate", version, "sha256:"+arrayFactsSHA(schema)
	request.Input.HostImports = []commonProgramHostImport{}
	request.Input.Target = &commonProgramTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", PointerBits: "64", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
	roots := []closureRoot{}
	for _, name := range applications {
		roots = append(roots, closureRoot{Kind: "application", SourcePath: "/src/main.ts", Name: name})
	}
	slices.SortFunc(roots, func(a, b closureRoot) int { return strings.Compare(a.Name, b.Name) })
	request.Input.CompilationClosure = &closureConfiguration{Roots: append(slices.Clone(roots), closureRoot{Kind: "module-initialization", SourcePath: "/src/main.ts", Name: "$moduleInit"}), RequiredRoots: roots}
	data, err := arrayFactsCanonical(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func constructorDefaultCapture23aa(t *testing.T, name string, data []byte) {
	t.Helper()
	if root := os.Getenv("TSFG_CONSTRUCTOR_DEFAULT_CAPTURE_ROOT"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name)
		if old, err := os.ReadFile(path); err == nil {
			if !bytes.Equal(old, data) {
				t.Fatal("retained original capture differs", path)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		} else if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCommonProgramConstructorDefaults23aa(t *testing.T) {
	source, err := os.ReadFile("testdata/common-program-v3/constructor-defaults.ts")
	if err != nil {
		t.Fatal(err)
	}
	input := constructorDefaultRequest23aa(t, string(source), []string{"actualUndefined", "argumentsFirst", "defaultStillUndefined", "derived", "inherited", "nullActual", "omitted", "thisDefault", "throwsAfterEffects"}, 3)
	constructorDefaultCapture23aa(t, "constructor-defaults-request.json", input)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v3"}, bytes.NewReader(input), &output); err != nil {
		if output.Len() != 0 {
			t.Fatal("failed producer published partial facts")
		}
		var failure *arrayFactsFailure
		if errors.As(err, &failure) {
			data, encodeErr := json.Marshal(failure)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			constructorDefaultCapture23aa(t, "constructor-defaults-failure.json", append(data, '\n'))
		}
		t.Fatal(err)
	}
	constructorDefaultCapture23aa(t, "constructor-defaults-package.json", output.Bytes())
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	p := packet.Payload
	if packet.Version != 3 || packet.SchemaHash != "sha256:"+arrayFactsSHA(commonProgramConstructorSchema) || packet.RequestID != "sha256:"+arrayFactsSHA(append([]byte("tsfg.program-facts.request.v3\x00"), input...)) || len(p.Diagnostics) != 0 || len(p.Files) != 2 || len(p.Globals) != 1 {
		t.Fatal("exact new protocol identity or complete original Program differs")
	}
	var raw struct{ Payload json.RawMessage }
	if err := json.Unmarshal(output.Bytes(), &raw); err != nil || packet.PackageID != "sha256:"+arrayFactsSHA(append([]byte("tsfg.program-facts.package.v3\x00"+packet.SchemaHash+"\x00"), raw.Payload...)) {
		t.Fatal("new package domain differs", err)
	}
	types, functions := map[string]arrayGenericType{}, map[string]arrayGenericFunction{}
	for _, typ := range p.Types {
		types[typ.ID] = typ
		if typ.Name != "" {
			types[typ.Name] = typ
		}
	}
	for _, fn := range p.Functions {
		functions[fn.Name] = fn
	}
	ctor := types["Numeric"].Constructor
	if ctor == nil || ctor.ParameterInitialization == nil || ctor.ParameterInitialization.MinimumArgumentCount != 0 || !ctor.ParameterInitialization.HasParameterExpressions || len(ctor.ParameterInitialization.Steps) != 2 {
		t.Fatal("two actual ordered default parameter steps required")
	}
	for i, step := range ctor.ParameterInitialization.Steps {
		if step.Binding != ctor.Parameters[i].ID || step.Range != ctor.Parameters[i].DeclarationRange || ctor.Parameters[i].DeclaredType != "i32" || step.ArgumentType != "union<i32,undefined>" || !slices.Equal(types[step.ArgumentType].Members, []string{"i32", "undefined"}) || step.Initializer == nil {
			t.Fatal("incoming undefined and initialized binding identity/range must stay distinct")
		}
	}
	steps := ctor.ParameterInitialization.Steps
	if steps[0].Initializer.Kind != "call" || steps[1].Initializer.Kind != "binary" || steps[1].Initializer.Left.Binding != ctor.Parameters[0].ID || steps[1].Initializer.Right.Kind != "call" {
		t.Fatal("single original calls and earlier parameter reference were lost")
	}
	units := utf16.Encode([]rune(string(source)))
	for _, entry := range []struct {
		rangeValue span
		text       string
	}{{steps[0].Range, "public first: i32 = mark(2)"}, {steps[1].Range, "public second: i32 = first + mark(3)"}, {steps[1].Initializer.Range, "first + mark(3)"}} {
		if string(utf16.Decode(units[entry.rangeValue.Start:entry.rangeValue.End])) != entry.text {
			t.Fatal("original UTF16 source anchor changed", entry.text)
		}
	}
	for _, name := range []string{"Numeric", "Derived", "Broken", "Inherited"} {
		construction := types[name].Construction
		if construction == nil || construction.Signature.MinimumArgumentCount == nil || *construction.Signature.MinimumArgumentCount != 0 {
			t.Fatal("original optional construct arity lost", name)
		}
	}
	omitted := functions["omitted"].Body[1].Initializer
	actual := functions["actualUndefined"].Body[1].Initializer
	nullActual := functions["nullActual"].Body[0].Initializer
	if omitted.Arguments == nil || len(*omitted.Arguments) != 0 || actual.Arguments == nil || len(*actual.Arguments) != 2 || (*actual.Arguments)[0].Kind != "undefinedLiteral" || (*actual.Arguments)[0].Type != "undefined" || nullActual.Arguments == nil || (*nullActual.Arguments)[0].Kind != "nullLiteral" || (*nullActual.Arguments)[0].Type != "null" {
		t.Fatal("actual argument count/undefined/null was invented or retyped")
	}
	derived := types["Derived"].Constructor
	if derived.ParameterInitialization.Steps[0].Initializer.Kind != "call" || derived.Body[0].Value.Kind != "superConstruct" || len(*derived.Body[0].Value.Arguments) != 1 || derived.Body[0].Value.Signature.MinimumArgumentCount == nil || *derived.Body[0].Value.Signature.MinimumArgumentCount != 0 || (*derived.Body[0].Value.Arguments)[0].Binding != derived.Parameters[0].ID {
		t.Fatal("derived default and original partially omitted super call changed")
	}
	broken := types["Broken"].Constructor.ParameterInitialization.Steps
	if functions["throwsAfterEffects"].Body[1].Kind != "try" || len(broken) != 3 || broken[0].Initializer.Callee != functions["mark"].ID || broken[1].Initializer.Callee != functions["fail"].ID || broken[2].Initializer.Callee != functions["mark"].ID {
		t.Fatal("ordinary exception region or throwing default call dropped")
	}
	thisDefault := types["ThisDefault"].Constructor.ParameterInitialization.Steps[0].Initializer
	if thisDefault.Kind != "binary" || thisDefault.Left.Kind != "fieldAccess" || thisDefault.Left.Receiver.Kind != "thisReference" || thisDefault.Left.Receiver.Type != types["ThisDefault"].ID || thisDefault.Right.Callee != functions["mark"].ID {
		t.Fatal("officially accepted base this default must retain its original initialized receiver/field")
	}
	stillUndefined := types["StillUndefined"].Constructor.ParameterInitialization.Steps[0]
	if stillUndefined.ArgumentType != "union<i32,undefined>" || stillUndefined.Initializer.Kind != "as" || stillUndefined.Initializer.Type != "i32" || stillUndefined.Initializer.Expression.Kind != "as" || stillUndefined.Initializer.Expression.Type != "unknown" || stillUndefined.Initializer.Expression.Expression.Kind != "undefinedLiteral" || stillUndefined.Initializer.Expression.Expression.Type != "undefined" {
		t.Fatal("a default assertion must not erase its real undefined operand or invent an i32 payload")
	}
}

func TestCommonProgramConstructorDefaultBoundaries23aa(t *testing.T) {
	for _, sample := range []struct{ name, source, kind string }{
		{"optional", "class C{constructor(n?:i32){}} export function f():C{return new C();}", "capability"},
		{"rest", "class C{constructor(...n:i32[]){}} export function f():C{return new C();}", "capability"},
		{"pattern", "class C{constructor([n]:i32[]=[1]){}} export function f():C{return new C();}", "capability"},
		{"ordinary-default", "export function f(n:i32=1):i32{return n;}", "capability"},
		{"inferred", "class C{constructor(n=1){}} export function f():C{return new C();}", "capability"},
		{"arrow-default", "class C{constructor(n:()=>i32=()=>1){}} export function f():C{return new C();}", "capability"},
		{"self", "class C{constructor(n:i32=n){}} export function f():C{return new C();}", "diagnostic"},
		{"later", "class C{constructor(n:i32=m,m:i32=1){}} export function f():C{return new C();}", "diagnostic"},
		{"body-var", "class C{constructor(n:i32=v){var v:i32=1;}} export function f():C{return new C();}", "diagnostic"},
		{"body-function", "class C{constructor(n:i32=g()){function g():i32{return 1;}}} export function f():C{return new C();}", "diagnostic"},
		{"super-default", "class B{} class C extends B{constructor(n:void=super()){}} export function f():C{return new C();}", "diagnostic"},
		{"super-default-body", "class B{} class C extends B{constructor(n:void=super()){super();}} export function f():C{return new C();}", "diagnostic"},
		{"arguments-read", "class C{constructor(n:unknown=arguments){}} export function f():C{return new C();}", "capability"},
		{"syntax", "class C{constructor(n:i32=){}} export function f():C{return new C();}", "diagnostic"},
		{"type", "class C{constructor(n:i32=true){}} export function f():C{return new C();}", "diagnostic"},
		{"actual-null-type", "class C{constructor(n:i32=1){}} export function f():C{return new C(null);}", "diagnostic"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v3"}, bytes.NewReader(constructorDefaultRequest23aa(t, sample.source, []string{"f"}, 3)), &output)
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || output.Len() != 0 || failure.Kind != sample.kind || len(failure.Diagnostics) == 0 {
				t.Fatalf("wrong original boundary: %v %#v", err, failure)
			}
			if sample.kind == "capability" && (failure.Diagnostics[0].Phase != "capability" || failure.Diagnostics[0].Category != "unsupported") || sample.kind == "diagnostic" && failure.Diagnostics[0].UpstreamCode == 0 {
				t.Fatal("legal unsupported input became syntax error, or official diagnostic was replaced")
			}
			codes := []int32{}
			for _, diagnostic := range failure.Diagnostics {
				codes = append(codes, diagnostic.UpstreamCode)
			}
			t.Logf("%s: kind=%s phase=%s upstream=%v", sample.name, failure.Kind, failure.Diagnostics[0].Phase, codes)
		})
	}
}

func TestCommonProgramConstructorDefaultArity23aa(t *testing.T) {
	source := "class C{constructor(public first:i32=1,public second:i32){}} export function f():C{return new C(undefined,2);}"
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v3"}, bytes.NewReader(constructorDefaultRequest23aa(t, source, []string{"f"}, 3)), &output); err != nil {
		t.Fatal(err)
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	for _, typ := range packet.Payload.Types {
		if typ.Name != "C" {
			continue
		}
		steps := typ.Constructor.ParameterInitialization
		if steps.MinimumArgumentCount != 2 || typ.Construction.Signature.MinimumArgumentCount == nil || *typ.Construction.Signature.MinimumArgumentCount != 2 || steps.Steps[0].ArgumentType != "union<i32,undefined>" || steps.Steps[1].ArgumentType != "i32" || steps.Steps[1].Initializer != nil {
			t.Fatal("default before a required parameter changed official arity or fabricated a default")
		}
		return
	}
	t.Fatal("actual class missing")
}

func TestCommonProgramConstructorDefaultsDescriptor23aa(t *testing.T) {
	if len(commonProgramConstructorSchema) == 0 || commonProgramConstructorSchema[len(commonProgramConstructorSchema)-1] != '\n' {
		t.Fatal("descriptor requires final LF")
	}
	raw := commonProgramConstructorSchema[:len(commonProgramConstructorSchema)-1]
	canonical := tsjson.Value(bytes.Clone(raw))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, raw) {
		t.Fatal("descriptor raw bytes must match consumer JCS plus LF hash", err)
	}
}

func TestCommonProgramConstructorDefaultLegacy23aa(t *testing.T) {
	source := "class C{constructor(public n:i32=1){}} export function f():C{return new C();}"
	var output bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(constructorDefaultRequest23aa(t, source, []string{"f"}, 2)), &output)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Kind != "capability" || output.Len() != 0 {
		t.Fatal("legacy source capability changed", err)
	}
	plain := "class C{constructor(public n:i32){}} export function f():C{return new C(1);}"
	output.Reset()
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(constructorDefaultRequest23aa(t, plain, []string{"f"}, 2)), &output); err != nil || bytes.Contains(output.Bytes(), []byte("parameterInitialization")) || bytes.Contains(output.Bytes(), []byte("minimumArgumentCount")) {
		t.Fatal("new fields leaked into old protocol", err)
	}
}
