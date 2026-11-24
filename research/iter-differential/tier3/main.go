// Command tier3 runs the asserts of Nanz (.nanz) or C (.c) programs on the
// raw MIR2 VM, the optimised MIR2 VM and the assembled Z80 program, and
// prints one line per file: "<file>: OK" or the first failure of each tier.
// It uses only APIs present on origin/main, so the same source builds
// against either tree:  go -C <tree>/minzc build -o tier3 <this file>
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func main() {
	for _, f := range os.Args[1:] {
		fmt.Printf("%s: %s\n", f, oneLine(run(f)))
	}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func run(f string) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = fmt.Sprintf("PANIC %v", r)
		}
	}()
	b, err := os.ReadFile(f)
	if err != nil {
		return "READ " + err.Error()
	}
	parse := func() (*hir.Module, error) {
		if strings.HasSuffix(f, ".c") {
			return c89.CompileWithOpts(string(b), f, c89.CompileOpts{NoLibc: true})
		}
		return nanz.Parse(string(b), f)
	}
	m, err := parse()
	if err != nil {
		return "PARSE " + err.Error()
	}
	out := ""
	func() {
		defer func() {
			if r := recover(); r != nil {
				out += fmt.Sprintf("rawVM: PANIC %v; ", r)
			}
		}()
		if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(m)); err != nil {
			out += "rawVM: " + err.Error() + "; "
		}
	}()
	m2, _ := parse()
	steps, err := pipeline.CompileHIRSteps(m2, pipeline.DefaultOptions())
	if err != nil {
		return out + "COMPILE " + err.Error()
	}
	if err := pipeline.RunAssertsMIR2(m2, steps.MIR2Module); err != nil {
		out += "optVM: " + err.Error() + "; "
	}
	if err := pipeline.RunAssertsZ80(m2, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		out += "z80: " + err.Error()
	}
	if out == "" {
		return "OK"
	}
	return out
}
