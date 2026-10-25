// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// The complete input is fixed before projection. RHS eligibility follows the
// original bool short-circuit AST; this test never evaluates a numeric trap.
const exceptionShortCircuitSource = "// 😀\r\nexport function skippedAnd(run:bool,zero:i32):bool{try{throw undefined;}catch(caught){const viewed:i32=caught as i32;const alias:i32=viewed;return run&&(alias>zero);}finally{const done:bool=true;}}\r\nexport function skippedOr(run:bool,zero:i32):bool{try{throw undefined;}catch(caught){const viewed:i32=caught as i32;const alias:i32=viewed;return run||(alias>zero);}finally{const done:bool=true;}}"

func exceptionShortCircuitOriginal(t *testing.T, r request) {
	t.Helper()
	p := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, p)
	f := p.GetSourceFile(r.RootFile)
	c, release := p.GetTypeCheckerForFileExclusive(t.Context(), f)
	defer release()
	if f.Text() != exceptionShortCircuitSource || len(f.Statements.Nodes) != 2 || len(p.GetSourceFiles()) != 2 || len(c.GetExportsOfModule(f.AsNode().Symbol())) != 2 {
		t.Fatal("complete original Program and two exports")
	}
	is := func(n, declaration *ast.Node) bool {
		s := c.GetSymbolAtLocation(n)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == declaration
	}
	for i, name := range []string{"skippedAnd", "skippedOr"} {
		n := f.Statements.Nodes[i]
		if n.Kind != ast.KindFunctionDeclaration || n.Name().Text() != name {
			t.Fatal("original ordered functions")
		}
		fn := n.AsFunctionDeclaration()
		if len(fn.Parameters.Nodes) != 2 || !c.GetTypeFromTypeNode(fn.Type).IsBooleanLike() || len(fn.Body.AsBlock().Statements.Nodes) != 1 {
			t.Fatal("original bool function with bool/i32 parameters and complete body")
		}
		for j, p := range fn.Parameters.Nodes {
			annotation := p.AsParameterDeclaration().Type
			want := []string{"bool", "i32"}[j]
			typ := c.GetTypeFromTypeNode(annotation)
			valid := typ.IsBooleanLike()
			if want == "i32" {
				valid = typ.Alias() != nil && typ.Alias().Symbol().Name == want
			}
			if !valid || c.GetTypeAtLocation(p.Name()) != typ {
				t.Fatal("original ordered parameter annotation/type")
			}
		}
		attempt := fn.Body.AsBlock().Statements.Nodes[0].AsTryStatement()
		u := attempt.TryBlock.AsBlock().Statements.Nodes[0].AsThrowStatement().Expression
		builtin := c.GetSymbolAtLocation(u)
		if u.Kind != ast.KindIdentifier || u.Text() != "undefined" || builtin == nil || len(builtin.Declarations) != 0 || c.GetTypeAtLocation(u).Flags()&checker.TypeFlagsUndefined == 0 {
			t.Fatal("official builtin undefined rather than a same-name declaration")
		}
		caught := attempt.CatchClause.AsCatchClause()
		body := caught.Block.AsBlock().Statements.Nodes
		if len(body) != 3 || caught.VariableDeclaration.Name().Text() != "caught" || c.GetTypeAtLocation(caught.VariableDeclaration.Name()) != c.GetUnknownType() {
			t.Fatal("complete current unknown catch body")
		}
		viewed := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		alias := body[1].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		assertion := viewed.AsVariableDeclaration().Initializer
		if assertion.Kind != ast.KindAsExpression || !is(assertion.AsAsExpression().Expression, caught.VariableDeclaration) || c.GetTypeAtLocation(assertion.AsAsExpression().Expression) != c.GetUnknownType() || c.GetTypeAtLocation(assertion) != c.GetTypeFromTypeNode(assertion.AsAsExpression().Type) || !is(alias.AsVariableDeclaration().Initializer, viewed) {
			t.Fatal("unconverted unknown assertion operand followed by own alias")
		}
		ret := body[2].AsReturnStatement().Expression
		if ret.Kind != ast.KindBinaryExpression {
			t.Fatal("original return short circuit")
		}
		logical := ret.AsBinaryExpression()
		operator := []ast.Kind{ast.KindAmpersandAmpersandToken, ast.KindBarBarToken}[i]
		if logical.OperatorToken.Kind != operator || !is(logical.Left, fn.Parameters.Nodes[0]) || logical.Right.Kind != ast.KindParenthesizedExpression || !c.GetTypeAtLocation(ret).IsBooleanLike() {
			t.Fatal("original bool short-circuit operator and ordered operands")
		}
		numeric := logical.Right.AsParenthesizedExpression().Expression.AsBinaryExpression()
		if numeric.OperatorToken.Kind != ast.KindGreaterThanToken || !is(numeric.Left, alias) || !is(numeric.Right, fn.Parameters.Nodes[1]) || !c.GetTypeAtLocation(logical.Right).IsBooleanLike() {
			t.Fatal("RHS numeric comparison retains exact alias and i32 parameter")
		}
		final := attempt.FinallyBlock.AsBlock().Statements.Nodes
		if len(final) != 1 || final[0].Kind != ast.KindVariableStatement || final[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration().Initializer.Kind != ast.KindTrueKeyword {
			t.Fatal("complete normal finally")
		}
		for _, node := range []*ast.Node{n, u, caught.VariableDeclaration, viewed, assertion, assertion.AsAsExpression().Expression, assertion.AsAsExpression().Type, alias, ret, logical.Left, logical.Right, numeric.Left, numeric.Right, numeric.OperatorToken, attempt.FinallyBlock} {
			throwBoundaryNodeForTest(t, f, node)
		}
	}
}

