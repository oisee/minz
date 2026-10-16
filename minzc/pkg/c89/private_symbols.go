package c89

import cc "github.com/minz/minzc/pkg/cparse"

func (l *lowerer) symbolName(name string) string {
	if renamed, ok := l.privateSymbols[name]; ok {
		return renamed
	}
	return name
}

// Source-module imports are separate C translation units (C99 6.2.2).
// Preserve ordinary standalone HIR names; imported units supply a unique prefix.
func (l *lowerer) initPrivateSymbols(prefix string) {
	if prefix == "" {
		return
	}
	l.privateSymbols = map[string]string{}
	register := func(d *cc.Declarator) {
		if d != nil && d.IsStatic() && !d.IsTypename() {
			l.privateSymbols[d.Name()] = prefix + d.Name()
		}
	}
	for tu := l.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			register(ed.FunctionDefinition.Declarator)
		case cc.ExternalDeclarationDecl:
			for list := ed.Declaration.InitDeclaratorList; list != nil; list = list.InitDeclaratorList {
				if list.InitDeclarator != nil {
					register(list.InitDeclarator.Declarator)
				}
			}
		}
	}
}
