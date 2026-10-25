// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func commonProgramActualRequest37p(t *testing.T, name string) []byte {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(commonProgramRequest37n(t, name), &request); err != nil {
		t.Fatal(err)
	}
	root := func(kind, name string) closureRoot {
		return closureRoot{Kind: kind, SourcePath: "/src/main.ts", Name: name}
	}
	request["input"].(map[string]any)["compilationClosure"] = map[string]any{
		"roots":         []closureRoot{root("application", "consumeUndefined"), root("application", "ordinary"), root("application", "transportBool"), root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit")},
		"requiredRoots": []closureRoot{root("kernel", "left"), root("kernel", "right")},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return append(canonical, '\n')
}

func commonProgramActualCatch37p(t *testing.T, fn arrayGenericFunction, kind, boxField string, thrown, assertion span) {
	t.Helper()
	if len(fn.Body) < 3 || fn.Body[0].Initializer == nil || fn.Body[0].Initializer.Kind != "new" || fn.Body[1].Kind != "try" {
		t.Fatal("actual allocation and exception region must remain")
	}
	attempt := fn.Body[1]
	if attempt.TryBlock == nil || len(attempt.TryBlock.Body) != 1 || attempt.TryBlock.Body[0].Kind != "throw" || attempt.CatchClause == nil {
		t.Fatal("original throw and catch must remain")
	}
	value := attempt.TryBlock.Body[0].Value
	if value == nil || value.Kind != kind || value.Range != thrown || kind == "booleanLiteral" && value.Token != "false" {
		t.Fatal("original false/undefined throw or UTF16 anchor changed")
	}
	caught := attempt.CatchClause
	if caught.Binding == nil || caught.Binding.DeclaredType != "unknown" || len(caught.Block.Body) != 1 {
		t.Fatal("catch must retain its actual unknown binding and field assignment")
	}
	store := caught.Block.Body[0].Value
	if store == nil || store.Kind != "fieldAssignment" || store.Target == nil || store.Target.Property != boxField || store.Target.Receiver == nil || store.Target.Receiver.Binding != fn.Body[0].Binding || store.Value == nil {
		t.Fatal("field store must join its original Box allocation")
	}
	cast := store.Value
	if cast.Kind != "as" || cast.Type != "i32" || cast.Range != assertion || cast.Expression == nil || cast.Expression.Kind != "bindingReference" || cast.Expression.Binding != caught.Binding.ID || cast.Expression.Type != "unknown" {
		t.Fatal("pure assertion must preserve its original unknown operand and anchor")
	}
}

// Both whole sources must project successfully. Numeric actual-domain decisions
// belong to the common Core consumer, not to the original Source facts producer.
func TestCommonProgramActualStoragePublic37p(t *testing.T) {
	for _, tc := range []struct {
		name, sha     string
		length, shift int
		boolReturn    span
	}{
		{"actual-storage", "8288c88ed11e6c428ee2935286e18ab6d3a65f825f98a8897039a2616240adb7", 980, 0, span{762, 767}},
		{"actual-storage-bool-consume", "54eb2d2cc2cd0710c62a921099e3da812f6801652f13e314baed36fe602b9bf5", 989, 9, span{762, 776}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := commonProgramActualRequest37p(t, tc.name)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(input), &output); err != nil {
				t.Fatal(err)
			}
			if root := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
					if err := os.WriteFile(filepath.Join(root, tc.name+"-"+suffix), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			p := packet.Payload
			if packet.Family != "tsfg.program-facts-package-candidate" || packet.Version != 1 || len(p.Files) != 2 || len(p.Functions) != 7 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Modules[0].Initialization) != 2 || len(p.Closure.Roots) != 6 || len(p.Closure.RequiredRoots) != 2 {
				t.Fatal("complete actual-storage Program inventory changed")
			}
			if p.Files[0].Path != "/src/main.ts" || p.Files[0].SHA256 != tc.sha || p.Files[0].UTF16Length != tc.length || p.Globals[0].Binding.Name != "leftTotal" || p.Globals[1].Binding.Name != "rightTotal" {
				t.Fatal("original source identity or global inventory changed")
			}
			functions := map[string]arrayGenericFunction{}
			names := []string{}
			for _, fn := range p.Functions {
				functions[fn.Name], names = fn, append(names, fn.Name)
			}
			if !slices.Equal(names, []string{"add", "left", "right", "ordinary", "side", "transportBool", "consumeUndefined"}) {
				t.Fatal("original function order changed")
			}
			for i, name := range []string{"consumeUndefined", "ordinary", "transportBool", "left", "right", "$moduleInit"} {
				root := p.Closure.Roots[i]
				if root.SourcePath != "/src/main.ts" || root.Name != name || i < 5 && root.Target != functions[name].ID || i == 5 && root.Target != "/src/main.ts" {
					t.Fatal("root must join its actual original function or module")
				}
			}
			for i, name := range []string{"left", "right"} {
				if p.Closure.RequiredRoots[i].Name != name || p.Closure.RequiredRoots[i].Target != functions[name].ID {
					t.Fatal("required kernel roots changed")
				}
			}
			classNames, boxField := []string{}, ""
			for _, typ := range p.Types {
				if typ.Kind == "value" || typ.Kind == "reference" {
					classNames = append(classNames, typ.Name)
				}
				if typ.Name == "Box" && typ.Fields != nil && len(*typ.Fields) == 1 {
					boxField = (*typ.Fields)[0].ID
				}
			}
			slices.Sort(classNames)
			if !slices.Equal(classNames, []string{"Box", "Delta"}) || boxField == "" {
				t.Fatal("both original classes and Box field must remain")
			}
			transport, consume := functions["transportBool"], functions["consumeUndefined"]
			commonProgramActualCatch37p(t, transport, "booleanLiteral", boxField, span{694, 699}, span{734, 747})
			commonProgramActualCatch37p(t, consume, "undefinedLiteral", boxField, span{862 + tc.shift, 871 + tc.shift}, span{906 + tc.shift, 919 + tc.shift})
			if len(transport.Body) != 3 || len(consume.Body) != 4 {
				t.Fatal("complete transport and consume bodies changed")
			}
			alias, use := consume.Body[2], consume.Body[3].Value
			if alias.Initializer == nil || alias.Initializer.Kind != "fieldAccess" || alias.Initializer.Property != boxField || alias.Initializer.Range != (span{946 + tc.shift, 951 + tc.shift}) || use == nil || use.Kind != "binary" || use.Operator != "+" || use.Range != (span{962 + tc.shift, 976 + tc.shift}) || use.Left == nil || use.Right == nil || use.Left.Binding != alias.Binding || use.Left.Range != (span{962 + tc.shift, 967 + tc.shift}) || use.Right.Kind != "call" || use.Right.Callee != functions["side"].ID || use.Right.Range != (span{970 + tc.shift, 976 + tc.shift}) {
				t.Fatal("field load, alias and left-before-right side call must retain original anchors")
			}
			returned := transport.Body[2].Value
			if returned == nil || returned.Range != tc.boolReturn {
				t.Fatal("original bool transport return anchor changed")
			}
			if tc.shift == 0 {
				if returned.Kind != "fieldAccess" || returned.Property != boxField {
					t.Fatal("bool transport must remain a pure field load")
				}
			} else if returned.Kind != "binary" || returned.Operator != "+" || returned.Left == nil || returned.Left.Kind != "fieldAccess" || returned.Left.Property != boxField || returned.Right == nil || returned.Right.Callee != functions["side"].ID {
				t.Fatal("bool numeric-consume source must retain both original operands")
			}
			side := functions["side"]
			if len(side.Body) != 2 || side.Body[0].Value == nil || side.Body[0].Value.Target == nil || side.Body[0].Value.Target.Binding != p.Globals[1].Binding.ID || side.Body[0].Value.Operator != "+=" || side.Body[1].Value == nil || side.Body[1].Value.Decimal != "1" {
				t.Fatal("side() must retain its original rightTotal update and integer return")
			}
		})
	}
}
