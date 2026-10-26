// Package crystalmeta contains research-only generators shared by Crystal adapters.
package crystalmeta

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

//go:embed repeat.nanz
var generator string

type Receipt struct {
	Protocol           string `json:"protocol"`
	Target             string `json:"target"`
	Function           string `json:"function"`
	Count              int    `json:"count"`
	Delta              int    `json:"delta"`
	GeneratedAdditions int    `json:"generated_additions"`
	GeneratorSHA256    string `json:"generator_sha256"`
	Lanz               string `json:"lanz"`
	HIR                string `json:"hir"`
}

func GenerateRepeat(count, delta int) (Receipt, error) {
	if count < 0 || count > 32 || delta < 0 || delta > 255 {
		return Receipt{}, fmt.Errorf("repeat requires count 0..32 and delta 0..255")
	}
	name := fmt.Sprintf("meta_repeat_u8_%d_%d", count, delta)
	src := generator + fmt.Sprintf("\nfun %s(value:u8)->u8 { @repeat(\"%d\",\"%d\") return value }\n", name, count, delta)
	m, err := nanz.Parse(src, "crystal-meta-repeat.nanz")
	if err != nil {
		return Receipt{}, err
	}
	if err := hir.Verify(m); err != nil {
		return Receipt{}, err
	}
	if len(m.Funcs) != 1 || m.Funcs[0].Name != name || m.Funcs[0].RetTy != mir2.TyU8 || len(m.Funcs[0].Params) != 1 || m.Funcs[0].Params[0].Ty != mir2.TyU8 {
		return Receipt{}, fmt.Errorf("unexpected meta output signature")
	}
	additions := 0
	var walk func(hir.Stmt)
	walk = func(s hir.Stmt) {
		switch s := s.(type) {
		case *hir.Block:
			for _, x := range s.Body {
				walk(x)
			}
		case *hir.AssignStmt:
			if b, ok := s.Val.(*hir.BinExpr); ok && b.Op == "+" {
				additions++
			}
		}
	}
	walk(m.Funcs[0].Body)
	if additions != count {
		return Receipt{}, fmt.Errorf("metafunction emitted %d additions, expected %d", additions, count)
	}
	text := lanz.Dump(m)
	roundtrip, err := lanz.Compile(text, "crystal-meta-roundtrip")
	if err != nil {
		return Receipt{}, err
	}
	if err := hir.Verify(roundtrip); err != nil {
		return Receipt{}, err
	}
	return Receipt{Protocol: "minz-meta-hir-v1", Target: "z80", Function: name, Count: count, Delta: delta, GeneratedAdditions: additions, GeneratorSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(generator))), Lanz: text, HIR: m.Dump()}, nil
}
