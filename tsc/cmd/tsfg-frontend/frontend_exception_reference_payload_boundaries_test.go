// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// These four original boundaries share one reference contract. The return
// controls remain in the caller; each throw owns a fresh official Program and
// complete independent JSON, including declaration and occurrence presence.
func throwReferencePayloadForTest(t *testing.T, source string, r request, annotation string) {
	t.Helper()
	t.Logf("TSFG_EXCEPTION_REFERENCE_PAYLOAD_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
	requestBytes, err := json.Marshal(r)
	t.Logf("TSFG_EXCEPTION_REFERENCE_PAYLOAD_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	wantType, wantNull, wantUndefined := "", false, false
	switch annotation {
	case "bool", "i16":
		wantType = annotation
	case "i32 | null":
		wantType, wantNull = "i32", true
	case "i32 | undefined":
		wantType, wantUndefined = "i32", true
	default:
		t.Fatal("only the four original finite reference sources belong to this boundary")
	}
	if source != "/* 😀 */ export function fail(value: "+annotation+"): "+annotation+" { throw value; }" {
		t.Fatal("original finite reference throw source changed")
	}
	defer func() {
		after, marshalErr := json.Marshal(r)
		if marshalErr != nil || !bytes.Equal(requestBytes, after) {
			t.Fatal("official/public reference throw checks changed the original request")
		}
	}()
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
		return
	}
	anchors, tokens := map[string]span{}, map[string]span{}
	payloadType, hasNull, hasUndefined := "", false, false
	if !t.Run("officialReferenceUnionNominalIdentityUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 {
			t.Fatal("official original reference source/function table changed")
		}
		fnNode := file.Statements.Nodes[0]
		if fnNode.Kind != ast.KindFunctionDeclaration || fnNode.Name() == nil || fnNode.Name().Text() != "fail" {
			t.Fatal("official fail declaration changed")
		}
		fn := fnNode.AsFunctionDeclaration()
		if fn.Type == nil || fn.TypeParameters != nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
			t.Fatal("official reference function shape changed")
		}
		parameter := fn.Parameters.Nodes[0]
		decl := parameter.AsParameterDeclaration()
		body := fn.Body.AsBlock().Statements.Nodes
		if parameter.Name() == nil || parameter.Name().Text() != "value" || decl.Type == nil || decl.QuestionToken != nil || decl.Initializer != nil || decl.DotDotDotToken != nil || len(body) != 1 || body[0].Kind != ast.KindThrowStatement {
			t.Fatal("official required parameter or original single throw changed")
		}
		payload := body[0].AsThrowStatement().Expression
		if payload == nil || payload.Kind != ast.KindIdentifier || payload.Text() != "value" {
			t.Fatal("official throw must retain one original reference")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		fnSymbol := check.GetSymbolAtLocation(fnNode.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
		declared := check.GetTypeFromTypeNode(decl.Type)
		if fnSymbol == nil || parameterSymbol == nil || fnSymbol == parameterSymbol || len(fnSymbol.Declarations) != 1 || fnSymbol.Declarations[0] != fnNode || len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameter || check.GetSymbolAtLocation(payload) != parameterSymbol {
			t.Fatal("official throw lost its distinct original parameter declaration/use identity")
		}
		if declared == nil || check.GetTypeOfSymbol(parameterSymbol) != declared || check.GetTypeAtLocation(parameter.Name()) != declared || check.GetTypeAtLocation(payload) != declared || check.GetTypeFromTypeNode(fn.Type) != declared {
			t.Fatal("official declaration and original reference must retain the same complete type object")
		}
		// Query each actual official union member. This deliberately avoids the
		// producer's finitePresence/scalar and never types a throw from fn.Type.
		var nominal *checker.Type
		if wantNull || wantUndefined {
			if !declared.IsUnion() || len(declared.Types()) != 2 {
				t.Fatal("original nullable annotation must retain exactly two official members")
			}
			for _, member := range declared.Types() {
				switch {
				case member.Flags()&checker.TypeFlagsNull != 0:
					if hasNull {
						t.Fatal("duplicate official null member")
					}
					hasNull = true
				case member.Flags()&checker.TypeFlagsUndefined != 0:
					if hasUndefined {
						t.Fatal("duplicate official undefined member")
					}
					hasUndefined = true
				default:
					if nominal != nil {
						t.Fatal("multiple official nominal payload members")
					}
					nominal = member
				}
			}
		} else if annotation == "bool" {
			if !declared.IsBooleanLike() {
				t.Fatal("original bool annotation lost official boolean identity")
			}
			payloadType = "bool"
		} else {
			if declared.IsUnion() {
				t.Fatal("original i16 must remain one nominal payload")
			}
			nominal = declared
		}
		if nominal != nil {
			if nominal.Alias() == nil || nominal.Alias().Symbol() == nil {
				t.Fatal("official payload lost its actual nominal alias")
			}
			alias := nominal.Alias().Symbol()
			payloadType = alias.Name
			if payloadType != wantType || len(alias.Declarations) != 1 || alias.Declarations[0].Kind != ast.KindTypeAliasDeclaration || ast.GetSourceFileOfNode(alias.Declarations[0]) != program.GetSourceFile("/src/minimal-integers-v3.d.ts") || alias.Declarations[0].Name() == nil || alias.Declarations[0].Name().Text() != wantType {
				t.Fatal("nominal payload alias/declaration provenance changed")
			}
		}
		if payloadType != wantType || hasNull != wantNull || hasUndefined != wantUndefined {
			t.Fatal("official payload members differ from the unchanged original annotation")
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(fnSymbol), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != fnNode || len(signatures[0].Parameters()) != 1 || signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != declared {
			t.Fatal("complete official signature differs from original declaration identities")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "fail" || exports[0].Flags&ast.SymbolFlagsAlias != 0 || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official reference exports changed")
		}
		remember := func(label string, node *ast.Node, spelling string, at int) {
			t.Helper()
			if at < 0 || at > len(source) || !strings.HasPrefix(source[at:], spelling) {
				t.Fatalf("original reference preimage missing for %s", label)
			}
			raw := throwBoundaryNodeForTest(t, file, node)
			token := throwBoundarySpanForTest(source, at, at+len(spelling))
			if node.Pos() > at || node.End() != at+len(spelling) || raw.Start > token.Start || raw.End != token.End {
				t.Fatalf("independent official raw/token reference anchor differs for %s", label)
			}
			anchors[label], tokens[label] = raw, token
		}
		functionStart := strings.Index(source, "export function")
		remember("function", fnNode, source[functionStart:], functionStart)
		remember("definition", fnNode.Name(), "fail", strings.Index(source, "fail("))
		parameterStart := strings.Index(source, "value: "+annotation)
		remember("parameter", parameter, "value: "+annotation, parameterStart)
		remember("parameterName", parameter.Name(), "value", parameterStart)
		remember("parameterAnnotation", decl.Type, annotation, parameterStart+len("value: "))
		returnStart := strings.Index(source, "): "+annotation) + len("): ")
		remember("returnAnnotation", fn.Type, annotation, returnStart)
		throwStart := strings.Index(source, "throw value;")
		remember("throw", body[0], "throw value;", throwStart)
		remember("payload", payload, "value", throwStart+len("throw "))
		if anchors["function"].Start != 0 || tokens["function"].Start != 9 {
			t.Fatal("original comment/non-BMP source prefix changed")
		}
		oracle, marshalErr := json.Marshal(map[string]any{"kind": "test-only-official-reference-payload-oracle", "sourcePath": file.FileName(), "raw": anchors, "token": tokens, "payload": payloadType, "null": hasNull, "undefined": hasUndefined, "exports": []string{"fail"}, "identity": "declared-parameter-type == original-throw-reference-type; same-defining-symbol"})
		t.Logf("TSFG_EXCEPTION_REFERENCE_PAYLOAD_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
	}) {
		return
	}
	parameter := map[string]any{"id": "s2", "name": "value", "kind": "parameter", "type": payloadType, "writable": true,
		"range": anchors["parameterName"], "presence": map[string]any{
			"undefined": hasUndefined, "null": hasNull, "optional": false, "default": false,
			"range": anchors["parameter"], "annotation": tokens["parameterAnnotation"]}}
	definition := map[string]any{"id": "s1", "name": "fail", "kind": "function", "type": payloadType, "writable": false, "range": anchors["definition"]}
	value := map[string]any{"kind": "reference", "type": payloadType, "symbol": "s2", "range": anchors["payload"]}
	fn := map[string]any{"name": "fail", "symbol": "s1", "returnType": payloadType, "parameters": []any{parameter},
		"parameterInitializers": []any{nil}, "range": anchors["function"], "body": []any{map[string]any{"kind": "throw", "range": anchors["throw"], "expression": value}}}
	if hasNull || hasUndefined {
		declared := map[string]any{"undefined": hasUndefined, "null": hasNull, "range": anchors["function"], "annotation": tokens["returnAnnotation"]}
		definition["returnPresence"], fn["returnPresence"] = declared, declared
		value["presenceFlow"] = map[string]any{"undefined": hasUndefined, "null": hasNull}
	}
	expected := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": []any{definition, parameter}, "functions": []any{fn}}
	expectedBytes, err := json.Marshal(expected)
	t.Logf("TSFG_EXCEPTION_REFERENCE_PAYLOAD_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("ownedProjection", func(t *testing.T) {
		var output bytes.Buffer
		commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(requestBytes), &output)
		raw := output.Bytes()
		t.Logf("TSFG_EXCEPTION_REFERENCE_PAYLOAD_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
		if commandErr != nil {
			t.Fatalf("public command failed after official oracle: %v; raw=%s", commandErr, raw)
		}
		if !bytes.HasSuffix(raw, []byte("\n")) {
			t.Fatal("public reference response lost final newline")
		}
		var got, want any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("public reference throw response is not JSON: %v; raw=%s", err, raw)
		}
		if err := json.Unmarshal(expectedBytes, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("official reference oracle passed; complete public throw differs: got=%s want=%s", raw, expectedBytes)
		}
	})
}
