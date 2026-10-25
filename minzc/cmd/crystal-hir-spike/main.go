// A research-only consumer of the native Crystal parser bridge's Lanz output.
package main

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: crystal-hir-spike input.lanz output-prefix")
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	m, err := lanz.Compile(string(src), "crystal_spike")
	if err != nil {
		return err
	}
	// Signature stubs are only for Lanz's single-pass type lookup.
	var funcs []*hir.Func
	for _, f := range m.Funcs {
		if f.Body != nil {
			funcs = append(funcs, f)
		}
	}
	m.Funcs = funcs
	if err := hir.Verify(m); err != nil {
		return err
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		return fmt.Errorf("raw MIR2: %w", err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true})
	if err != nil {
		return err
	}
	for _, out := range []struct{ suffix, contents string }{
		{".hir", steps.HIR}, {".a80", steps.Assembly}, {".mir2", steps.MIR2Opt},
	} {
		if err := os.WriteFile(os.Args[2]+out.suffix, []byte(out.contents), 0644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "Crystal AST -> Lanz -> HIR -> MIR2 -> Z80: %d functions, %d assertions passed\n", len(m.Funcs), len(m.Asserts))
	return nil
}
