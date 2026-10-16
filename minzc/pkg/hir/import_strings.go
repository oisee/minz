package hir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// MergeImportedStrings consumes an imported module's string references into
// the target namespace. Importers already transfer its function objects;
// this must run before doing so. Encoding participates in literal identity.
func MergeImportedStrings(target, imported *Module) error {
	if len(imported.Strings) == 0 {
		return nil
	}
	kindAt := func(m *Module, i int) mir2.StringKind {
		if i < len(m.StrKinds) {
			return m.StrKinds[i]
		}
		return mir2.StrSString
	}
	values := append([]string(nil), target.Strings...)
	kinds := make([]mir2.StringKind, len(values))
	for i := range kinds {
		kinds[i] = kindAt(target, i)
	}
	indices := make([]int, len(imported.Strings))
	for i, s := range imported.Strings {
		kind := kindAt(imported, i)
		index := -1
		for j, existing := range values {
			if existing == s && kinds[j] == kind {
				index = j
				break
			}
		}
		if index < 0 {
			index = len(values)
			values = append(values, s)
			kinds = append(kinds, kind)
		}
		indices[i] = index
	}
	remap := func(symbol string) (string, error) {
		if !strings.HasPrefix(symbol, "@mir2.str.") {
			return symbol, nil
		}
		i, err := strconv.Atoi(strings.TrimPrefix(symbol, "@mir2.str."))
		if err != nil || i < 0 || i >= len(indices) {
			return "", fmt.Errorf("invalid imported string reference %q", symbol)
		}
		return fmt.Sprintf("@mir2.str.%d", indices[i]), nil
	}
	// Validate and collect updates first, so a bad reference cannot partially
	// rewrite the linked namespace.
	updates := map[*string]string{}
	var firstErr error
	add := func(p *string) {
		value, err := remap(*p)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		updates[p] = value
	}
	for _, f := range imported.Funcs {
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				visitStmtExprs(s, func(e Expr) {
					if a, ok := e.(*AddrOfExpr); ok && a.X == nil {
						add(&a.Sym)
					}
				})
			}
		})
	}
	for i := range imported.Globals {
		for j := range imported.Globals[i].VtableSyms {
			add(&imported.Globals[i].VtableSyms[j])
		}
	}
	if firstErr != nil {
		return firstErr
	}
	for p, value := range updates {
		*p = value
	}
	target.Strings, target.StrKinds = values, kinds
	return nil
}