// Reuse the independent official expression/block oracle. Its older test entry
// fixes every signature to i32, so this input owns its actual bool/i32 headers.
func exceptionShortCircuitFactsOracle(t *testing.T, r request, facts response) error {
	t.Helper()
	p := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, p)
	f := p.GetSourceFile(r.RootFile)
	c, release := p.GetTypeCheckerForFileExclusive(t.Context(), f)
	defer release()
	o := exceptionNumericOracle{t: t, file: f, check: c, rows: map[string]symbol{}, ids: map[*ast.Symbol]string{}}
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 || len(facts.Functions) != 2 || len(facts.Functions) != len(f.Statements.Nodes) || len(p.GetSourceFiles()) != len(r.Files) {
		return fmt.Errorf("complete official Program and function table")
	}
	for _, s := range facts.Symbols {
		if s.ID == "" || o.rows[s.ID].ID != "" {
			return fmt.Errorf("distinct owned symbol rows")
		}
		o.rows[s.ID] = s
	}
	for i, n := range f.Statements.Nodes {
		actual := facts.Functions[i]
		if actual.Name != n.Name().Text() || actual.ReturnType != "bool" || actual.Range != o.at(n, false) {
			return fmt.Errorf("original bool function identity/type/range")
		}
		if err := o.bind(n, o.rows[actual.Symbol], "function", "bool", false); err != nil {
			return err
		}
	}
	for i, n := range f.Statements.Nodes {
		actual, decl := facts.Functions[i], n.AsFunctionDeclaration()
		if len(actual.Parameters) != 2 || len(actual.ParameterInitializers) != 2 {
			return fmt.Errorf("complete ordered own parameters")
		}
		for j, param := range decl.Parameters.Nodes {
			if err := o.bind(param, actual.Parameters[j], "parameter", []string{"bool", "i32"}[j], true); err != nil {
				return err
			}
			shape := actual.Parameters[j].Presence
			if actual.ParameterInitializers[j] != nil || shape == nil || shape.Undefined || shape.Null || shape.Optional || shape.Default || shape.Range != o.at(param, false) || shape.Annotation != o.at(param.AsParameterDeclaration().Type, true) {
				return fmt.Errorf("original required parameter declaration")
			}
		}
		if err := o.block(decl.Body.AsBlock().Statements.Nodes, actual.Body); err != nil {
			return err
		}
	}
	if len(o.ids) != len(facts.Symbols) {
		return fmt.Errorf("complete exact declaration catalogue")
	}
	return nil
}

