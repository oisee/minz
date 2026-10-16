package nanz_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Existing source-module imports must preserve C globals, included types and
// private calls, and give distinct modules distinct namespaces on native Z80.
func TestCSourceImportNativeVM(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"driver.h": "struct Pair { unsigned char tag; unsigned int value; };\n",
		"driver.c": `#include "driver.h"
unsigned int calls=0;
struct Pair packet={2,513};
static unsigned int local(unsigned int x) { return x+3; }
unsigned int step(unsigned int x) { calls++; return local(x); }
unsigned int packet_value(void) { return packet.value+packet.tag; }`,
		"other.c": `static unsigned int local(unsigned int x) { return x+5; }
unsigned int step(unsigned int x) { return local(x); }`,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	src := `import driver { step as advance, calls, packet_value, Pair }
import other
fun exercise(x:u16)->u16 {
 calls=0
 let a:u16=advance(x)
 let b:u16=advance(a)
 return b+calls
}
fun other_step(x:u16)->u16 {return other.step(x)}
fun included_packet()->u16 {return packet_value()}
fun local_pair()->u16 {var v:Pair=Pair{tag:7,value:513} return v.value+u16(v.tag)}
`
	parse := func() *hir.Module {
		m, err := nanz.ParseWithOpts(src, "caller.nanz", nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	cases := []struct {
		name      string
		arg, want int64
	}{{"exercise", 0, 8}, {"exercise", 32767, 32775}, {"exercise", 65535, 7}, {"other_step", 10, 15}, {"included_packet", 0, 515}, {"local_pair", 0, 520}}
	for _, opt := range []pipeline.Options{{AssertMode: "z80", ContractOpt: true}, {AssertMode: "z80", ContractOpt: true, UseGrace: true, UseHIRGrace: true}} {
		m := parse()
		for _, tc := range cases {
			args := []int64{tc.arg}
			if tc.name == "included_packet" || tc.name == "local_pair" {
				args = nil
			}
			m.Asserts = append(m.Asserts, hir.Assert{FuncName: tc.name, Args: args, Expected: tc.want, Via: "z80"})
		}
		steps, err := pipeline.CompileHIRSteps(m, opt)
		if err != nil {
			t.Fatal(err)
		}
		for _, mod := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(mod)
			for _, tc := range cases {
				args := []mir2.Value{{I: tc.arg}}
				if tc.name == "included_packet" || tc.name == "local_pair" {
					args = nil
				}
				got, err := vm.Call(tc.name, args)
				if err != nil || len(got) != 1 || got[0].I != tc.want {
					t.Fatalf("%s(%d): %v, %v; want%d", tc.name, tc.arg, got, err, tc.want)
				}
			}
		}
	}
}
