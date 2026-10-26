// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode/utf16"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

// The source/oracle were frozen before this public command ran. These are
// original Source facts, not a dispatch/effects verdict or an execution result.
func TestCommonProgramMethodPublic37q(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/common-program-v1/common-method-dispatch.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(sourceBytes) != 1732 || arrayFactsSHA(sourceBytes) != "b6e5f5f7a8d3e2b414cb8a0504e21f82401bf702aae2abc82a7da98404a45c0c" {
		t.Fatal("frozen complete Source bytes changed")
	}
	packet := commonMethodPacket37q(t, "common-method-dispatch", []string{"concreteDerived", "entryBase", "inheritedEntry", "ordered", "ordinary", "peerEntry"})
	p := packet.Payload
	if packet.Version != 1 || len(p.Diagnostics) != 0 || len(p.Functions) != 17 || len(p.Modules) != 1 || len(p.Globals) != 2 || len(p.Closure.Roots) != 9 || len(p.EntryCompatibilityRoots) != 8 {
		t.Fatal("whole Program function/module/global/root/entry inventory changed")
	}
	if p.Files[0].SHA256 != arrayFactsSHA(sourceBytes) || p.Files[0].UTF16Length != 1730 {
		t.Fatal("original Source UTF16 identity changed")
	}
	types, typeNames := map[string]arrayGenericType{}, map[string]string{}
	fields, members := map[string]string{}, map[string]string{}
	for _, typ := range p.Types {
		if typ.Kind == "reference" || typ.Kind == "value" {
			types[typ.Name], typeNames[typ.ID] = typ, typ.Name
			for _, field := range *typ.Fields {
				fields[field.ID], members[field.ID] = typ.Name+"."+field.Name, typ.Name+"."+field.Name
			}
		}
	}
	if len(types) != 7 || len(fields) != 4 {
		t.Fatal("seven actual classes/four own fields required")
	}
	functions, free := map[string]arrayGenericFunction{}, []string{}
	for _, fn := range p.Functions {
		name := fn.Name
		if fn.OwnerType != "" {
			name = typeNames[fn.OwnerType] + "." + fn.Name
		} else {
			free = append(free, fn.Name)
		}
		functions[name], members[fn.ID] = fn, name
	}
	if !slices.Equal(free, []string{"add", "left", "right", "ordinary", "callBase", "toBase", "entryBase", "concreteDerived", "inheritedEntry", "peerEntry", "makeReceiver", "makeArgument", "ordered"}) {
		t.Fatal("complete original free-function declaration order changed")
	}
	for _, name := range []string{"Base", "Middle", "Derived", "Peer"} {
		if !slices.Equal(types[name].Methods, []string{functions[name+".m"].ID}) {
			t.Fatal("original own method directory changed", name)
		}
	}
	if len(types["Inherited"].Methods) != 0 || types["Inherited"].Base != types["Middle"].ID || types["Derived"].Base != types["Middle"].ID || types["Middle"].Base != types["Base"].ID || functions["Derived.m"].OverrideTarget != functions["Middle.m"].ID || functions["Middle.m"].OverrideTarget != functions["Base.m"].ID || functions["Base.m"].OverrideTarget != "" || functions["Peer.m"].OverrideTarget != "" {
		t.Fatal("original two-level override or unchanged inherited selection changed")
	}
	call := functions["callBase"].Body[0].Value
	if call.Kind != "call" || call.Callee != functions["Base.m"].ID || call.Receiver.Binding != functions["callBase"].Parameters[0].ID || call.Receiver.Type != types["Base"].ID || (*call.Arguments)[0].Binding != functions["callBase"].Parameters[1].ID || call.Signature == nil || call.Signature.DeclarationRange != functions["Base.m"].DeclarationRange {
		t.Fatal("static member, original receiver/argument binding or selected signature changed")
	}
	ordered := functions["ordered"].Body[0].Value
	if ordered.Kind != "call" || ordered.Callee != functions["Base.m"].ID || ordered.Receiver.Kind != "call" || ordered.Receiver.Callee != functions["makeReceiver"].ID || len(*ordered.Arguments) != 1 || (*ordered.Arguments)[0].Kind != "call" || (*ordered.Arguments)[0].Callee != functions["makeArgument"].ID || ordered.Receiver.Range.End >= (*ordered.Arguments)[0].Range.Start {
		t.Fatal("receiver then argument once must remain as nested original calls")
	}
	units := utf16.Encode([]rune(string(sourceBytes)))
	for _, occurrence := range []struct {
		v    *arrayGenericExpression
		text string
	}{{call, "b.m(a)"}, {ordered, "makeReceiver().m(makeArgument())"}, {ordered.Receiver, "makeReceiver()"}, {&(*ordered.Arguments)[0], "makeArgument()"}} {
		if string(utf16.Decode(units[occurrence.v.Range.Start:occurrence.v.Range.End])) != occurrence.text {
			t.Fatal("original UTF16 operand/call anchor changed", occurrence.text)
		}
	}
	converted := functions["toBase"].Body[0].Value
	if converted.Kind != "bindingReference" || converted.Binding != functions["toBase"].Parameters[0].ID || converted.FlowType != types["Peer"].ID || len(converted.CompatibilityRoots) != 1 {
		t.Fatal("M12 conversion must retain original Peer parameter and expression root")
	}
	found := false
	for _, row := range p.CompatibilityWitnesses {
		proof := row.Compatibility
		if proof.SourceType == types["Peer"].ID && proof.TargetType == types["Base"].ID && proof.Origin.Start == converted.Range.Start && proof.Origin.End == converted.Range.End {
			pairs := []string{}
			for _, pair := range proof.Members {
				pairs = append(pairs, members[pair.Source]+">"+members[pair.Target])
			}
			slices.Sort(pairs)
			found = proof.Assignable && slices.Equal(pairs, []string{"Peer.m>Base.m", "Peer.n>Base.n"}) && len(row.Dependencies) == 0
		}
	}
	if !found {
		t.Fatal("original explicit Peer-to-Base field and method correspondence missing")
	}
	if len(functions["Base.m"].Body) != 2 || functions["Base.m"].Body[0].Value.Target.Binding != p.Globals[0].Binding.ID || functions["Middle.m"].Body[0].Value.Kind != "elementAssignment" || functions["Middle.m"].Body[1].Value.Kind != "checkedElementAccess" || functions["Derived.m"].Body[0].Kind != "throw" || functions["Peer.m"].Body[0].Value.Target.Binding != p.Globals[1].Binding.ID {
		t.Fatal("full method global/Array/throw bodies were pruned")
	}
	if functions["ordinary"].Body[1].Kind != "try" || functions["entryBase"].Body[0].CatchClause == nil || functions["concreteDerived"].Body[1].CatchClause == nil || functions["makeReceiver"].Body[1].Value.Callee != types["Inherited"].ID || functions["makeArgument"].Body[1].Value.Kind != "arrayLiteral" {
		t.Fatal("ordinary EH, actual construction and argument bodies must remain complete")
	}
	commonMethodOfficial37q(t, string(sourceBytes), functions, types)
}

