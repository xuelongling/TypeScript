// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Every case is a complete frozen Program. The official oracle observes it
// before the producer and joins captures through original declaration Symbols.
func TestCommonProgramNullableCapture19x(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/nullable-capture")
	}
	raw, err := os.ReadFile("testdata/common-program-v2/nullable-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, SHA, Failure string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/nullable-capture/" + sample.Name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete original Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			request := scalarParameterRequest19m(t, source)
			applicationRoot := sample.Name == "root-callable-null" || sample.Name == "root-callable-undefined" || sample.Name == "root-callable-null-undefined"
			if applicationRoot {
				var input commonProgramRequest
				if err := json.Unmarshal(request, &input); err != nil {
					t.Fatal(err)
				}
				input.Input.CompilationClosure.Roots = append(input.Input.CompilationClosure.Roots, closureRoot{Kind: "application", SourcePath: "/src/main.ts", Name: "rootNullableCallable"})
				sort.Slice(input.Input.CompilationClosure.Roots, func(i, j int) bool {
					a, b := input.Input.CompilationClosure.Roots[i], input.Input.CompilationClosure.Roots[j]
					return a.Kind+"\x00"+a.SourcePath+"\x00"+a.Name < b.Kind+"\x00"+b.SourcePath+"\x00"+b.Name
				})
				request, err = arrayFactsCanonical(input)
				if err != nil {
					t.Fatal(err)
				}
				request = append(request, '\n')
			}
			readSpanCaptureSave26i(t, sample.Name+"-request.json", request)
			var compare func(commonProgramPayload)
			if sample.Failure != "r03-array-facts-types" {
				compare = unionCaptureOfficial19x(t, string(source), sample.Failure == "")
			}
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if err != nil {
				failure, ok := err.(*arrayFactsFailure)
				if !ok || output.Len() != 0 {
					t.Fatalf("non-atomic original failure: %v/%d", err, output.Len())
				}
				raw, e := json.Marshal(failure)
				if e != nil {
					t.Fatal(e)
				}
				readSpanCaptureSave26i(t, sample.Name+"-failure.json", append(raw, '\n'))
				t.Log(string(raw))
				if sample.Failure != failure.Code {
					t.Fatalf("wanted %q, got %s", sample.Failure, failure.Code)
				}
				if sample.Failure == "r03-array-facts-capability" {
					details := map[string]string{
						"block-local":     "block capture requires an initialized scalar local",
						"deferred-init":   "dense local requires initialization",
						"multi-payload":   "nullable capture requires one supported payload and null/undefined presence",
						"span-payload":    "nullable capture requires one supported payload and null/undefined presence",
						"generic-payload": "closed original nominal class required",
					}
					if expected := details[sample.Name]; expected == "" || failure.Detail != expected {
						t.Fatal("wrong original capability boundary", failure.Detail)
					}
				}
				return
			}
			if sample.Failure != "" {
				t.Fatal("unsupported original program published")
			}
			readSpanCaptureSave26i(t, sample.Name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			compare(packet.Payload)
			if applicationRoot {
				found := 0
				for _, root := range packet.Payload.Closure.Roots {
					if root.Name != "rootNullableCallable" {
						continue
					}
					for _, fn := range packet.Payload.Functions {
						if fn.ID == root.Target && fn.Name == "rootNullableCallable" && fn.SourcePath == "/src/main.ts" && root.Kind == "application" && root.SourcePath == fn.SourcePath && len(fn.Parameters) == 1 {
							found++
						}
					}
				}
				if found != 1 {
					t.Fatal("original nullable callable application root missing or duplicated")
				}
			}
		})
	}
}

