// Research-only adapter: native Crystal macro marker -> actual Nanz meta VM
// execution -> verified typed HIR -> Lanz interchange. No generator in Go.
package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

//go:embed repeat.nanz
var generator string

type receipt struct {
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

func generate(count, delta int) (receipt, error) {
	if count < 0 || count > 32 || delta < 0 || delta > 255 {
		return receipt{}, fmt.Errorf("repeat requires count 0..32 and delta 0..255")
	}
	name := fmt.Sprintf("meta_repeat_u8_%d_%d", count, delta)
	src := generator + fmt.Sprintf("\nfun %s(value:u8)->u8 { @repeat(\"%d\",\"%d\") return value }\n", name, count, delta)
	m, err := nanz.Parse(src, "crystal-meta-repeat.nanz")
	if err != nil {
		return receipt{}, err
	}
	if err := hir.Verify(m); err != nil {
		return receipt{}, err
	}
	if len(m.Funcs) != 1 || m.Funcs[0].Name != name || m.Funcs[0].RetTy != mir2.TyU8 || len(m.Funcs[0].Params) != 1 || m.Funcs[0].Params[0].Ty != mir2.TyU8 {
		return receipt{}, fmt.Errorf("unexpected meta output signature")
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
		return receipt{}, fmt.Errorf("metafunction emitted %d additions, expected %d", additions, count)
	}
	text := lanz.Dump(m)
	roundtrip, err := lanz.Compile(text, "crystal-meta-roundtrip")
	if err != nil {
		return receipt{}, err
	}
	if err := hir.Verify(roundtrip); err != nil {
		return receipt{}, err
	}
	return receipt{Protocol: "minz-meta-hir-v1", Target: "z80", Function: name, Count: count, Delta: delta, GeneratedAdditions: additions, GeneratorSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(generator))), Lanz: text, HIR: m.Dump()}, nil
}

func run() error {
	if len(os.Args) != 4 || os.Args[1] != "repeat" {
		return fmt.Errorf("usage: crystal-meta-bridge repeat COUNT DELTA")
	}
	count, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return fmt.Errorf("invalid count: %w", err)
	}
	delta, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return fmt.Errorf("invalid delta: %w", err)
	}
	out, err := generate(count, delta)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
