// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"testing"
)

// This direct scalar case is separate from the eight original whole packets
// and the callable i32 case. The official recipe, not a producer output, fixes
// the only direct call. Its scalar header must retain required presence.
//
//go:embed testdata/module-callable-facts-v1/presence-direct-scenario.json
var mcPresenceDirectScenario23d []byte

func mcPresenceDirectCase23d(t *testing.T) mcCase23d {
	t.Helper()
	var fixture struct{ Case mcCase23d }
	if err := json.Unmarshal(mcPresenceDirectScenario23d, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Case.Name != "i32-direct-presence" || len(fixture.Case.Recipe) != 1 || len(fixture.Case.Recipe[0].Functions) != 2 {
		t.Fatal("complete independent direct presence recipe required")
	}
	use := fixture.Case.Recipe[0].Functions[1]
	if use.Name != "use" || len(use.Body) != 1 || use.Body[0].Kind != "return" || use.Body[0].Value.Kind != "call" || use.Body[0].Value.Target != "/src/app.ts#identity" {
		t.Fatal("independent direct call body required")
	}
	return fixture.Case
}

// Root freezes the complete canonical package emitted here before it runs the
// separate public producer comparison. No public frontend projection is called.
func TestModuleCallable23dIndependentDirectPresenceOracle(t *testing.T) {
	mcGenerateIndependent23d(t, mcPresenceDirectCase23d(t))
}

func TestModuleCallableFactsDirectScalarPresenceWhole(t *testing.T) {
	mcPresenceDirectCase23d(t)
	expected, err := mcPublicExpected23d.ReadFile("testdata/module-callable-facts-v1/expected/i32-direct-presence.json")
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Payload struct {
			Facts struct {
				Functions []struct {
					Name       string
					Parameters []struct{ Presence json.RawMessage }
					Body       []struct {
						Expression struct {
							Kind      string
							Signature struct{ Presence []json.RawMessage }
						}
					}
				}
			}
		}
	}
	if err := json.Unmarshal(expected, &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.Payload.Facts.Functions) != 2 {
		t.Fatal("complete independent source functions required")
	}
	identity, use := packet.Payload.Facts.Functions[0], packet.Payload.Facts.Functions[1]
	if identity.Name != "identity" || use.Name != "use" || len(identity.Parameters) != 1 || len(use.Body) != 1 || use.Body[0].Expression.Kind != "call" || len(use.Body[0].Expression.Signature.Presence) != 1 {
		t.Fatal("independent scalar direct signature presence omitted")
	}
	if !bytes.Equal(identity.Parameters[0].Presence, use.Body[0].Expression.Signature.Presence[0]) {
		t.Fatal("independent signature presence must retain actual callee ParameterDeclaration and TypeNode anchors")
	}
	mcPublicDirectedWhole23d(t, mcPresenceDirectScenario23d, "i32-direct-presence")
}
