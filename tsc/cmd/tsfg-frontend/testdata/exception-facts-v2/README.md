<!-- SPDX-License-Identifier: Apache-2.0 -->
# Exception facts v2

`runFrontendCommand --exception-facts-io-v2` selects the explicit v2 request,
package, descriptor and identity domains. `descriptor.json` is complete canonical
JSON without a final newline. Its SHA-256 is checked by the producer before
decoding a v2 request. The descriptor embeds the inherited v1 DTO definitions;
it does not ask a reader to select a format from encountered expression nodes.
The independently selected public v1 protocol keeps schema `5221b508…`, its
original identity domains and vocabulary. Old package bytes are not migrated.
V2 additionally requires an explicit existing scalar target; the legacy
target-free minimal-scalars profile remains a v1-only input capability.

The v2-only `assertion` expression records an official static view in `type`,
its `as` or angle-assertion syntax, annotation/operator UTF16 ranges, and the
complete original `operand`. An unknown operand stays unknown in that child;
an assertion is neither a conversion nor proof of an actual i32 value.
References, aliases, returns and current writable catch bindings keep their
defining symbols and original use-site facts. No dynamic domain is inferred.

The `undefined` expression requires the official built-in symbol with no
declaration and actual undefined type. A same-spelled parameter/local remains
a reference to its own declaration. A current catch reference whose checker
type becomes undefined also remains that reference, rather than reverting to
the original throw or being replaced by a built-in token.

Scalar arithmetic reuses the existing R01 projector and original operand order.
These Source facts do not evaluate operands, insert numeric checks, request a
trap, or grant Compiler/wire/Runtime/VM/GC qualification. A later Compiler bridge
must preserve assertion operands and distinguish transport from numeric use.
Assertion views here are admitted scalar types or unknown; aggregate/callable
assertions and standalone null payloads retain their existing refusal boundary.

The independent tests use complete official Programs, checker types, actual
declaration/symbol joins, all functions/exports and per-file UTF16 identities.
The versioned package remains bounded and publishes only after whole-program
projection, export and canonical-identity validation succeeds.
