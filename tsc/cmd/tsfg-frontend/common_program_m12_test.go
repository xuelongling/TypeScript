// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

// Source-derived expectations were frozen before this public producer ran.
// This checks original M12 authority, never runtime execution or kernel safety.
func TestCommonProgramM12Public37o(t *testing.T) {
	var request map[string]any
	if err := json.Unmarshal(commonProgramRequest37n(t, "m12-witness-forward"), &request); err != nil {
		t.Fatal(err)
	}
	root := func(kind, name string) closureRoot {
		return closureRoot{Kind: kind, SourcePath: "/src/main.ts", Name: name}
	}
	request["input"].(map[string]any)["compilationClosure"] = map[string]any{
		"roots":         []closureRoot{root("application", "compatibilityEntry"), root("application", "derivedEntry"), root("application", "ordinary"), root("application", "relationRoundtrip"), root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit")},
		"requiredRoots": []closureRoot{root("kernel", "left"), root("kernel", "right")},
	}
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
	runErr := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(input), &output)
	if capture := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); capture != "" {
		if err := os.MkdirAll(capture, 0700); err != nil {
			t.Fatal(err)
		}
		for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
			if err := os.WriteFile(filepath.Join(capture, "m12-witness-forward-"+suffix), data, 0600); err != nil {
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
	p := packet.Payload
	if len(p.Functions) != 8 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Closure.Roots) != 7 || len(p.EntryCompatibilityRoots) != 6 {
		t.Fatal("complete free functions, globals, module, roots or exported parameter inventory changed")
	}
	if p.Files[0].SHA256 != "3d9806429b949289ff808b228aceac45b51c14e53d9b797405330cf954b1f380" || p.Files[0].UTF16Length != 1074 {
		t.Fatal("frozen original Source bytes or UTF16 identity changed")
	}
	functions, names := map[string]arrayGenericFunction{}, []string{}
	for _, fn := range p.Functions {
		functions[fn.Name], names = fn, append(names, fn.Name)
	}
	if !slices.Equal(names, []string{"add", "left", "right", "ordinary", "convert", "compatibilityEntry", "relationRoundtrip", "derivedEntry"}) {
		t.Fatal("ordinary functions or original declaration order were pruned")
	}
	types, typeNames, fieldNames := map[string]arrayGenericType{}, map[string]string{}, map[string]string{}
	constructors, fields := 0, 0
	for _, typ := range p.Types {
		if typ.Kind != "reference" && typ.Kind != "value" {
			continue
		}
		types[typ.Name], typeNames[typ.ID] = typ, typ.Name
		if typ.Constructor != nil {
			constructors++
		}
		for _, field := range *typ.Fields {
			fields++
			fieldNames[field.ID] = typ.Name + "." + field.Name
		}
	}
	if len(types) != 6 || constructors != 1 || fields != 8 {
		t.Fatal("all six classes/eight own fields/embedded constructor must survive")
	}
	expected := map[string]string{"786:787:X>Y": "X.n>Y.n,X.next>Y.next", "826:830:X>Y": "X.n>Y.n,X.next>Y.next", "887:891:Y>X": "Y.n>X.n,Y.next>X.next"}
	if len(p.CompatibilityWitnesses) != len(expected) {
		t.Fatal("exact same-origin closed M12 directory changed")
	}
	for _, record := range p.CompatibilityWitnesses {
		proof := record.Compatibility
		key := fmt.Sprintf("%d:%d:%s>%s", proof.Origin.Start, proof.Origin.End, typeNames[proof.SourceType], typeNames[proof.TargetType])
		wanted, ok := expected[key]
		if !ok || !proof.Assignable || proof.Origin.Source != "/src/main.ts" || len(proof.Members) != 2 || len(record.Dependencies) != 1 {
			t.Fatalf("wrong M12 origin/pair/closure: %s", key)
		}
		members := []string{}
		for _, member := range proof.Members {
			members = append(members, fieldNames[member.Source]+">"+fieldNames[member.Target])
		}
		slices.Sort(members)
		if fmt.Sprintf("%s,%s", members[0], members[1]) != wanted || record.Dependencies[0].SourceType != proof.SourceType || record.Dependencies[0].TargetType != proof.TargetType {
			t.Fatal("original explicit field correspondence or recursive Array member dependency changed")
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		t.Fatal("missing original M12 occurrence")
	}
	selected := map[string]string{"compatibilityEntry": "X>Y", "relationRoundtrip": "Y>X"}
	for _, entry := range p.EntryCompatibilityRoots {
		var name string
		for fnName, fn := range functions {
			if fn.ID == entry.FunctionID {
				name = fnName
			}
		}
		wanted, nonempty := selected[name]
		if !nonempty {
			if len(entry.Roots) != 0 {
				t.Fatal("invented scalar/value entry candidate")
			}
			continue
		}
		if len(entry.Roots) != 1 || typeNames[entry.Roots[0].SourceType]+">"+typeNames[entry.Roots[0].TargetType] != wanted || entry.ParameterBinding != functions[name].Parameters[0].ID {
			t.Fatal("actual entry must join its original exported parameter and structural candidate")
		}
		delete(selected, name)
	}
	if len(selected) != 0 {
		t.Fatal("missing nonempty original entry")
	}
	base, derived := types["Base"], types["Derived"]
	forward := derived.Construction.ImplicitForward
	if derived.Base != base.ID || derived.Constructor != nil || derived.Construction.Kind != "implicit-derived" || forward == nil || forward.Target != base.Construction.ID || len(forward.Parameters) != 1 || len(derived.Construction.Parameters) != 1 {
		t.Fatal("derived construction must retain actual implicit forwarding")
	}
	if derived.Construction.Parameters[0].Type != "i32" || derived.Construction.Parameters[0].InheritedFrom != base.Construction.Parameters[0].Symbol || forward.Parameters[0].Source != derived.Construction.Parameters[0].Symbol || forward.Parameters[0].Target != base.Construction.Parameters[0].Symbol || !forward.Parameters[0].Compatibility.Assignable || len(forward.Parameters[0].Compatibility.Members) != 0 {
		t.Fatal("forwarded parameter must preserve original scalar identity and base binding")
	}
	if functions["convert"].Body[0].Value.Kind != "bindingReference" || len(functions["convert"].Body[0].Value.CompatibilityRoots) != 1 || functions["ordinary"].Body[1].Kind != "try" {
		t.Fatal("expression root or full ordinary EH body was dropped")
	}
}
