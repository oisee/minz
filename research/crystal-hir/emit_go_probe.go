// Research probe: existing MIR2-to-Go backend for Crystal scalar HIR.
// This is not a port of the Crystal compiler to Go.
package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2go"
	"os"
	"strings"
)

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: emit_go_probe input.lanz output.go")
	}
	source, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	m, err := lanz.Compile(string(source), "crystal-go-probe")
	if err != nil {
		return err
	}
	funcs := m.Funcs[:0]
	for _, f := range m.Funcs {
		if f.Body != nil {
			if f.Name == "main" {
				return fmt.Errorf("probe expects kernels without main")
			}
			funcs = append(funcs, f)
		}
	}
	m.Funcs = append(funcs, &hir.Func{Name: "main", RetTy: mir2.TyVoid, Body: hir.Blk(&hir.ReturnStmt{})})
	if err = hir.Verify(m); err != nil {
		return err
	}
	output, err := mir2go.Compile(hir.LowerModule(m))
	if err != nil {
		return err
	}
	if strings.Contains(output, "// TODO:") {
		return fmt.Errorf("Go backend left an unsupported opcode; refusing output")
	}
	return os.WriteFile(os.Args[2], []byte(output), 0644)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