func TestExceptionNumericShortCircuitCompleteProgram(t *testing.T) {
	r := integerRequest(exceptionShortCircuitSource)
	exceptionShortCircuitOriginal(t, r)
	requestBytes := nullableFactsBoundaryCanonical(t, r)
	input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 2, "schemaHash": exceptionNumericFactsSchema, "input": r})
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v2"}, bytes.NewReader(input), &out)
	var packet exceptionFactsPackage
	if decode := json.Unmarshal(out.Bytes(), &packet); decode != nil {
		t.Fatalf("public complete package: %v (producer %v)", decode, err)
	}
	if root := os.Getenv("TSFG_EXCEPTION_SHORT_CIRCUIT_CAPTURE_ROOT"); root != "" {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"request.json": requestBytes, "exception-request.json": input, "package.json": out.Bytes(), "response.json": packet.Payload.Facts, "tested-input-set.json": nullableFactsBoundaryCanonical(t, map[string]any{"rootFile": r.RootFile, "files": r.Files})} {
			if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		for _, f := range r.Files {
			path := filepath.Join(root, "sources", filepath.FromSlash(strings.TrimPrefix(f.Path, "/")))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.exception-facts-package-candidate" || packet.Version != 2 || packet.SchemaHash != exceptionNumericFactsSchema || packet.RequestID != exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.request.v2\x00"), input...)) || packet.PackageID != exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.package.v2\x00"+exceptionNumericFactsSchema+"\x00"), nullableFactsBoundaryCanonical(t, packet.Payload)...)) || packet.Payload.InputIdentity != exceptionPackageExportsDigestForTest(requestBytes) {
		t.Fatal("exact public v2/c01 request/package/input identity")
	}
	var facts response
	if err := json.Unmarshal(packet.Payload.Facts, &facts); err != nil {
		t.Fatal(err)
	}
	if err := exceptionShortCircuitFactsOracle(t, r, facts); err != nil {
		t.Fatal(err)
	}
	files := slices.Clone(r.Files)
	slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	wantFiles := []exceptionFactsFile{}
	for _, f := range files {
		wantFiles = append(wantFiles, exceptionFactsFile{Path: f.Path, Digest: exceptionPackageExportsDigestForTest([]byte(f.Content)), ByteLength: len(f.Content), Length: len(utf16.Encode([]rune(f.Content))), PositionUnit: "utf16"})
	}
	wantExports := []exceptionFactsExport{}
	p := throwBoundaryProgramForTest(t, r)
	f := p.GetSourceFile(r.RootFile)
	for i, n := range f.Statements.Nodes {
		wantExports = append(wantExports, exceptionFactsExport{Name: n.Name().Text(), Target: facts.Functions[i].Symbol, SourcePath: r.RootFile, Range: throwBoundaryNodeForTest(t, f, n), Definition: throwBoundaryNodeForTest(t, f, n.Name())})
	}
	if !reflect.DeepEqual(packet.Payload.Files, wantFiles) || !reflect.DeepEqual(packet.Payload.Exports, []exceptionFactsExportGroup{{SourcePath: r.RootFile, External: true, Bindings: wantExports}}) {
		t.Fatal("complete original files and exported function identities")
	}
	for _, mutation := range []string{"short-kind", "short-operator", "operand-order", "numeric-operator", "numeric-binding", "numeric-anchor", "assertion-erasure"} {
		t.Run(mutation, func(t *testing.T) {
			var changed response
			if err := json.Unmarshal(packet.Payload.Facts, &changed); err != nil {
				t.Fatal(err)
			}
			body := changed.Functions[0].Body[0].Exception.CatchClause.Block.Body
			logical := body[2].Expression
			numeric := logical.Right.Operand
			switch mutation {
			case "short-kind":
				logical.Kind = "binary"
			case "short-operator":
				logical.Operator = "||"
			case "operand-order":
				logical.Left, logical.Right = logical.Right, logical.Left
			case "numeric-operator":
				numeric.Operator = ">="
			case "numeric-binding":
				numeric.Left.Symbol = changed.Functions[0].Parameters[1].ID
			case "numeric-anchor":
				numeric.Range.Start++
			case "assertion-erasure":
				body[0].Initializer.Operand.Type = "i32"
			}
			if exceptionShortCircuitFactsOracle(t, r, changed) == nil {
				t.Fatal("copied response mutation accepted by independent official oracle")
			}
		})
	}
}
