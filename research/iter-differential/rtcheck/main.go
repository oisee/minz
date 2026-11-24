// Command rtcheck round-trips Nanz programs through the text forms of HIR and
// checks that nothing is lost: for each file it compiles the source directly,
// then re-reads the program from
//
//	hir  — the --emit hir text (steps.HIR) through hir.ParseHIR
//	lanz — lanz.Dump through lanz.Compile
//	nanz — the hir text through nanz.Print and nanz.Parse (mz x.hir --emit nanz)
//
// attaches the source's asserts, and runs them on the raw MIR2 VM, the
// optimised MIR2 VM and the Z80. It prints per format the assert result and
// whether the final .a80 equals the direct compile's. Only APIs present on
// origin/main are used:  go -C <tree>/minzc build -o rtcheck <this file>
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func main() {
	for _, f := range os.Args[1:] {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("%s: READ %v\n", f, err)
			continue
		}
		fmt.Printf("%s: %s\n", f, check(string(b), f))
	}
}

func one(s string) string { return strings.Join(strings.Fields(s), " ") }

func check(src, name string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("PANIC %v", r)
		}
	}()
	orig, err := nanz.Parse(src, name)
	if err != nil {
		return "PARSE " + one(err.Error())
	}
	direct, _ := nanz.Parse(src, name)
	steps, err := pipeline.CompileHIRSteps(direct, pipeline.DefaultOptions())
	if err != nil {
		return "COMPILE " + one(err.Error())
	}
	hirText := steps.HIR
	lanzText := lanz.Dump(direct)
	reread := map[string]func() (*hir.Module, error){
		"hir":  func() (*hir.Module, error) { return hir.ParseHIR(hirText, name) },
		"lanz": func() (*hir.Module, error) { return lanz.Compile(lanzText, name) },
		"nanz": func() (*hir.Module, error) {
			m, err := hir.ParseHIR(hirText, name)
			if err != nil {
				return nil, err
			}
			return nanz.Parse(nanz.Print(m), name)
		},
	}
	var parts []string
	for _, format := range []string{"hir", "lanz", "nanz"} {
		parts = append(parts, format+"="+roundTrip(reread[format], orig, steps.Assembly))
	}
	return strings.Join(parts, " | ")
}

func roundTrip(read func() (*hir.Module, error), orig *hir.Module, directAsm string) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = one(fmt.Sprintf("PANIC %v", r))
		}
	}()
	m, err := read()
	if err != nil {
		return "READ:" + one(err.Error())
	}
	m.Asserts, m.Sandboxes = orig.Asserts, orig.Sandboxes
	m2, _ := read()
	m2.Asserts, m2.Sandboxes = orig.Asserts, orig.Sandboxes
	tiers := ""
	func() {
		defer func() {
			if r := recover(); r != nil {
				tiers += "rawVM-panic "
			}
		}()
		if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(m)); err != nil {
			tiers += "rawVM-fail "
		}
	}()
	steps, err := pipeline.CompileHIRSteps(m2, pipeline.DefaultOptions())
	if err != nil {
		return "COMPILE:" + one(err.Error())
	}
	if err := pipeline.RunAssertsMIR2(m2, steps.MIR2Module); err != nil {
		tiers += "optVM-fail "
	}
	if err := pipeline.RunAssertsZ80(m2, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		tiers += "z80-fail "
	}
	if tiers == "" {
		tiers = "ok"
	}
	same := "a80-differs"
	if steps.Assembly == directAsm {
		same = "a80-same"
	}
	return strings.TrimSpace(tiers) + "/" + same
}
