// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
)

type phaseASixOccurrenceForTest struct {
	Name                   string `json:"name"`
	Raw                    [2]int `json:"raw"`
	Flags                  uint64 `json:"flags"`
	SameParameter          bool   `json:"sameParameter"`
	SameCatch              bool   `json:"sameCatch"`
	IsParameterDeclaration bool   `json:"isParameterDeclaration"`
	IsCatchDeclaration     bool   `json:"isCatchDeclaration"`
}

// Independently prepared and frozen before command execution. Complete
// expected facts are written independently of every public/private response.
// Official anchor/type constants come from the separately qualified eleven-case
// discovery; this tracer rechecks all of them on fresh Programs before commands.
func TestExceptionFactsCommandV1PhaseASixPositiveControls(t *testing.T) {
	cases := []struct {
		name, source, functionName, parameterName, sha256 string
		bytes, utf16                                      int
		facts, anchors, occurrences                       string
	}{
		{name: "unicodeActualAstralControl", source: "// 😀\r\nexport function entry(𝐀: i32): i32 {\r\n  try {\r\n    throw 𝐀;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = 𝐀;\r\n  }\r\n}", functionName: "entry", parameterName: "𝐀", sha256: "5341ccf5ce2ffb31d50e35f5115403f141062001297acfed17186e3f392cd9ad", bytes: 169, utf16: 161,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"entry","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":28}},{"id":"s2","name":"𝐀","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":31},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":36},"annotation":{"start":33,"end":36}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":81,"end":87}},{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":135,"end":142}}],"functions":[{"name":"entry","symbol":"s1","parameters":[{"id":"s2","name":"𝐀","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":31},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":36},"annotation":{"start":33,"end":36}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":161},"body":[{"kind":"try","range":{"start":44,"end":158},"exception":{"tryBlock":{"range":{"start":51,"end":73},"body":[{"kind":"throw","range":{"start":53,"end":68},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":64,"end":67}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":81,"end":87}},"declarationRange":{"start":81,"end":87},"range":{"start":73,"end":114},"block":{"range":{"start":88,"end":114},"body":[{"kind":"throw","range":{"start":90,"end":109},"expression":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":101,"end":108}}}]}},"finallyBlock":{"range":{"start":122,"end":158},"body":[{"kind":"variable","range":{"start":135,"end":152},"symbol":{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":135,"end":142}},"initializer":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":149,"end":152}}}]}}}]}]}`,
			anchors:     `[[263,0,161,7],[94,0,13,7],[79,22,28,23],[170,29,36,29],[79,29,31,29],[184,32,36,33],[79,32,36,33],[184,38,42,39],[79,38,42,39],[242,42,161,43],[259,44,158,48],[242,51,73,52],[258,53,68,59],[79,64,67,65],[300,73,114,74],[261,81,87,81],[79,81,87,81],[242,88,114,89],[258,90,109,96],[79,101,108,102],[242,122,158,123],[244,124,153,130],[262,124,152,130],[261,135,152,136],[79,135,142,136],[184,143,147,144],[79,143,147,144],[79,149,152,150]]`,
			occurrences: `[{"name":"𝐀","raw":[29,31],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"𝐀","raw":[64,67],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[81,87],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[101,108],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"𝐀","raw":[149,152],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
		{name: "unicodeBracedEscapeControl", source: "// 😀\r\nexport function entry(\\u{1D400}: i32): i32 {\r\n  try {\r\n    throw \\u{1D400};\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = \\u{1D400};\r\n  }\r\n}", functionName: "entry", parameterName: "𝐀", sha256: "633012ca7981332354701117f858489b30016fbd609d0f38f0d43e387010935a", bytes: 184, utf16: 182,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"entry","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":28}},{"id":"s2","name":"𝐀","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":38},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":43},"annotation":{"start":40,"end":43}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":95,"end":101}},{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":149,"end":156}}],"functions":[{"name":"entry","symbol":"s1","parameters":[{"id":"s2","name":"𝐀","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":38},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":43},"annotation":{"start":40,"end":43}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":182},"body":[{"kind":"try","range":{"start":51,"end":179},"exception":{"tryBlock":{"range":{"start":58,"end":87},"body":[{"kind":"throw","range":{"start":60,"end":82},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":71,"end":81}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":95,"end":101}},"declarationRange":{"start":95,"end":101},"range":{"start":87,"end":128},"block":{"range":{"start":102,"end":128},"body":[{"kind":"throw","range":{"start":104,"end":123},"expression":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":115,"end":122}}}]}},"finallyBlock":{"range":{"start":136,"end":179},"body":[{"kind":"variable","range":{"start":149,"end":173},"symbol":{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":149,"end":156}},"initializer":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":163,"end":173}}}]}}}]}]}`,
			anchors:     `[[263,0,182,7],[94,0,13,7],[79,22,28,23],[170,29,43,29],[79,29,38,29],[184,39,43,40],[79,39,43,40],[184,45,49,46],[79,45,49,46],[242,49,182,50],[259,51,179,55],[242,58,87,59],[258,60,82,66],[79,71,81,72],[300,87,128,88],[261,95,101,95],[79,95,101,95],[242,102,128,103],[258,104,123,110],[79,115,122,116],[242,136,179,137],[244,138,174,144],[262,138,173,144],[261,149,173,150],[79,149,156,150],[184,157,161,158],[79,157,161,158],[79,163,173,164]]`,
			occurrences: `[{"name":"𝐀","raw":[29,38],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"𝐀","raw":[71,81],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[95,101],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[115,122],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"𝐀","raw":[163,173],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
		{name: "intrinsicUndefinedModuleControl", source: "// 😀\r\nexport function undefined(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", functionName: "undefined", parameterName: "value", sha256: "d4e623eb2aac62d696ae0c49b340032e8ddeba1bfe71bcae288aa6b971aa6e8f", bytes: 176, utf16: 174,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"undefined","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":32}},{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":33,"end":38},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":33,"end":43},"annotation":{"start":40,"end":43}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":91,"end":97}},{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":145,"end":152}}],"functions":[{"name":"undefined","symbol":"s1","parameters":[{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":33,"end":38},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":33,"end":43},"annotation":{"start":40,"end":43}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":174},"body":[{"kind":"try","range":{"start":51,"end":171},"exception":{"tryBlock":{"range":{"start":58,"end":83},"body":[{"kind":"throw","range":{"start":60,"end":78},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":71,"end":77}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":91,"end":97}},"declarationRange":{"start":91,"end":97},"range":{"start":83,"end":124},"block":{"range":{"start":98,"end":124},"body":[{"kind":"throw","range":{"start":100,"end":119},"expression":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":111,"end":118}}}]}},"finallyBlock":{"range":{"start":132,"end":171},"body":[{"kind":"variable","range":{"start":145,"end":165},"symbol":{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":145,"end":152}},"initializer":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":159,"end":165}}}]}}}]}]}`,
			anchors:     `[[263,0,174,7],[94,0,13,7],[79,22,32,23],[170,33,43,33],[79,33,38,33],[184,39,43,40],[79,39,43,40],[184,45,49,46],[79,45,49,46],[242,49,174,50],[259,51,171,55],[242,58,83,59],[258,60,78,66],[79,71,77,72],[300,83,124,84],[261,91,97,91],[79,91,97,91],[242,98,124,99],[258,100,119,106],[79,111,118,112],[242,132,171,133],[244,134,166,140],[262,134,165,140],[261,145,165,146],[79,145,152,146],[184,153,157,154],[79,153,157,154],[79,159,165,160]]`,
			occurrences: `[{"name":"value","raw":[33,38],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"value","raw":[71,77],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[91,97],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[111,118],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"value","raw":[159,165],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
		{name: "intrinsicGlobalThisModuleControl", source: "// 😀\r\nexport function globalThis(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", functionName: "globalThis", parameterName: "value", sha256: "42d8b129e874b3d86bbdf3550f34e0ed0fe9abe38afac1ea6358a602b0c3fde7", bytes: 177, utf16: 175,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"globalThis","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":33}},{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":34,"end":39},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":34,"end":44},"annotation":{"start":41,"end":44}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":92,"end":98}},{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":146,"end":153}}],"functions":[{"name":"globalThis","symbol":"s1","parameters":[{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":34,"end":39},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":34,"end":44},"annotation":{"start":41,"end":44}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":175},"body":[{"kind":"try","range":{"start":52,"end":172},"exception":{"tryBlock":{"range":{"start":59,"end":84},"body":[{"kind":"throw","range":{"start":61,"end":79},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":72,"end":78}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":92,"end":98}},"declarationRange":{"start":92,"end":98},"range":{"start":84,"end":125},"block":{"range":{"start":99,"end":125},"body":[{"kind":"throw","range":{"start":101,"end":120},"expression":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":112,"end":119}}}]}},"finallyBlock":{"range":{"start":133,"end":172},"body":[{"kind":"variable","range":{"start":146,"end":166},"symbol":{"id":"s4","name":"copied","kind":"local","type":"i32","writable":false,"range":{"start":146,"end":153}},"initializer":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":160,"end":166}}}]}}}]}]}`,
			anchors:     `[[263,0,175,7],[94,0,13,7],[79,22,33,23],[170,34,44,34],[79,34,39,34],[184,40,44,41],[79,40,44,41],[184,46,50,47],[79,46,50,47],[242,50,175,51],[259,52,172,56],[242,59,84,60],[258,61,79,67],[79,72,78,73],[300,84,125,85],[261,92,98,92],[79,92,98,92],[242,99,125,100],[258,101,120,107],[79,112,119,113],[242,133,172,134],[244,135,167,141],[262,135,166,141],[261,146,166,147],[79,146,153,147],[184,154,158,155],[79,154,158,155],[79,160,166,161]]`,
			occurrences: `[{"name":"value","raw":[34,39],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"value","raw":[72,78],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[92,98],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[112,119],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"value","raw":[160,166],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
		{name: "finallyGuardBeforeControl", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught !== \"boolean\") { throw value; }\r\n    try { } finally { }\r\n    throw caught;\r\n  }\r\n}", functionName: "entry", parameterName: "value", sha256: "b2ca25a821095218da697f602248e4ce0f56fa927c33728c12fbfbe15f6887c4", bytes: 197, utf16: 195,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"entry","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":28}},{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":34},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":39},"annotation":{"start":36,"end":39}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":79,"end":85}}],"functions":[{"name":"entry","symbol":"s1","parameters":[{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":34},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":39},"annotation":{"start":36,"end":39}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":195},"body":[{"kind":"try","range":{"start":47,"end":192},"exception":{"tryBlock":{"range":{"start":54,"end":71},"body":[{"kind":"throw","range":{"start":56,"end":69},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":62,"end":68}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":79,"end":85}},"declarationRange":{"start":79,"end":85},"range":{"start":71,"end":192},"block":{"range":{"start":86,"end":192},"body":[{"kind":"if","range":{"start":94,"end":143},"condition":{"kind":"type-test","type":"bool","range":{"start":98,"end":125},"operator":"!==","operand":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":104,"end":111}},"typeTest":{"tag":"boolean","typeofRange":{"start":98,"end":111},"tagRange":{"start":115,"end":125}}},"thenBody":[{"kind":"block","range":{"start":126,"end":143},"body":[{"kind":"throw","range":{"start":128,"end":141},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":134,"end":140}}}]}]},{"kind":"try","range":{"start":143,"end":168},"exception":{"tryBlock":{"range":{"start":152,"end":156},"body":[]},"finallyBlock":{"range":{"start":164,"end":168},"body":[]}}},{"kind":"throw","range":{"start":168,"end":187},"expression":{"kind":"reference","type":"bool","symbol":"s3","range":{"start":179,"end":186}}}]}}}}]}]}`,
			anchors:     `[[263,0,195,7],[94,0,13,7],[79,22,28,23],[170,29,39,29],[79,29,34,29],[184,35,39,36],[79,35,39,36],[184,41,45,42],[79,41,45,42],[242,45,195,46],[259,47,192,51],[242,54,71,55],[258,56,69,57],[79,62,68,63],[300,71,192,72],[261,79,85,79],[79,79,85,79],[242,86,192,87],[246,88,143,94],[227,98,125,98],[222,98,111,98],[79,104,111,105],[37,111,115,112],[10,115,125,116],[242,126,143,127],[258,128,141,129],[79,134,140,135],[259,143,168,149],[242,152,156,153],[242,164,168,165],[258,168,187,174],[79,179,186,180]]`,
			occurrences: `[{"name":"value","raw":[29,34],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"value","raw":[62,68],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[79,85],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[104,111],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"value","raw":[134,140],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[179,186],"flags":134217984,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
		{name: "finallyGuardInsideCandidate", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    try {\r\n      if (typeof caught !== \"boolean\") { throw value; }\r\n    } finally { }\r\n    throw caught;\r\n  }\r\n}", functionName: "entry", parameterName: "value", sha256: "be7ebb8fc5f6eca9cd44166dc4ee86adf60e5c6885ea5357b999df165945d47c", bytes: 204, utf16: 202,
			facts:       `{"status":"ok","inputs":["/src/main.ts","/src/minimal-integers-v3.d.ts"],"symbols":[{"id":"s1","name":"entry","kind":"function","type":"i32","writable":false,"range":{"start":22,"end":28}},{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":34},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":39},"annotation":{"start":36,"end":39}}},{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":79,"end":85}}],"functions":[{"name":"entry","symbol":"s1","parameters":[{"id":"s2","name":"value","kind":"parameter","type":"i32","writable":true,"range":{"start":29,"end":34},"presence":{"undefined":false,"null":false,"optional":false,"default":false,"range":{"start":29,"end":39},"annotation":{"start":36,"end":39}}}],"parameterInitializers":[null],"returnType":"i32","range":{"start":0,"end":202},"body":[{"kind":"try","range":{"start":47,"end":199},"exception":{"tryBlock":{"range":{"start":54,"end":71},"body":[{"kind":"throw","range":{"start":56,"end":69},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":62,"end":68}}}]},"catchClause":{"binding":{"id":"s3","name":"caught","kind":"local","type":"unknown","writable":true,"range":{"start":79,"end":85}},"declarationRange":{"start":79,"end":85},"range":{"start":71,"end":199},"block":{"range":{"start":86,"end":199},"body":[{"kind":"try","range":{"start":88,"end":175},"exception":{"tryBlock":{"range":{"start":97,"end":163},"body":[{"kind":"if","range":{"start":107,"end":156},"condition":{"kind":"type-test","type":"bool","range":{"start":111,"end":138},"operator":"!==","operand":{"kind":"reference","type":"unknown","symbol":"s3","range":{"start":117,"end":124}},"typeTest":{"tag":"boolean","typeofRange":{"start":111,"end":124},"tagRange":{"start":128,"end":138}}},"thenBody":[{"kind":"block","range":{"start":139,"end":156},"body":[{"kind":"throw","range":{"start":141,"end":154},"expression":{"kind":"reference","type":"i32","symbol":"s2","range":{"start":147,"end":153}}}]}]}]},"finallyBlock":{"range":{"start":171,"end":175},"body":[]}}},{"kind":"throw","range":{"start":175,"end":194},"expression":{"kind":"reference","type":"bool","symbol":"s3","range":{"start":186,"end":193}}}]}}}}]}]}`,
			anchors:     `[[263,0,202,7],[94,0,13,7],[79,22,28,23],[170,29,39,29],[79,29,34,29],[184,35,39,36],[79,35,39,36],[184,41,45,42],[79,41,45,42],[242,45,202,46],[259,47,199,51],[242,54,71,55],[258,56,69,57],[79,62,68,63],[300,71,199,72],[261,79,85,79],[79,79,85,79],[242,86,199,87],[259,88,175,94],[242,97,163,98],[246,99,156,107],[227,111,138,111],[222,111,124,111],[79,117,124,118],[37,124,128,125],[10,128,138,129],[242,139,156,140],[258,141,154,142],[79,147,153,148],[242,171,175,172],[258,175,194,181],[79,186,193,187]]`,
			occurrences: `[{"name":"value","raw":[29,34],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":true,"isCatchDeclaration":false},{"name":"value","raw":[62,68],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[79,85],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":true},{"name":"caught","raw":[117,124],"flags":2,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"value","raw":[147,153],"flags":268435456,"sameParameter":true,"sameCatch":false,"isParameterDeclaration":false,"isCatchDeclaration":false},{"name":"caught","raw":[186,193],"flags":134217984,"sameParameter":false,"sameCatch":true,"isParameterDeclaration":false,"isCatchDeclaration":false}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.source
			sum := sha256.Sum256([]byte(source))
			if len(source) >= 1024 || len(source) != tc.bytes || len(utf16.Encode([]rune(source))) != tc.utf16 || hex.EncodeToString(sum[:]) != tc.sha256 {
				t.Fatal("frozen literal identity changed")
			}
			r := integerRequest(source)
			requestBytes, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				after, err := json.Marshal(r)
				if err != nil || !bytes.Equal(requestBytes, after) {
					t.Fatal("command/oracle mutated original request")
				}
			}()
			var expectedFacts map[string]any
			var wantAnchors [][4]int
			var wantOccurrences []phaseASixOccurrenceForTest
			for _, item := range []struct {
				raw    string
				target any
			}{{tc.facts, &expectedFacts}, {tc.anchors, &wantAnchors}, {tc.occurrences, &wantOccurrences}} {
				if err := json.Unmarshal([]byte(item.raw), item.target); err != nil {
					t.Fatal(err)
				}
			}
			program := throwBoundaryProgramForTest(t, r)
			if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
				return
			}
			if !t.Run("officialAnchorsSymbolsTypesAndCompleteExports", func(t *testing.T) {
				file := program.GetSourceFile(r.RootFile)
				if file == nil || file.Text() != source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 1 {
					t.Fatal("original external function source missing")
				}
				consumed := []string{}
				for _, file := range program.GetSourceFiles() {
					consumed = append(consumed, file.FileName())
				}
				slices.Sort(consumed)
				if !slices.Equal(consumed, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
					t.Fatal("complete consumed source closure differs")
				}
				fnNode := file.Statements.Nodes[0]
				if fnNode.Kind != ast.KindFunctionDeclaration {
					t.Fatal("function AST required")
				}
				fn := fnNode.AsFunctionDeclaration()
				if fn.Name() == nil || fn.Name().Text() != tc.functionName || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Type == nil {
					t.Fatal("signature syntax differs")
				}
				parameter := fn.Parameters.Nodes[0]
				if parameter.Name() == nil || parameter.Name().Text() != tc.parameterName || parameter.AsParameterDeclaration().Type == nil {
					t.Fatal("decoded parameter declaration differs")
				}
				body := fn.Body.AsBlock().Statements.Nodes
				if len(body) != 1 || body[0].Kind != ast.KindTryStatement || body[0].AsTryStatement().CatchClause == nil {
					t.Fatal("one original outer catch required")
				}
				caught := body[0].AsTryStatement().CatchClause.AsCatchClause().VariableDeclaration
				if caught == nil || caught.Name().Text() != "caught" || caught.AsVariableDeclaration().Type != nil {
					t.Fatal("unannotated catch binding required")
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				fnSymbol, parameterSymbol, catchSymbol := check.GetSymbolAtLocation(fn.Name()), check.GetSymbolAtLocation(parameter.Name()), check.GetSymbolAtLocation(caught.Name())
				declarationNodes := []*ast.Node{fnNode, parameter, caught}
				declarationSymbols := []*ast.Symbol{fnSymbol, parameterSymbol, catchSymbol}
				seen := map[*ast.Symbol]bool{}
				for i, symbol := range declarationSymbols {
					if symbol == nil || seen[symbol] || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declarationNodes[i] || ast.GetSourceFileOfNode(symbol.Declarations[0]) != file {
						t.Fatal("actual owned declaration/symbol identity differs")
					}
					seen[symbol] = true
				}
				i32 := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
				if i32 == nil || i32.Alias() == nil || i32.Alias().Symbol().Name != "i32" || check.GetTypeAtLocation(parameter.Name()) != i32 || check.GetTypeFromTypeNode(fn.Type) != i32 {
					t.Fatal("same nominal i32 signature required")
				}
				unknown := check.GetTypeAtLocation(caught.Name())
				if unknown == nil || unknown.Flags() != checker.TypeFlagsUnknown || check.GetTypeOfSymbol(catchSymbol) != unknown {
					t.Fatal("catch declaration must stay unknown")
				}
				signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(fnSymbol), checker.SignatureKindCall)
				if len(signatures) != 1 || signatures[0].Declaration() != fnNode || len(signatures[0].Parameters()) != 1 || signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != i32 {
					t.Fatal("official full signature identity differs")
				}
				exports := check.GetExportsOfModule(file.AsNode().Symbol())
				if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != tc.functionName || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode {
					t.Fatal("complete official export table differs")
				}
				gotAnchors, gotOccurrences := [][4]int{}, []phaseASixOccurrenceForTest{}
				localCount := 0
				position := func(start, end int) [2]int {
					if start < 0 || end < start || end > len(source) {
						t.Fatal("invalid original AST endpoint")
					}
					pair := [2]int{len(utf16.Encode([]rune(source[:start]))), len(utf16.Encode([]rune(source[:end])))}
					official := file.GetPositionMap()
					if pair != ([2]int{official.UTF8ToUTF16(start), official.UTF8ToUTF16(end)}) {
						t.Fatal("independent UTF16 conversion differs")
					}
					return pair
				}
				var visit func(*ast.Node) bool
				visit = func(node *ast.Node) bool {
					if node == nil {
						return false
					}
					raw, trim := position(node.Pos(), node.End()), position(scanner.SkipTrivia(source, node.Pos()), node.End())
					gotAnchors = append(gotAnchors, [4]int{int(node.Kind), raw[0], raw[1], trim[0]})
					if node.Kind == ast.KindIdentifier && (node.Text() == tc.parameterName || node.Text() == "caught") {
						symbol, typ := check.GetSymbolAtLocation(node), check.GetTypeAtLocation(node)
						if symbol == nil || typ == nil || symbol == parameterSymbol && typ != i32 {
							t.Fatal("reference lost actual nominal payload/symbol")
						}
						gotOccurrences = append(gotOccurrences, phaseASixOccurrenceForTest{Name: node.Text(), Raw: raw, Flags: uint64(typ.Flags()), SameParameter: symbol == parameterSymbol, SameCatch: symbol == catchSymbol, IsParameterDeclaration: node == parameter.Name(), IsCatchDeclaration: node == caught.Name()})
					}
					if node.Kind == ast.KindVariableDeclaration && node.Name() != nil && node.Name().Text() == "copied" {
						localCount++
						local := node.AsVariableDeclaration()
						symbol := check.GetSymbolAtLocation(node.Name())
						if symbol == nil || seen[symbol] || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || local.Initializer == nil || local.Type == nil || check.GetTypeAtLocation(node.Name()) != i32 || check.GetTypeFromTypeNode(local.Type) != i32 || check.GetTypeAtLocation(local.Initializer) != i32 || check.GetSymbolAtLocation(local.Initializer) != parameterSymbol {
							t.Fatal("finally local declaration/type/initializer identity differs")
						}
						seen[symbol] = true
					}
					node.ForEachChild(visit)
					return false
				}
				visit(fnNode)
				if !reflect.DeepEqual(gotAnchors, wantAnchors) {
					t.Fatalf("all official raw/trim anchors differ: got=%v want=%v", gotAnchors, wantAnchors)
				}
				if !reflect.DeepEqual(gotOccurrences, wantOccurrences) {
					t.Fatalf("all measured same-symbol occurrence types differ: got=%+v want=%+v", gotOccurrences, wantOccurrences)
				}
				wantLocal := 0
				if len(expectedFacts["symbols"].([]any)) == 4 {
					wantLocal = 1
				}
				if localCount != wantLocal {
					t.Fatal("finally local ownership count differs")
				}
			}) {
				return
			}
			var publicResponse []byte
			if !t.Run("completePublicIndependentControl", func(t *testing.T) {
				var output bytes.Buffer
				if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(requestBytes), &output); err != nil {
					t.Fatalf("public command failed: %v raw=%s", err, output.Bytes())
				}
				publicResponse = bytes.Clone(output.Bytes())
				if !bytes.HasSuffix(publicResponse, []byte("\n")) {
					t.Fatal("existing public framing changed")
				}
				var got any
				if err := json.Unmarshal(publicResponse, &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, expectedFacts) {
					t.Fatalf("complete independent public facts differ: got=%s want=%s", publicResponse, tc.facts)
				}
			}) {
				return
			}
			canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
			digest := func(value []byte) string { sum := sha256.Sum256(value); return "sha256:" + hex.EncodeToString(sum[:]) }
			const schema = "sha256:5221b50822965c633418462b28a9b58bfbc41b01e0fa4b6d31c50ab83805424d"
			var normalizedInput map[string]any
			if err := json.Unmarshal(requestBytes, &normalizedInput); err != nil {
				t.Fatal(err)
			}
			files := slices.Clone(r.Files)
			slices.SortFunc(files, func(a, b inputFile) int {
				if a.Path < b.Path {
					return -1
				}
				if a.Path > b.Path {
					return 1
				}
				return 0
			})
			identityFiles, manifest := []map[string]any{}, []map[string]any{}
			for _, file := range files {
				identityFiles = append(identityFiles, map[string]any{"path": file.Path, "content": file.Content})
				manifest = append(manifest, map[string]any{"path": file.Path, "digest": digest([]byte(file.Content)), "byteLength": len(file.Content), "length": len(utf16.Encode([]rune(file.Content))), "positionUnit": "utf16"})
			}
			if len(files) != 2 || files[0].Path != "/src/main.ts" || files[1].Path != "/src/minimal-integers-v3.d.ts" || digest([]byte(files[1].Content)) != "sha256:9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e" {
				t.Fatal("complete pinned declaration profile differs")
			}
			normalizedInput["files"] = identityFiles
			inputIdentity := digest(canonical(normalizedInput))
			privateRequest := canonical(map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": schema, "input": r})
			requestID := digest(append([]byte("tsfg.exception-facts.request.v1\x00"), privateRequest...))
			exports := []any{map[string]any{"sourcePath": r.RootFile, "external": true, "bindings": []any{map[string]any{"name": tc.functionName, "target": "s1", "sourcePath": r.RootFile, "range": span{0, tc.utf16}, "definition": span{wantAnchors[2][1], wantAnchors[2][2]}}}}}
			payload := map[string]any{"inputIdentity": inputIdentity, "files": manifest, "facts": expectedFacts, "exports": exports}
			packageID := digest(append([]byte("tsfg.exception-facts.package.v1\x00"+schema+"\x00"), canonical(payload)...))
			expectedPackage := canonical(map[string]any{"family": "tsfg.exception-facts-package-candidate", "version": 1, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload})
			t.Run("actualPrivateCompleteIndependentPacket", func(t *testing.T) {
				before := bytes.Clone(privateRequest)
				var output bytes.Buffer
				err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(privateRequest), &output)
				raw := output.Bytes()
				record := map[string]any{"name": tc.name, "source": source, "sourceSha256": tc.sha256, "publicRequestBase64": base64.StdEncoding.EncodeToString(requestBytes), "publicResponseBase64": base64.StdEncoding.EncodeToString(publicResponse), "privateRequestBase64": base64.StdEncoding.EncodeToString(privateRequest), "privateExpectedBase64": base64.StdEncoding.EncodeToString(expectedPackage), "privateResponseBase64": base64.StdEncoding.EncodeToString(raw)}
				encoded, marshalErr := json.Marshal(record)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				t.Logf("TSFG_EXCEPTION_PHASE_A_ACTUAL_POSITIVE_V1 %s", encoded)
				if err != nil {
					t.Fatalf("private command rejected clean actual source: %v raw=%s", err, raw)
				}
				if !bytes.Equal(privateRequest, before) {
					t.Fatal("private input bytes mutated")
				}
				if !bytes.Equal(raw, expectedPackage) {
					t.Fatalf("complete private frame differs from frozen independent expectation: got=%s want=%s", raw, expectedPackage)
				}
			})
		})
	}
}