func unionCaptureOfficial19x(t *testing.T, source string, supported bool) func(commonProgramPayload) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	t.Cleanup(release)
	anchor := func(n *ast.Node) span {
		source := ast.GetSourceFileOfNode(n).Text()
		start := n.Pos()
		for start < n.End() && (source[start] == ' ' || source[start] == '\n' || source[start] == '\r' || source[start] == '\t') {
			start++
		}
		return span{len(utf16.Encode([]rune(source[:start]))), len(utf16.Encode([]rune(source[:n.End()])))}
	}
	owner := func(n *ast.Node) *ast.Node {
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Kind == ast.KindFunctionDeclaration {
				return p
			}
		}
		return nil
	}
	declarations := map[*ast.Symbol]*ast.Node{}
	functions := []*ast.Node{}
	calls := map[span]*ast.Node{}
	var visit func(*ast.Node)
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindFunctionDeclaration {
			functions = append(functions, n)
			signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(n.Name()), checker.SignatureKindCall)
			if len(signatures) != 1 || signatures[0].Declaration() != n {
				t.Fatal("original function signature differs")
			}
		}
		if n.Kind == ast.KindCallExpression || n.Kind == ast.KindNewExpression {
			signature := check.GetResolvedSignature(n)
			if signature == nil {
				t.Fatal("original selected signature missing")
			}
			if signature.Declaration() == nil {
				n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
				return
			}
			calls[anchor(n)] = signature.Declaration()
		}
		if (n.Kind == ast.KindVariableDeclaration || n.Kind == ast.KindParameter) && owner(n) != nil && n.Type() != nil && check.GetTypeFromTypeNode(n.Type()).IsUnion() {
			absence := false
			for _, member := range check.GetTypeFromTypeNode(n.Type()).Types() {
				absence = absence || member.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0
			}
			annotation := n.Type()
			for annotation.Kind == ast.KindParenthesizedType {
				annotation = annotation.AsParenthesizedTypeNode().Type
			}
			if !absence && annotation.Kind != ast.KindUnionType {
				n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
				return
			}
			symbol := check.GetSymbolAtLocation(n.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != n {
				t.Fatal("unique original nullable binding Symbol missing")
			}
			declarations[symbol] = n
		}
		n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
	}
	visit(file.AsNode())
	if len(declarations) == 0 {
		t.Fatal("independent union declaration oracle empty")
	}
	uses := map[*ast.Node]map[*ast.Node][]commonProgramCaptureOccurrence{}
	flows := map[span]struct {
		declaration *ast.Node
		actual      *checker.Type
	}{}
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindIdentifier {
			if declaration := declarations[check.GetSymbolAtLocation(n)]; declaration != nil && n != declaration.Name() {
				pureWrite := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && n.Parent.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken
				if !pureWrite {
					flows[anchor(n)] = struct {
						declaration *ast.Node
						actual      *checker.Type
					}{declaration, check.GetTypeAtLocation(n)}
				}
				fn := owner(n)
				if fn != owner(declaration) {
					if uses[fn] == nil {
						uses[fn] = map[*ast.Node][]commonProgramCaptureOccurrence{}
					}
					write := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && ast.IsAssignmentOperator(n.Parent.AsBinaryExpression().OperatorToken.Kind)
					read := !write || n.Parent.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken
					uses[fn][declaration] = append(uses[fn][declaration], commonProgramCaptureOccurrence{Range: anchor(n), Read: read, Write: write})
				}
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
	}
	visit(file.AsNode())
	if len(uses) == 0 {
		t.Fatal("independent union capture oracle empty")
	}
	t.Log("pre-producer original official diagnostics, union Symbols/types, selected signatures, flow and UTF16 accesses verified")
	return func(payload commonProgramPayload) {
		t.Helper()
		if !supported {
			t.Fatal("unsupported oracle consumed a packet")
		}
		types := map[string]arrayGenericType{}
		for _, typ := range payload.Types {
			types[typ.ID] = typ
		}
		// The annotation is interpreted at the original official declaration. The
		// null/undefined constituents come from checker objects, never DTO spelling.
		var match func(string, *checker.Type) bool
		match = func(id string, actual *checker.Type) bool {
			typ := types[id]
			switch {
			case actual.Flags()&checker.TypeFlagsNull != 0:
				return typ.Kind == "null"
			case actual.Flags()&checker.TypeFlagsUndefined != 0:
				return typ.Kind == "undefined"
			case actual.Flags()&checker.TypeFlagsStringLike != 0:
				return typ.Kind == "string"
			case actual.Flags()&checker.TypeFlagsNumberLike != 0:
				return typ.Kind == "scalar" && typ.ID == "i32"
			case actual.Flags()&checker.TypeFlagsBooleanLike != 0:
				return typ.Kind == "scalar" && typ.ID == "bool"
			case check.IsArrayType(actual):
				arguments := check.GetTypeArguments(actual)
				return typ.Kind == "array" && len(arguments) == 1 && match(typ.ElementType, arguments[0])
			}
			if signatures := check.GetSignaturesOfType(actual, checker.SignatureKindCall); len(signatures) == 1 {
				if typ.Kind != "function" || typ.Parameters == nil || len(*typ.Parameters) != len(signatures[0].Parameters()) || !match(typ.ReturnType, check.GetReturnTypeOfSignature(signatures[0])) {
					return false
				}
				for index, parameter := range signatures[0].Parameters() {
					if !match((*typ.Parameters)[index], check.GetTypeOfSymbol(parameter)) {
						return false
					}
				}
				return true
			}
			symbol := actual.Symbol()
			if symbol == nil || len(symbol.Declarations) != 1 {
				return false
			}
			declaration := symbol.Declarations[0]
			kind := "reference"
			if declaration.Modifiers() != nil {
				for _, modifier := range declaration.Modifiers().Nodes {
					if modifier.Kind == ast.KindDecorator {
						marker := check.GetSymbolAtLocation(modifier.AsDecorator().Expression)
						if marker == nil || marker.Name != "tsfgValue" || len(marker.Declarations) != 1 || ast.GetSourceFileOfNode(marker.Declarations[0]).FileName() != "/src/minimal-arrays-v8.d.ts" {
							t.Fatal("original nullable nominal marker differs")
						}
						kind = "value"
					}
				}
			}
			return typ.Kind == kind && typ.SourcePath == ast.GetSourceFileOfNode(declaration).FileName() && typ.DeclarationRange != nil && *typ.DeclarationRange == anchor(declaration)
		}
		bindings := map[*ast.Node]arrayFactsBinding{}
		for _, fn := range payload.Functions {
			for _, binding := range append(fn.Parameters, fn.Bindings...) {
				for _, declaration := range declarations {
					if binding.DeclarationRange == anchor(declaration) {
						declared := check.GetTypeFromTypeNode(declaration.Type())
						typ := types[binding.DeclaredType]
						if typ.Kind != "union" {
							t.Fatal("original nullable constituents differ")
						}
						for _, constituent := range declared.Types() {
							found := 0
							for _, member := range typ.Members {
								if match(member, constituent) {
									found++
								}
							}
							if found != 1 {
								t.Fatal("original nullable constituent identity differs")
							}
						}
						for _, member := range typ.Members {
							found := false
							for _, constituent := range declared.Types() {
								found = found || match(member, constituent)
							}
							if !found {
								t.Fatal("extra nullable constituent published")
							}
						}
						if binding.Name != declaration.Name().Text() || binding.NameRange != anchor(declaration.Name()) || binding.Writable != (declaration.Kind == ast.KindParameter || declaration.Parent.Flags&ast.NodeFlagsConst == 0) || (binding.Kind == "parameter") != (declaration.Kind == ast.KindParameter) || binding.LexicalOwner == nil || *binding.LexicalOwner == nil || **binding.LexicalOwner != fn.ID {
							t.Fatal("original nullable binding/owner/writable differs")
						}
						bindings[declaration] = binding
					}
				}
			}
		}
		for fn, captures := range uses {
			for declaration, occurrences := range captures {
				binding, ok := bindings[declaration]
				if !ok {
					t.Fatal("original nullable declaration missing")
				}
				found := 0
				for _, projected := range payload.Functions {
					if projected.DeclarationRange == anchor(fn) {
						for _, capture := range *projected.Captures {
							if capture.Binding == binding.ID {
								read, write := false, false
								for _, use := range occurrences {
									read = read || use.Read
									write = write || use.Write
								}
								if !reflect.DeepEqual(capture.Occurrences, occurrences) || capture.Read != read || capture.Write != write {
									t.Fatal("original shared slot access occurrences differ")
								}
								found++
							}
						}
					}
				}
				if found != 1 {
					t.Fatal("original nullable capture missing or duplicated")
				}
			}
		}
		observedFlows := map[span]bool{}
		observedCalls := map[span]bool{}
		var expression func(*arrayGenericExpression)
		expression = func(value *arrayGenericExpression) {
			if value == nil {
				return
			}
			if expected, ok := flows[value.Range]; ok && value.Kind == "bindingReference" {
				binding := bindings[expected.declaration]
				if value.Binding != binding.ID || value.Type != binding.DeclaredType {
					t.Fatal("original nullable reference changed slots")
				}
				actual := []*checker.Type{expected.actual}
				if expected.actual.IsUnion() {
					actual = expected.actual.Types()
				}
				projected := []string{value.FlowType}
				if types[value.FlowType].Kind == "union" {
					projected = types[value.FlowType].Members
				}
				// bool's two literal checker constituents share one logical bool payload.
				expectedMembers := []string{}
				for _, member := range types[binding.DeclaredType].Members {
					for _, constituent := range actual {
						if match(member, constituent) {
							expectedMembers = append(expectedMembers, member)
							break
						}
					}
				}
				sort.Strings(expectedMembers)
				sort.Strings(projected)
				if !reflect.DeepEqual(projected, expectedMembers) {
					t.Fatalf("original checker narrowing differs at %+v: %v/%v", value.Range, projected, expectedMembers)
				}
				observedFlows[value.Range] = true
			}
			if declaration, ok := calls[value.Range]; ok && value.Signature != nil {
				if value.Signature.SourcePath != ast.GetSourceFileOfNode(declaration).FileName() || value.Signature.DeclarationRange != anchor(declaration) {
					t.Fatalf("original selected call/constructor signature differs at %+v: packet=%+v original=%+v", value.Range, value.Signature, anchor(declaration))
				}
				observedCalls[value.Range] = true
			}
			for _, child := range []*arrayGenericExpression{value.Receiver, value.Index, value.Expression, value.Value, value.Left, value.Right} {
				expression(child)
			}
			if value.Target != nil {
				expression(value.Target.Receiver)
				expression(value.Target.Index)
			}
			for _, children := range []*[]arrayGenericExpression{value.Arguments, value.Elements} {
				if children != nil {
					for _, child := range *children {
						expression(&child)
					}
				}
			}
		}
		var statements func([]arrayGenericStatement)
		statements = func(rows []arrayGenericStatement) {
			for _, row := range rows {
				expression(row.Initializer)
				expression(row.Value)
				expression(row.Condition)
				for _, child := range []*[]arrayGenericStatement{row.Then, row.Else, row.Body} {
					if child != nil {
						statements(*child)
					}
				}
				if row.CatchClause != nil {
					statements(row.CatchClause.Block.Body)
				}
				if row.FinallyBlock != nil {
					statements(row.FinallyBlock.Body)
				}
			}
		}
		for _, fn := range payload.Functions {
			statements(fn.Body)
		}
		// Assignment targets are separate facts; their original capture occurrences
		// were checked above. All other original nullable reads must retain flow.
		for location, expected := range flows {
			if !observedFlows[location] {
				writeOnly := false
				for _, captures := range uses {
					for declaration, occurrences := range captures {
						if declaration == expected.declaration {
							for _, occurrence := range occurrences {
								if occurrence.Range == location && occurrence.Write && !occurrence.Read {
									writeOnly = true
								}
							}
						}
					}
				}
				if !writeOnly {
					t.Fatalf("original nullable read missing at %+v", location)
				}
			}
		}
		if len(observedCalls) == 0 {
			t.Fatal("selected signature packet oracle empty")
		}
	}
}
