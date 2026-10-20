// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate provenance binds actual options; it is not a formal TypeId or ABI.
package main

import "github.com/microsoft/typescript-go/internal/core"

const classSemanticProfile = "tsfg-owned-class-facts-candidate-v5"

func classOfficialOptions(resources *classResources, options *core.CompilerOptions) (classCompilerOptions, error) {
	if err := resources.ctx.Err(); err != nil {
		return classCompilerOptions{}, err
	}
	strict := classStrictOptions{
		StrictNullChecks:             options.GetStrictOptionValue(options.StrictNullChecks),
		StrictFunctionTypes:          options.GetStrictOptionValue(options.StrictFunctionTypes),
		StrictBindCallApply:          options.GetStrictOptionValue(options.StrictBindCallApply),
		StrictPropertyInitialization: options.GetStrictOptionValue(options.StrictPropertyInitialization),
		StrictBuiltinIteratorReturn:  options.GetStrictOptionValue(options.StrictBuiltinIteratorReturn),
		NoImplicitAny:                options.GetStrictOptionValue(options.NoImplicitAny),
		NoImplicitThis:               options.GetStrictOptionValue(options.NoImplicitThis),
		UseUnknownInCatchVariables:   options.GetStrictOptionValue(options.UseUnknownInCatchVariables),
	}
	if !strict.StrictNullChecks || !strict.StrictFunctionTypes || !strict.StrictBindCallApply ||
		!strict.StrictPropertyInitialization || !strict.StrictBuiltinIteratorReturn || !strict.NoImplicitAny ||
		!strict.NoImplicitThis || !strict.UseUnknownInCatchVariables {
		return classCompilerOptions{}, classError("input", "effective official strict checks differ from the candidate profile", classAnchor{})
	}
	// Marshal the actual official tagged CompilerOptions, including tagged
	// internal options and OrderedMap's official MarshalerTo. This is the raw
	// configuration identity, not a claim that all derived defaults are expanded.
	// No shallow Clone, reflection-based projection, or compiler pointer escapes.
	canonical, err := resources.canonicalJSON(options, "official options", classOptionsByteLimit, classAnchor{})
	if err != nil {
		return classCompilerOptions{}, err
	}
	return classCompilerOptions{Target: int32(options.Target), Module: int32(options.Module),
		UseDefineForClassFields: options.GetUseDefineForClassFields(), EffectiveStrict: strict,
		OfficialOptions: string(canonical), OfficialOptionsSHA256: classTextSHA(string(canonical))}, nil
}
