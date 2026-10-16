package c89_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// ISO C90 6.3.3.4: sizeof has type size_t and does not evaluate its operand.
// The target uses unsigned 16-bit size_t; VLA is a C99 extension without runtime support.
func TestC89TargetObjectLimits(t *testing.T) {
	for _, src := range []string{
		"unsigned char data[70000];",
		"unsigned int f(void){return sizeof(char[65536]);}",
		"unsigned int data=sizeof(char[70000]);",
		"unsigned int f(void){char data[70000];return 1;}",
		"unsigned int f(void){static char data[70000];return 1;}",
		"unsigned int data[40000];",
		"struct Big{char data[65536];};struct Big data;",
		"struct Big{char data[65536];};unsigned int f(struct Big data){return 1;}",
		"unsigned int f(void){return sizeof(struct {unsigned int data[40000];});}",
	} {
		if _, err := c89.Compile(src, "target-size.c"); err == nil || !strings.Contains(err.Error(), "exceeds target limit 65535 bytes") {
			t.Fatalf("expected target-size diagnostic for %s: %v", src, err)
		}
	}
	for _, src := range []string{
		"unsigned int f(unsigned int n){return sizeof(char[n]);}",
		"unsigned int f(unsigned int n){return sizeof(char[2][n]);}",
		"unsigned int f(unsigned int n,char data[n]){return data[0];}",
		"unsigned int f(unsigned int n){char (*p)[n];return 1;}",
	} {
		if _, err := c89.Compile(src, "target-vla.c"); err == nil || !strings.Contains(err.Error(), "variable-length array") {
			t.Fatalf("expected VLA diagnostic for %s: %v", src, err)
		}
	}
	// Opaque pointers do not allocate the pointed-to large object.
	for _, src := range []string{
		"extern char data[1]; unsigned int f(void){return 1;}",
		"typedef char Huge[70000];unsigned int f(Huge *p){return 1;}",
	} {
		if _, err := c89.Compile(src, "opaque.c"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestC89SizeofTargetBoundaryVMZ80(t *testing.T) {
	src := "unsigned int size=sizeof(char[65535]); unsigned int f(void){unsigned int x=7;return sizeof(char[65535])+sizeof(x++)+x;} unsigned int global_check(void){return size;}"
	parse := func() *hir.Module {
		m, err := c89.Compile(src, "sizeof-boundary.c")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	check := func(m *mir2.Module) {
		for name, want := range map[string]int64{"f": 8, "global_check": 65535} {
			v, err := mir2.NewVM(m).Call(name, nil)
			if err != nil || len(v) != 1 || v[0].I != want {
				t.Fatalf("%s=%v err=%v want=%d", name, v, err, want)
			}
		}
	}
	check(hir.LowerModule(parse()))
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse()
			m.Asserts = append(m.Asserts, hir.Assert{FuncName: "f", Expected: 8, Via: "z80", Source: "f()"}, hir.Assert{FuncName: "global_check", Expected: 65535, Via: "z80", Source: "global_check()"})
			s, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			check(s.MIR2Module)
		})
	}
}
