// SPDX-License-Identifier: Apache-2.0
// Every alias retains its original local declaration and checker-resolved target.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *arrayGenericProjection) commonImport(node *ast.Node) (*commonProgramImport, error) {
	var specifier *ast.Node
	var names []*ast.Node
	kind := "import"
	if node.Kind == ast.KindImportDeclaration {
		decl := node.AsImportDeclaration()
		specifier = decl.ModuleSpecifier
		if clause := decl.ImportClause; clause != nil {
			value := clause.AsImportClause()
			if value.Name() != nil {
				names = append(names, value.Name())
			}
			if value.NamedBindings != nil {
				if value.NamedBindings.Kind == ast.KindNamespaceImport {
					names = append(names, value.NamedBindings.Name())
				} else if value.NamedBindings.Kind == ast.KindNamedImports {
					for _, binding := range value.NamedBindings.AsNamedImports().Elements.Nodes {
						names = append(names, binding.Name())
					}
				} else {
					return nil, p.fail(node, "original named or namespace import required")
				}
			}
		}
	} else {
		kind = "export"
		decl := node.AsExportDeclaration()
		specifier = decl.ModuleSpecifier
		if decl.ExportClause != nil && decl.ExportClause.Kind == ast.KindNamedExports {
			for _, binding := range decl.ExportClause.AsNamedExports().Elements.Nodes {
				names = append(names, binding.Name())
			}
		}
		if specifier == nil {
			return nil, nil // Local exports are complete in the module export directory.
		}
	}
	if specifier == nil {
		return nil, p.fail(node, "original module specifier required")
	}
	module := p.check.ResolveExternalModuleName(specifier)
	if module == nil || len(module.Declarations) == 0 {
		return nil, p.fail(specifier, "actual resolved module required")
	}
	file := ast.GetSourceFileOfNode(module.Declarations[0])
	if file == nil {
		return nil, p.fail(specifier, "original resolved module source required")
	}
	out := &commonProgramImport{Kind: kind, Specifier: specifier.Text(), SourcePath: file.FileName(), Range: *genericSpan(node), Bindings: []commonProgramAlias{}}
	for _, name := range names {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		symbol := p.check.GetSymbolAtLocation(name)
		id, targetKind, declaration := p.commonTarget(symbol)
		imported := name.Text()
		if name.Parent.Kind == ast.KindNamespaceImport {
			id, targetKind, declaration, imported = file.FileName(), "namespace", file.AsNode(), "*"
		} else if name.Parent.Kind == ast.KindImportClause {
			imported = "default"
		} else if name.Parent.Kind == ast.KindImportSpecifier {
			if property := name.Parent.AsImportSpecifier().PropertyName; property != nil {
				imported = property.Text()
			}
		} else if name.Parent.Kind == ast.KindExportSpecifier {
			if property := name.Parent.AsExportSpecifier().PropertyName; property != nil {
				imported = property.Text()
			}
		}
		if id == "" || declaration == nil {
			return nil, p.fail(name, "module alias has no complete original owned target")
		}
		alias := commonProgramAlias{Kind: targetKind, Name: name.Text(), Imported: imported, Target: id,
			TargetSourcePath: ast.GetSourceFileOfNode(declaration).FileName(), Range: *genericSpan(name.Parent)}
		if kind == "import" {
			binding, err := arrayFactsBindingID(ast.GetSourceFileOfNode(name).FileName(), *genericSpan(name))
			if err != nil {
				return nil, err
			}
			alias.LocalBinding, alias.NameRange = &binding, genericSpan(name)
		}
		out.Bindings = append(out.Bindings, alias)
	}
	return out, nil
}
