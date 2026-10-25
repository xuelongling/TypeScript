// SPDX-License-Identifier: Apache-2.0
// Module storage and roots join the same original symbols as typed bodies.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
)

func commonProgramAccess(node *ast.Node) string {
	if node.ModifierFlags()&ast.ModifierFlagsPrivate != 0 {
		return "private"
	}
	if node.ModifierFlags()&ast.ModifierFlagsProtected != 0 {
		return "protected"
	}
	return "public"
}

func (p *arrayGenericProjection) commonOriginalSymbol(symbol *ast.Symbol) *ast.Symbol {
	if symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
		return p.check.GetAliasedSymbol(symbol)
	}
	return symbol
}
func (p *arrayGenericProjection) seedCommonGlobals(state *arrayGenericState) error {
	if p.common == nil || state.commonSeeded {
		return nil
	}
	for symbol, binding := range p.common.globals {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if state.bindings == nil {
			state.bindings = map[*ast.Symbol]arrayFactsBinding{}
		}
		state.bindings[symbol] = binding
	}
	state.commonSeeded = true
	return nil
}
func (p *arrayGenericProjection) commonHeaders(files []*ast.SourceFile) error {
	c := p.common
	c.globals, c.declarations = map[*ast.Symbol]arrayFactsBinding{}, map[*ast.Node]arrayFactsBinding{}
	c.globalNodes = map[*ast.Symbol]*ast.Node{}
	c.globalRows, c.modules = []commonProgramGlobal{}, []commonProgramModule{}
	for _, file := range files {
		if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil {
			return p.fail(file.AsNode(), "common Program requires actual module symbols")
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindVariableStatement {
				continue
			}
			list := node.AsVariableStatement().DeclarationList
			if list.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
				return p.fail(node, "module binding requires original let or const")
			}
			kind := "let"
			if list.Flags&ast.NodeFlagsConst != 0 {
				kind = "const"
			}
			for _, declaration := range list.AsVariableDeclarationList().Declarations.Nodes {
				if err := p.annotationWork(); err != nil {
					return err
				}
				decl := declaration.AsVariableDeclaration()
				if decl.Initializer == nil || decl.Type == nil {
					return p.fail(declaration, "common global requires its initialized concrete annotation")
				}
				binding, err := p.binding(declaration, decl.Type, "global", kind == "let")
				if err != nil {
					return err
				}
				symbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(declaration.Name()))
				if symbol == nil {
					return p.fail(declaration, "global has no original symbol")
				}
				c.globals[symbol], c.declarations[declaration] = binding, binding
				c.globalNodes[symbol] = declaration
				c.globalRows = append(c.globalRows, commonProgramGlobal{file.FileName(), kind, binding})
			}
		}
	}
	return nil
}
func (p *arrayGenericProjection) commonTarget(symbol *ast.Symbol) (string, string, *ast.Node) {
	symbol = p.commonOriginalSymbol(symbol)
	if header := p.headers[symbol]; header != nil {
		return header.function.ID, "function", header.node
	}
	if binding, ok := p.common.globals[symbol]; ok {
		return binding.ID, "global", p.common.globalNodes[symbol]
	}
	if id := p.classes[symbol]; id != "" {
		return id, "type", p.classNodes[id]
	}
	return "", "", nil
}
func (p *arrayGenericProjection) commonModules(files []*ast.SourceFile) error {
	byPath := map[string]*ast.SourceFile{}
	for _, file := range files {
		byPath[file.FileName()] = file
		module := commonProgramModule{SourcePath: file.FileName(), External: true,
			Imports: []commonProgramImport{}, Exports: []commonProgramExport{}, Globals: []string{}, Initialization: []arrayGenericStatement{}}
		for _, exported := range p.check.GetExportsOfModule(file.AsNode().Symbol()) {
			if err := p.annotationWork(); err != nil {
				return err
			}
			id, kind, declaration := p.commonTarget(exported)
			if id == "" || declaration == nil {
				return p.fail(file.AsNode(), "export has no complete owned target")
			}
			where := *genericSpan(declaration)
			for _, node := range exported.Declarations {
				if ast.GetSourceFileOfNode(node) == file {
					where = *genericSpan(node)
					break
				}
			}
			module.Exports = append(module.Exports, commonProgramExport{Name: exported.Name, Kind: kind, Target: id,
				TargetSourcePath: ast.GetSourceFileOfNode(declaration).FileName(), Range: where, DefinitionRange: *genericSpan(declaration)})
		}
		if err := sortArrayGeneric(module.Exports, p.annotationWork, func(a, b commonProgramExport) bool { return a.Name < b.Name }); err != nil {
			return err
		}
		state := &arrayGenericState{bindings: map[*ast.Symbol]arrayFactsBinding{}, lengths: map[string]*arrayFactsLengthState{}}
		for _, node := range file.Statements.Nodes {
			if err := p.annotationWork(); err != nil {
				return err
			}
			switch node.Kind {
			case ast.KindVariableStatement:
				for _, declaration := range localDeclarations(node) {
					binding := p.common.declarations[declaration]
					value, err := p.expression(declaration.AsVariableDeclaration().Initializer, binding.DeclaredType, state, 0)
					if err != nil {
						return err
					}
					module.Globals = append(module.Globals, binding.ID)
					module.Initialization = append(module.Initialization, arrayGenericStatement{Kind: "initialize", Range: *genericSpan(declaration), Binding: binding.ID, Initializer: value})
				}
			case ast.KindImportDeclaration, ast.KindExportDeclaration:
				entry, err := p.commonImport(node)
				if err != nil {
					return err
				}
				if entry != nil {
					module.Imports = append(module.Imports, *entry)
				}
			case ast.KindClassDeclaration, ast.KindFunctionDeclaration, ast.KindTypeAliasDeclaration:
				// Definitions live once in the complete type/function directories.
			default:
				return p.fail(node, "module statement has no complete common representation")
			}
		}
		p.common.modules = append(p.common.modules, module)
	}
	resolve := func(roots []closureRoot) ([]commonProgramRoot, error) {
		out := []commonProgramRoot{}
		for _, root := range roots {
			if err := p.annotationWork(); err != nil {
				return nil, err
			}
			file := byPath[root.SourcePath]
			if file == nil {
				return nil, arrayFactsError("identity", "root original source module", nil)
			}
			if root.Kind == "module-initialization" {
				if root.Name != "$moduleInit" {
					return nil, p.fail(file.AsNode(), "original module initializer root name")
				}
				out = append(out, commonProgramRoot{root.Kind, root.SourcePath, root.Name, root.SourcePath, span{0, file.GetPositionMap().UTF8ToUTF16(len(file.Text()))}})
				continue
			}
			var selected *ast.Symbol
			for _, exported := range p.check.GetExportsOfModule(file.AsNode().Symbol()) {
				if err := p.annotationWork(); err != nil {
					return nil, err
				}
				if exported.Name == root.Name {
					selected = exported
				}
			}
			id, kind, declaration := p.commonTarget(selected)
			if kind != "function" || declaration == nil || declaration.Kind != ast.KindFunctionDeclaration {
				return nil, p.fail(file.AsNode(), "root must join its real exported function")
			}
			out = append(out, commonProgramRoot{root.Kind, root.SourcePath, root.Name, id, *genericSpan(declaration)})
		}
		return out, nil
	}
	var err error
	p.common.closure.Roots, err = resolve(p.common.input.CompilationClosure.Roots)
	if err != nil {
		return err
	}
	p.common.closure.RequiredRoots, err = resolve(p.common.input.CompilationClosure.RequiredRoots)
	return err
}
