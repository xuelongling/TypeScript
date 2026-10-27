// SPDX-License-Identifier: Apache-2.0
// Original official Counter facts only; no semantic contract or permission.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type counterHostRow37v struct {
	ID               string
	SourcePath       string
	Name             string
	CapabilityID     uint32
	ParameterTypes   []string
	ReturnType       string
	DeclarationRange span
	NameRange        span
	Signature        arrayGenericSignature
}

func counterHostRequest37v(t *testing.T) []byte {
	t.Helper()
	var r commonProgramRequest
	if err := json.Unmarshal(commonProgramRequest37n(t, "positive"), &r); err != nil {
		t.Fatal(err)
	}
	r.Input.Files = []arrayFactsInputFile{}
	for _, name := range []string{"host.d.ts", "main.ts", "relay.ts"} {
		raw, e := os.ReadFile("testdata/common-program-v2/host-counter/" + name)
		if e != nil {
			t.Fatal(e)
		}
		r.Input.Files = append(r.Input.Files, arrayFactsInputFile{Path: "/src/" + name, Content: string(raw), SHA256: arrayFactsSHA(raw)})
	}
	r.Input.Files = append(r.Input.Files, arrayFactsInputFile{Path: "/src/minimal-arrays-v8.d.ts", Content: string(arrayGenericDeclarations), SHA256: arrayFactsSHA(arrayGenericDeclarations)})
	slices.SortFunc(r.Input.Files, func(a, b arrayFactsInputFile) int { return strings.Compare(a.Path, b.Path) })
	r.Input.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: "/src/main.ts", Name: "entry"}, {Kind: "kernel", SourcePath: "/src/main.ts", Name: "direct"}, {Kind: "kernel", SourcePath: "/src/main.ts", Name: "right"}, {Kind: "module-initialization", SourcePath: "/src/main.ts", Name: "$moduleInit"}, {Kind: "module-initialization", SourcePath: "/src/relay.ts", Name: "$moduleInit"}}, RequiredRoots: []closureRoot{}}
	raw, e := arrayFactsCanonical(r)
	if e != nil {
		t.Fatal(e)
	}
	var envelope map[string]json.RawMessage
	if e = json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	var input map[string]json.RawMessage
	if e = json.Unmarshal(envelope["input"], &input); e != nil {
		t.Fatal(e)
	}
	input["hostImports"], e = json.Marshal([]hostImport{{SourcePath: "/src/host.d.ts", Name: "record", CapabilityID: 31, ParameterTypes: []string{"i32"}, ReturnType: "i32"}})
	if e != nil {
		t.Fatal(e)
	}
	envelope["input"], e = json.Marshal(input)
	if e != nil {
		t.Fatal(e)
	}
	raw, e = json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	canonical := tsjson.Value(raw)
	if e = canonical.Canonicalize(); e != nil {
		t.Fatal(e)
	}
	raw = canonical
	return append(raw, '\n')
}
func counterHostOriginal37v(t *testing.T, data []byte) *compiler.Program {
	t.Helper()
	var r commonProgramRequest
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	original := request{SourceRoot: r.Input.SourceRoot, RootFile: r.Input.RootFile}
	original.Config.Strict, original.Config.NoEmit = true, true
	original.Declarations.Version = r.Input.Declarations.Version
	original.Declarations.Paths = append([]string{}, r.Input.Declarations.Paths...)
	original.Declarations.Paths = append(original.Declarations.Paths, "/src/host.d.ts")
	for _, file := range r.Input.Files {
		original.Files = append(original.Files, inputFile{Path: file.Path, Content: file.Content})
	}
	program, _, consumed, failure := newOwnedProgram(original)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(consumed) != 4 {
		t.Fatal("complete original source closure differs", consumed)
	}
	if err := arrayFactsOfficialDiagnostics(t.Context(), program, original.RootFile); err != nil {
		t.Fatal(err)
	}
	file := program.GetSourceFile("/src/host.d.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	if len(file.Statements.Nodes) != 1 {
		t.Fatal("unique original Host declaration absent")
	}
	decl := file.Statements.Nodes[0]
	if decl.Kind != ast.KindFunctionDeclaration || decl.Body() != nil || decl.Name().Text() != "record" {
		t.Fatal("original ambient function differs")
	}
	symbol := check.GetSymbolAtLocation(decl.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != decl {
		t.Fatal("Host Symbol is not unique original declaration")
	}
	d := decl.AsFunctionDeclaration()
	if d.Type == nil || len(d.Parameters.Nodes) != 1 {
		t.Fatal("original signature arity differs")
	}
	var scalarDecl *ast.Node
	for _, n := range program.GetSourceFile("/src/minimal-arrays-v8.d.ts").Statements.Nodes {
		if n.Kind == ast.KindTypeAliasDeclaration && n.Name().Text() == "i32" {
			scalarDecl = n
		}
	}
	for _, annotation := range []*ast.Node{d.Type, d.Parameters.Nodes[0].AsParameterDeclaration().Type} {
		if annotation == nil || annotation.Kind != ast.KindTypeReference || check.GetSymbolAtLocation(annotation.AsTypeReferenceNode().TypeName) != check.GetSymbolAtLocation(scalarDecl.Name()) {
			t.Fatal("original pinned i32 declaration Symbol differs")
		}
	}
	calls := 0
	for _, path := range []string{"/src/main.ts", "/src/relay.ts"} {
		source := program.GetSourceFile(path)
		var visit func(*ast.Node)
		visit = func(n *ast.Node) {
			if n.Kind == ast.KindCallExpression && n.AsCallExpression().Expression.Text() == "record" {
				call := n.AsCallExpression()
				selected := check.GetResolvedSignature(n)
				if check.GetSymbolAtLocation(call.Expression) != symbol || selected == nil || selected.Declaration() != decl || len(selected.Parameters()) != 1 || len(call.Arguments.Nodes) != 1 || check.GetReturnTypeOfSignature(selected) != check.GetTypeFromTypeNode(d.Type) {
					t.Fatal("original selected Host call differs")
				}
				calls++
			}
			n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
		}
		visit(source.AsNode())
	}
	if calls != 3 {
		t.Fatal("all three Host calls must be original", calls)
	}
	t.Log("official complete Program four diagnostic groups0; unique ambient declaration/Symbol/nominal i32 signature and all3 selected calls independently verified")
	return program
}
func counterHostCheck37v(program *compiler.Program, p commonProgramPayload, raw []byte) error {
	var packet struct {
		Payload struct{ HostImports []counterHostRow37v }
	}
	if err := json.Unmarshal(raw, &packet); err != nil {
		return err
	}
	if len(packet.Payload.HostImports) != 1 || len(p.Files) != 4 || len(p.Modules) != 2 || len(p.Functions) != 6 || len(p.Globals) != 1 || len(p.Closure.Roots) != 5 {
		return fmt.Errorf("complete original directories differ")
	}
	file := program.GetSourceFile("/src/host.d.ts")
	node := file.Statements.Nodes[0]
	host := packet.Payload.HostImports[0]
	rangeOf := func(file *ast.SourceFile, n *ast.Node) span {
		return span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(file.Text(), n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	declRange := rangeOf(file, node)
	nameRange := rangeOf(file, node.Name())
	id, _ := arrayFactsBindingID(file.FileName(), nameRange)
	signature, _ := genericNodeID("tsfg.array-facts.signature.v8\x00", node)
	if host.ID != id || host.SourcePath != file.FileName() || host.Name != "record" || host.CapabilityID != 31 || !reflect.DeepEqual(host.ParameterTypes, []string{"i32"}) || host.ReturnType != "i32" || host.DeclarationRange != declRange || host.NameRange != nameRange || host.Signature.ID != signature || host.Signature.SourcePath != file.FileName() || host.Signature.DeclarationRange != declRange || !reflect.DeepEqual(host.Signature.Parameters, []string{"i32"}) || host.Signature.ReturnType != "i32" {
		return fmt.Errorf("original Host identity/signature/UTF16 differs")
	}
	for _, fn := range p.Functions {
		if fn.Name == "record" {
			return fmt.Errorf("Host fabricated as owned function")
		}
	}
	calls := map[string][]*arrayGenericExpression{}
	for i := range p.Functions {
		fn := &p.Functions[i]
		for j := range fn.Body {
			if fn.Body[j].Value != nil {
				calls[fn.SourcePath] = append(calls[fn.SourcePath], fn.Body[j].Value)
			}
		}
	}
	calls["/src/main.ts"] = append(calls["/src/main.ts"], p.Modules[0].Initialization[0].Initializer)
	total := 0
	for path, values := range calls {
		source := program.GetSourceFile(path)
		var expected []*ast.Node
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			if n.Kind == ast.KindCallExpression && n.AsCallExpression().Expression.Text() == "record" {
				expected = append(expected, n)
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
		}
		walk(source.AsNode())
		for _, value := range values {
			if value.Kind != "hostCall" {
				continue
			}
			var actual *ast.Node
			for _, n := range expected {
				if value.Range == rangeOf(source, n) {
					actual = n
				}
			}
			if actual == nil || value.Callee != id || value.CalleeRange == nil || *value.CalleeRange != rangeOf(source, actual.AsCallExpression().Expression) || value.Signature == nil || !reflect.DeepEqual(*value.Signature, host.Signature) || value.Arguments == nil || len(*value.Arguments) != 1 || (*value.Arguments)[0].Range != rangeOf(source, actual.AsCallExpression().Arguments.Nodes[0]) || value.Type != "i32" || value.FlowType != "i32" {
				return fmt.Errorf("original Host call/operand/UTF16 differs")
			}
			arg := actual.AsCallExpression().Arguments.Nodes[0]
			operand := (*value.Arguments)[0]
			if arg.Kind == ast.KindNumericLiteral {
				if operand.Kind != "numericLiteral" || operand.Token != arg.Text() {
					return fmt.Errorf("original literal operand differs")
				}
			} else if arg.Kind == ast.KindIdentifier {
				owner := actual.Parent
				for owner != nil && owner.Kind != ast.KindFunctionDeclaration {
					owner = owner.Parent
				}
				if owner == nil {
					return fmt.Errorf("original parameter owner absent")
				}
				parameter := owner.AsFunctionDeclaration().Parameters.Nodes[0]
				binding, _ := arrayFactsBindingID(path, rangeOf(source, parameter.Name()))
				if operand.Kind != "bindingReference" || operand.Binding != binding {
					return fmt.Errorf("original Host actual parameter binding differs")
				}
			} else {
				return fmt.Errorf("unexpected original argument")
			}
			total++
		}
	}
	if total != 3 {
		return fmt.Errorf("expected3 original hostCall nodes, got%d", total)
	}
	return nil
}
func TestCommonProgramCounterHost37v(t *testing.T) {
	data := counterHostRequest37v(t)
	program := counterHostOriginal37v(t, data)
	root := os.Getenv("TSFG_COMMON_HOST_CAPTURE")
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "counter-request.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(data), &output)
	if err != nil {
		if root != "" {
			if e := os.WriteFile(filepath.Join(root, "counter-failure.txt"), []byte(err.Error()+"\n"), 0600); e != nil {
				t.Fatal(e)
			}
		}
		if output.Len() != 0 {
			t.Fatal("failed producer published partial packet")
		}
		t.Fatal(err)
	}
	if root != "" {
		if err := os.WriteFile(filepath.Join(root, "counter-package.json"), output.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var p commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if err := counterHostCheck37v(program, p.Payload, output.Bytes()); err != nil {
		t.Fatal(err)
	}
	counterHostMutations37v(t, output.Bytes(), func(payload commonProgramPayload, raw []byte) error {
		return counterHostCheck37v(program, payload, raw)
	})
}