func commonMethodPacket37q(t *testing.T, fixture string, applications []string) commonProgramPackage {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(commonProgramRequest37n(t, fixture), &request); err != nil {
		t.Fatal(err)
	}
	root := func(kind, name string) closureRoot {
		return closureRoot{Kind: kind, SourcePath: "/src/main.ts", Name: name}
	}
	roots := []closureRoot{}
	for _, name := range applications {
		roots = append(roots, root("application", name))
	}
	roots = append(roots, root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit"))
	request["input"].(map[string]any)["compilationClosure"] = map[string]any{"roots": roots, "requiredRoots": []closureRoot{root("kernel", "left"), root("kernel", "right")}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	input := tsjson.Value(encoded)
	if err := input.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	input = append(input, '\n')
	var output bytes.Buffer
	runErr := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(input), &output)
	if capture := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); capture != "" {
		if err := os.MkdirAll(capture, 0700); err != nil {
			t.Fatal(err)
		}
		for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
			file := filepath.Join(capture, fixture+"-"+suffix)
			old, err := os.ReadFile(file)
			if err == nil {
				if !bytes.Equal(old, data) {
					t.Fatal("retained original capture differs; never overwrite", file)
				}
			} else if os.IsNotExist(err) {
				if err := os.WriteFile(file, data, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Fatal(err)
			}
		}
	}
	if runErr != nil {
		t.Fatal(runErr)
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	return packet
}
