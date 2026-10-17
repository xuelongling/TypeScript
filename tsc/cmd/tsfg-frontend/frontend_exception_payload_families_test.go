// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Every candidate follows its independently checked existing return/local/new
// control. These are owned Source facts; components do not claim GC execution.
// Bare null has no nominal context in this slice and is deliberately not guessed.
func TestFrontendExceptionPayloadFamiliesOwnedFacts(t *testing.T) {
	reject := func(t *testing.T, invalid bool, reason string) {
		t.Helper()
		if invalid {
			t.Fatal(reason)
		}
	}
	for _, tc := range []struct{ name, annotation, payload, category, className, mode string }{
		{"i16", "i16", "i16", "", "", "reference"},
		{"f32", "f32", "f32", "", "", "reference"},
		{"null", "i32 | null", "i32", "", "", "reference"},
		{"undefined", "i32 | undefined", "i32", "", "", "reference"},
		{"callable", "() => bool", "function", "", "", "reference"},
		{"value-local", "i32", "a1", "value", "Pair", "local"},
		{"value-new", "Pair", "a1", "value", "Pair", "new"},
		{"component-local", "i32", "a1", "component", "Box", "local"},
		{"component-new", "Box", "a1", "component", "Box", "new"},
		{"value-unknown-write", "i32", "a1", "value", "Pair", "write"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(t *testing.T, throwing bool) {
				marker := "tsfgValue"
				if tc.category == "component" {
					marker = "tsfgComponent"
				}
				source := "// 😀\r\n"
				if tc.category != "" {
					source += "@" + marker + " class " + tc.className + " {}\r\n"
				}
				signature, body := "payload: "+tc.annotation, "return payload;"
				if throwing {
					body = "throw payload;"
				}
				if tc.mode == "local" || tc.mode == "write" {
					signature = "value: i32"
					body = "const payload: " + tc.className + " = new " + tc.className + "(); return value;"
					if throwing {
						body = "const payload: " + tc.className + " = new " + tc.className + "(); throw payload;"
					}
				}
				if tc.mode == "new" {
					signature = ""
					body = "return new " + tc.className + "();"
					if throwing {
						body = "throw new " + tc.className + "();"
					}
				}
				if tc.mode == "write" && throwing {
					body = "try { throw value; } catch (caught) { throw (caught = new Pair()); }"
				}
				source += "export function entry(" + signature + "): " + tc.annotation + " { " + body + " }"
				r := integerRequest(source)
				if tc.name == "f32" {
					r = floatRequest(source)
				}
				if tc.category != "" {
					r = aggregateRequest(t, source)
				}
				input, err := json.Marshal(r)
				t.Logf("TSFG_EXCEPTION_PAYLOAD_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
				t.Logf("TSFG_EXCEPTION_PAYLOAD_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					after, e := json.Marshal(r)
					if e != nil || !bytes.Equal(after, input) {
						t.Fatal("original request mutated")
					}
				}()
				program := throwBoundaryProgramForTest(t, r)
				if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
					return
				}
				raw, token := map[string]span{}, map[string]span{}
				var expectedAggregate any
				if !t.Run("officialPayloadIdentitySymbolsSignaturesExportsUTF16", func(t *testing.T) {
					file := program.GetSourceFile(r.RootFile)
					count := 1
					if tc.category != "" {
						count = 2
					}
					reject(t, file == nil || file.Text() != source || len(file.Statements.Nodes) != count, "original declaration closure changed")
					node := file.Statements.Nodes[count-1]
					reject(t, node.Kind != ast.KindFunctionDeclaration || node.Name().Text() != "entry", "one original entry required")
					fn := node.AsFunctionDeclaration()
					check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
					defer release()
					anchor := func(label string, n *ast.Node) {
						t.Helper()
						if n == nil || n.Pos() < 0 || n.End() > len(source) {
							t.Fatalf("missing official %s anchor", label)
						}
						start := n.Pos()
						if n == node {
							start = strings.Index(source, "export function")
						} else if label == "class" || label == "marker" {
							start = strings.Index(source, "@"+marker)
						} else {
							for start < n.End() && strings.ContainsRune(" \t\r\n", rune(source[start])) {
								start++
							}
						}
						raw[label] = throwBoundaryNodeForTest(t, file, n)
						token[label] = throwBoundarySpanForTest(source, start, n.End())
						if raw[label].Start > token[label].Start || raw[label].End != token[label].End || sourceText(t, source, token[label]) == "" {
							t.Fatalf("original raw/token UTF16 %s mismatch", label)
						}
					}
					anchor("function", node)
					anchor("definition", fn.Name())
					anchor("functionBody", fn.Body)
					anchor("returnAnnotation", fn.Type)
					reject(t, token["function"].Start <= 0 || raw["function"].End != len(utf16.Encode([]rune(source))), "CRLF/non-BMP source anchor lost")
					fnSymbol := check.GetSymbolAtLocation(fn.Name())
					calls := check.GetSignaturesOfType(check.GetTypeOfSymbol(fnSymbol), checker.SignatureKindCall)
					returnType := check.GetTypeFromTypeNode(fn.Type)
					reject(t, fnSymbol == nil || len(fnSymbol.Declarations) != 1 || fnSymbol.Declarations[0] != node || len(calls) != 1 || calls[0].Declaration() != node || check.GetReturnTypeOfSignature(calls[0]) != returnType, "official entry declaration/signature identity changed")
					exports := check.GetExportsOfModule(file.AsNode().Symbol())
					reject(t, len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != node || ast.GetSourceFileOfNode(node) != file, "complete official export set changed")
					var instance *checker.Type
					var classSymbol *ast.Symbol
					if tc.category != "" {
						classNode := file.Statements.Nodes[0]
						reject(t, classNode.Kind != ast.KindClassDeclaration || classNode.Name().Text() != tc.className || len(classNode.Members()) != 0 || classNode.Modifiers() == nil || len(classNode.Modifiers().Nodes) != 1, "empty marked class control changed")
						decorator := classNode.Modifiers().Nodes[0]
						reject(t, decorator.Kind != ast.KindDecorator || decorator.AsDecorator().Expression.Text() != marker, "original compile-time marker changed")
						classSymbol = check.GetSymbolAtLocation(classNode.Name())
						instance = check.GetDeclaredTypeOfSymbol(classSymbol)
						static := check.GetTypeOfSymbol(classSymbol)
						ctors := check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
						reject(t, classSymbol == nil || classSymbol == fnSymbol || len(classSymbol.Declarations) != 1 || classSymbol.Declarations[0] != classNode || instance == nil || !instance.IsClass() || static == instance || len(check.GetSignaturesOfType(instance, checker.SignatureKindConstruct)) != 0 || len(ctors) != 1 || ctors[0].Declaration() != nil || len(ctors[0].Parameters()) != 0 || check.GetReturnTypeOfSignature(ctors[0]) != instance, "official nominal/static/implicit constructor identity changed")
						anchor("class", classNode)
						anchor("className", classNode.Name())
						anchor("marker", decorator)
						open := strings.Index(source[strings.Index(source, "class "):], "{") + strings.Index(source, "class ")
						token["classBody"] = throwBoundarySpanForTest(source, open, classNode.End())
						markerFile := program.GetSourceFile("/src/minimal-aggregates-v1.d.ts")
						var markerNode *ast.Node
						for _, decl := range markerFile.Statements.Nodes {
							if decl.Kind == ast.KindFunctionDeclaration && decl.Name().Text() == marker {
								markerNode = decl
							}
						}
						reject(t, markerNode == nil || check.GetSymbolAtLocation(decorator.AsDecorator().Expression) != check.GetSymbolAtLocation(markerNode.Name()), "decorator must bind the original marker declaration")
						mstart := markerNode.Pos()
						for mstart < markerNode.End() && strings.ContainsRune(" \t\r\n", rune(markerFile.Text()[mstart])) {
							mstart++
						}
						markerSpan := throwBoundarySpanForTest(markerFile.Text(), mstart, markerNode.End())
						markerRaw := throwBoundaryNodeForTest(t, markerFile, markerNode)
						reject(t, markerRaw.Start > markerSpan.Start || markerRaw.End != markerSpan.End, "marker declaration UTF16 identity changed")
						msum := sha256.Sum256([]byte(markerFile.Text()))
						digest := "sha256:" + hex.EncodeToString(msum[:])
						esum := sha256.Sum256([]byte("tsfg.marker\x00minimal-aggregates-v1\x00" + tc.category + "\x00" + digest))
						ctor := map[string]any{"id": "c1", "kind": "implicitDefault", "classRef": "a1", "signatureDeclaration": nil, "implementation": nil, "parameters": []any{}, "signature": map[string]any{"parameters": []any{}, "returnType": "a1"}, "resultType": "a1", "body": []any{}, "parameterInitializers": []any{}, "range": token["class"], "bodyRange": nil}
						declaration := map[string]any{"id": "a1", "name": tc.className, "symbol": "s1", "category": tc.category, "nominalKey": map[string]any{"sourcePath": r.RootFile, "declarationPath": []string{tc.className}, "category": tc.category}, "markerOrigin": map[string]any{"declarationPath": markerFile.FileName(), "declarationRange": markerSpan, "declarationSha256": digest, "entryId": "sha256:" + hex.EncodeToString(esum[:]), "version": "minimal-aggregates-v1", "contextName": "ClassDecoratorContext", "contextArity": 1, "mode": "standard", "range": token["marker"]}, "instanceType": "a1", "constructorType": "constructor:a1", "fields": []any{}, "constructors": []any{ctor}, "memberCoverage": []any{}, "range": token["class"], "bodyRange": token["classBody"], "noInheritance": true}
						expectedAggregate = map[string]any{"profile": "minimal-aggregates-v1", "declarations": []any{declaration}, "sourceCoverage": []any{map[string]any{"kind": "class", "name": tc.className, "symbol": "s1", "range": token["class"], "bodyRange": token["classBody"]}, map[string]any{"kind": "function", "name": "entry", "symbol": "s2", "range": raw["function"], "bodyRange": token["functionBody"]}}}
					}
					statements := fn.Body.AsBlock().Statements.Nodes
					var payload, construct, local *ast.Node
					if tc.mode == "write" && throwing {
						reject(t, len(statements) != 1 || statements[0].Kind != ast.KindTryStatement, "one original try required")
						attempt := statements[0].AsTryStatement()
						reject(t, attempt.CatchClause == nil || attempt.FinallyBlock != nil, "original ordinary catch required")
						clause := attempt.CatchClause.AsCatchClause()
						binding := clause.VariableDeclaration
						reject(t, binding == nil || binding.Name().Text() != "caught" || binding.AsVariableDeclaration().Type != nil, "unannotated original catch required")
						tryBody, catchBody := attempt.TryBlock.AsBlock().Statements.Nodes, clause.Block.AsBlock().Statements.Nodes
						reject(t, len(tryBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement || len(catchBody) != 1 || catchBody[0].Kind != ast.KindThrowStatement, "single payload per throw required")
						anchor("try", statements[0])
						anchor("tryBlock", attempt.TryBlock)
						anchor("tryThrow", tryBody[0])
						anchor("tryValue", tryBody[0].AsThrowStatement().Expression)
						anchor("catchClause", attempt.CatchClause)
						anchor("catchDeclaration", binding)
						anchor("catchName", binding.Name())
						anchor("catchBlock", clause.Block)
						anchor("terminal", catchBody[0])
						payload = catchBody[0].AsThrowStatement().Expression
						reject(t, payload.Kind != ast.KindParenthesizedExpression, "aggregate assignment result group required")
						write := payload.AsParenthesizedExpression().Expression
						reject(t, write.Kind != ast.KindBinaryExpression || write.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken, "one aggregate assignment expression required")
						target := write.AsBinaryExpression().Left
						construct = write.AsBinaryExpression().Right
						caughtSymbol := check.GetSymbolAtLocation(binding.Name())
						unknown := check.GetTypeOfSymbol(caughtSymbol)
						reject(t, caughtSymbol == nil || caughtSymbol == fnSymbol || caughtSymbol == classSymbol || len(caughtSymbol.Declarations) != 1 || caughtSymbol.Declarations[0] != binding || unknown.Flags() != checker.TypeFlagsUnknown || check.GetSymbolAtLocation(target) != caughtSymbol || check.GetTypeAtLocation(binding.Name()) != unknown || check.GetTypeAtLocation(target) != unknown || check.GetTypeAtLocation(write) != instance || check.GetTypeAtLocation(payload) != instance, "unknown aggregate Target/RHS/result official identity changed")
						anchor("write", write)
						anchor("target", target)
					} else {
						size := 1
						if tc.mode == "local" || tc.mode == "write" {
							size = 2
						}
						reject(t, len(statements) != size, "existing control/candidate statement count changed")
						terminal := statements[size-1]
						anchor("terminal", terminal)
						if throwing {
							if terminal.Kind != ast.KindThrowStatement {
								t.Fatal("one actual throw required")
							}
							payload = terminal.AsThrowStatement().Expression
						} else {
							if terminal.Kind != ast.KindReturnStatement {
								t.Fatal("existing return control required")
							}
							payload = terminal.AsReturnStatement().Expression
						}
						if size == 2 {
							reject(t, statements[0].Kind != ast.KindVariableStatement, "existing local control required")
							list := statements[0].AsVariableStatement().DeclarationList
							declarations := list.AsVariableDeclarationList().Declarations.Nodes
							reject(t, list.Flags&ast.NodeFlagsConst == 0 || len(declarations) != 1, "one immutable local required")
							local = declarations[0]
							construct = local.Initializer()
							localSymbol := check.GetSymbolAtLocation(local.Name())
							reject(t, local.Name().Text() != "payload" || local.AsVariableDeclaration().Type == nil || localSymbol == nil || localSymbol == classSymbol || len(localSymbol.Declarations) != 1 || localSymbol.Declarations[0] != local || check.GetTypeFromTypeNode(local.AsVariableDeclaration().Type) != instance || check.GetTypeOfSymbol(localSymbol) != instance, "local original aggregate type/identity changed")
							reject(t, throwing && (payload.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(payload) != localSymbol || check.GetTypeAtLocation(payload) != instance), "aggregate throw reference lost its original local")
							anchor("local", local)
							anchor("localName", local.Name())
						} else if tc.mode == "new" {
							construct = payload
						}
					}
					anchor("payload", payload)
					if construct != nil {
						reject(t, construct.Kind != ast.KindNewExpression || construct.AsNewExpression().Expression.Text() != tc.className || construct.AsNewExpression().TypeArguments != nil, "one actual direct construction required")
						call := construct.AsNewExpression()
						resolved := check.GetResolvedSignature(construct)
						reject(t, call.Arguments != nil && len(call.Arguments.Nodes) != 0 || check.GetSymbolAtLocation(call.Expression) != classSymbol || check.GetTypeAtLocation(construct) != instance || resolved == nil || resolved.Declaration() != nil || len(resolved.Parameters()) != 0 || check.GetReturnTypeOfSignature(resolved) != instance, "resolved new signature must return the same original instance")
						anchor("construct", construct)
					}
					expectedParams := 1
					if tc.mode == "new" {
						expectedParams = 0
					}
					reject(t, len(fn.Parameters.Nodes) != expectedParams || len(calls[0].Parameters()) != expectedParams, "official parameter count changed")
					if expectedParams == 1 {
						parameter := fn.Parameters.Nodes[0]
						ptype := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
						psymbol := check.GetSymbolAtLocation(parameter.Name())
						reject(t, psymbol == nil || psymbol == fnSymbol || psymbol == classSymbol || calls[0].Parameters()[0] != psymbol || check.GetTypeOfSymbol(psymbol) != ptype, "original parameter type/symbol changed")
						anchor("parameter", parameter)
						anchor("parameterName", parameter.Name())
						anchor("parameterAnnotation", parameter.AsParameterDeclaration().Type)
						use := payload
						if tc.mode == "write" && throwing {
							use = statements[0].AsTryStatement().TryBlock.AsBlock().Statements.Nodes[0].AsThrowStatement().Expression
						}
						if tc.mode == "reference" {
							payloadType, payloadSymbol := check.GetTypeAtLocation(payload), check.GetSymbolAtLocation(payload)
							t.Logf("TSFG_EXCEPTION_PAYLOAD_REFERENCE_IDENTITY_V1 kind=%s kindMatches=%t name=%q nameMatches=%t symbolMatches=%t typeMatches=%t returnTypeMatchesParameter=%t returnIdentityRequired=%t payloadType=%p parameterType=%p returnType=%p payloadSymbol=%p parameterSymbol=%p", payload.Kind.String(), payload.Kind == ast.KindIdentifier, payload.Text(), payload.Text() == "payload", payloadSymbol == psymbol, payloadType == ptype, returnType == ptype, tc.payload != "function", payloadType, ptype, returnType, payloadSymbol, psymbol)
							if tc.payload == "function" {
								// Official anonymous types are cached per annotation node. Each inline
								// source owns its signature; return legality also has four clean groups.
								invalidSignature := false
								for index, typ := range []*checker.Type{ptype, returnType} {
									origin := []*ast.Node{parameter.AsParameterDeclaration().Type, fn.Type}[index]
									sigs := check.GetSignaturesOfType(typ, checker.SignatureKindCall)
									t.Logf("TSFG_EXCEPTION_PAYLOAD_CALLABLE_SOURCE_V1 index=%d type=%p ownAnnotationType=%p ownTypeMatches=%t kind=%s rawStart=%d rawEnd=%d source64=%s signatureCount=%d", index, typ, check.GetTypeFromTypeNode(origin), check.GetTypeFromTypeNode(origin) == typ, origin.Kind.String(), origin.Pos(), origin.End(), base64.StdEncoding.EncodeToString([]byte(source[origin.Pos():origin.End()])), len(sigs))
									invalidSignature = invalidSignature || origin.Kind != ast.KindFunctionType || check.GetTypeFromTypeNode(origin) != typ || len(sigs) != 1
									for ordinal, resolved := range sigs {
										declaration := resolved.Declaration()
										declarationSource := []byte(nil)
										if declaration != nil {
											declarationSource = []byte(source[declaration.Pos():declaration.End()])
										}
										t.Logf("TSFG_EXCEPTION_PAYLOAD_CALLABLE_SIGNATURE_V1 origin=%d ordinal=%d signature=%p declaration=%p sourceNode=%p declarationMatches=%t declaration64=%s parameters=%d typeParameters=%d hasThis=%t hasRest=%t minArguments=%d returnType=%p boolType=%p returnsBool=%t", index, ordinal, resolved, declaration, origin, declaration == origin, base64.StdEncoding.EncodeToString(declarationSource), len(resolved.Parameters()), len(resolved.TypeParameters()), resolved.ThisParameter() != nil, resolved.HasRestParameter(), resolved.MinArgumentCount(), check.GetReturnTypeOfSignature(resolved), check.GetBooleanType(), check.GetReturnTypeOfSignature(resolved) == check.GetBooleanType())
										invalidSignature = invalidSignature || declaration != origin || len(resolved.Parameters()) != 0 || len(resolved.TypeParameters()) != 0 || resolved.ThisParameter() != nil || resolved.HasRestParameter() || resolved.MinArgumentCount() != 0 || check.GetReturnTypeOfSignature(resolved) != check.GetBooleanType()
									}
								}
								reject(t, invalidSignature, "each official callable annotation must own one zero-parameter bool signature")
							}
							reject(t, payload.Kind != ast.KindIdentifier || payload.Text() != "payload" || payloadSymbol != psymbol || payloadType != ptype || tc.payload != "function" && returnType != ptype, "reference payload must preserve original type, binding and return control")
						}
						aliasName := tc.payload
						if tc.mode != "reference" || tc.name == "null" || tc.name == "undefined" {
							aliasName = "i32"
						}
						if aliasName != "function" {
							var nominal *checker.Type
							for _, path := range r.Declarations.Paths {
								for _, decl := range program.GetSourceFile(path).Statements.Nodes {
									if decl.Kind == ast.KindTypeAliasDeclaration && decl.Name().Text() == aliasName {
										nominal = check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(decl.Name()))
									}
								}
							}
							reject(t, nominal == nil, "independent nominal declaration missing")
							if tc.name == "null" || tc.name == "undefined" {
								reject(t, !ptype.IsUnion() || len(ptype.Types()) != 2, "exact nominal/nullish union required")
								foundPayload, foundAbsence := false, false
								for _, member := range ptype.Types() {
									if member == nominal {
										foundPayload = true
									} else if tc.name == "null" && member.Flags()&checker.TypeFlagsNull != 0 || tc.name == "undefined" && member.Flags()&checker.TypeFlagsUndefined != 0 {
										foundAbsence = true
									} else {
										t.Fatal("foreign nullable union member")
									}
								}
								reject(t, !foundPayload || !foundAbsence, "nullable binding lost its exact original members")
							} else if ptype != nominal {
								t.Fatal("parameter must retain original nominal scalar object")
							}
							reject(t, tc.mode != "reference" && (returnType != nominal || (!throwing || tc.mode == "write") && (use == nil || use.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(use) != psymbol || check.GetTypeAtLocation(use) != nominal)), "existing scalar return control changed")
						}
					}
					reject(t, tc.mode == "new" && returnType != instance, "existing new return control must retain original instance")
					digest := sha256.Sum256([]byte(source))
					oracle, e := json.Marshal(map[string]any{"case": tc.name, "throwing": throwing, "sourceSha256": "sha256:" + hex.EncodeToString(digest[:]), "bytes": len(source), "units": len(utf16.Encode([]rune(source))), "raw": raw, "token": token, "payload": tc.payload, "category": tc.category, "exports": []map[string]any{{"name": "entry", "range": raw["function"], "definition": raw["definition"]}}})
					t.Logf("TSFG_EXCEPTION_PAYLOAD_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
					if e != nil {
						t.Fatal(e)
					}
				}) {
					return
				}
				t.Run("ownedProjection", func(t *testing.T) {
					sym := func(id, name, kind, typ string, writable bool, where span) map[string]any {
						return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
					}
					ref := func(typ, id, label string, aggregate bool) map[string]any {
						where := raw[label]
						if aggregate {
							where = token[label]
						}
						return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": where}
					}
					id, parameterID := "s1", "s2"
					symbols, parameters := []any{}, []any{}
					returns := tc.payload
					if tc.mode == "local" || tc.mode == "write" {
						returns = "i32"
					}
					if tc.category != "" {
						symbols = append(symbols, sym("s1", tc.className, "class", "a1", false, raw["className"]))
						id, parameterID = "s2", "s3"
					}
					fnSymbol := sym(id, "entry", "function", returns, false, raw["definition"])
					var returnPresence any
					leaf := map[string]any{"parameters": []any{}, "returnType": "bool"}
					if tc.name == "null" || tc.name == "undefined" {
						returnPresence = map[string]any{"undefined": tc.name == "undefined", "null": tc.name == "null", "range": raw["function"], "annotation": token["returnAnnotation"]}
						fnSymbol["returnPresence"] = returnPresence
					}
					symbols = append(symbols, fnSymbol)
					if tc.mode != "new" {
						name, typ := "payload", tc.payload
						if tc.mode != "reference" {
							name, typ = "value", "i32"
						}
						parameter := sym(parameterID, name, "parameter", typ, true, raw["parameterName"])
						if typ == "function" {
							parameter["signature"] = leaf
						} else {
							parameter["presence"] = map[string]any{"undefined": tc.name == "undefined", "null": tc.name == "null", "optional": false, "default": false, "range": raw["parameter"], "annotation": token["parameterAnnotation"]}
						}
						parameters, symbols = append(parameters, parameter), append(symbols, parameter)
					}
					construct := map[string]any{"kind": "construct", "type": "a1", "target": "c1", "arguments": []any{}, "signature": map[string]any{"parameters": []any{}, "returnType": "a1"}, "range": token["construct"]}
					var expression any = ref(tc.payload, parameterID, "payload", false)
					body := []any{}
					if tc.mode == "local" || tc.mode == "write" && !throwing {
						local := sym("s4", "payload", "local", "a1", false, raw["localName"])
						symbols = append(symbols, local)
						body = append(body, map[string]any{"kind": "variable", "symbol": local, "initializer": construct, "range": raw["local"]})
						expression = ref("i32", parameterID, "payload", false)
						if throwing {
							expression = ref("a1", "s4", "payload", true)
						}
					}
					if tc.mode == "new" {
						expression = construct
					}
					if tc.payload == "function" {
						expression.(map[string]any)["signature"] = leaf
					}
					if returnPresence != nil {
						expression.(map[string]any)["presenceFlow"] = map[string]any{"undefined": tc.name == "undefined", "null": tc.name == "null"}
					}
					kind := "return"
					if throwing {
						kind = "throw"
					}
					body = append(body, map[string]any{"kind": kind, "expression": expression, "range": raw["terminal"]})
					if tc.mode == "write" && throwing {
						caught := sym("s4", "caught", "local", "unknown", true, raw["catchName"])
						symbols = append(symbols, caught)
						write := map[string]any{"kind": "write", "type": "a1", "operator": "=", "range": token["write"], "left": ref("unknown", "s4", "target", true), "right": construct}
						group := map[string]any{"kind": "group", "type": "a1", "range": token["payload"], "operand": write}
						tryThrow := map[string]any{
							"kind":       "throw",
							"range":      raw["tryThrow"],
							"expression": ref("i32", parameterID, "tryValue", false),
						}
						tryBlock := map[string]any{
							"range": raw["tryBlock"],
							"body":  []any{tryThrow},
						}
						catchThrow := map[string]any{
							"kind":       "throw",
							"range":      raw["terminal"],
							"expression": group,
						}
						catchBlock := map[string]any{
							"range": raw["catchBlock"],
							"body":  []any{catchThrow},
						}
						catchClause := map[string]any{
							"binding":          caught,
							"declarationRange": raw["catchDeclaration"],
							"range":            raw["catchClause"],
							"block":            catchBlock,
						}
						exception := map[string]any{
							"tryBlock":    tryBlock,
							"catchClause": catchClause,
						}
						body = []any{map[string]any{
							"kind":      "try",
							"range":     raw["try"],
							"exception": exception,
						}}
					}
					fn := map[string]any{"name": "entry", "symbol": id, "parameters": parameters, "returnType": returns, "range": raw["function"], "body": body}
					if tc.mode != "new" && tc.payload != "function" {
						fn["parameterInitializers"] = []any{nil}
					}
					if returnPresence != nil {
						fn["returnPresence"] = returnPresence
					}
					if tc.payload == "function" {
						fn["returnSignature"] = leaf
					}
					expected := map[string]any{"status": "ok", "inputs": []string{r.RootFile, r.Declarations.Paths[0]}, "symbols": symbols, "functions": []any{fn}}
					if tc.category != "" {
						expected["inputs"] = []string{r.RootFile, "/src/minimal-aggregates-v1.d.ts", "/src/minimal-conversions-v5.d.ts"}
						expected["aggregateFacts"] = expectedAggregate
					}
					expectedBytes, e := json.Marshal(expected)
					t.Logf("TSFG_EXCEPTION_PAYLOAD_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
					if e != nil {
						t.Fatal(e)
					}
					var output bytes.Buffer
					commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
					responseBytes := output.Bytes()
					t.Logf("TSFG_EXCEPTION_PAYLOAD_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(responseBytes))
					if commandErr != nil {
						t.Fatalf("public command failed after independent official payload oracle: %v; raw=%s", commandErr, responseBytes)
					}
					var got, want any
					if e := json.Unmarshal(responseBytes, &got); e != nil {
						t.Fatalf("incomplete public JSON: %v; raw=%s", e, responseBytes)
					}
					if e := json.Unmarshal(expectedBytes, &want); e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("official payload/control oracle passed but complete owned identity facts differ: got=%s want=%s", responseBytes, expectedBytes)
					}
				})
			}
			if !t.Run("existingControl", func(t *testing.T) { run(t, false) }) {
				return
			}
			t.Run("throw", func(t *testing.T) { run(t, true) })
		})
	}
}
