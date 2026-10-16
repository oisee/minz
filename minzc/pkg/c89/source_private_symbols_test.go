package c89_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// C99 6.2.2: file-static objects/functions belong to their translation unit.
func TestCSourcePrivateSymbolsNativeVM(t *testing.T) {
	cases := []struct {
		name, first, second string
		want                func(uint32) uint16
	}{
		{"functions", `static unsigned int local(unsigned int n){return n+3;}unsigned int first(unsigned int n){return local(n);}`, `static unsigned int local(unsigned int n){return n+5;}unsigned int second(unsigned int n){return local(n);}`, func(n uint32) uint16 { return uint16(2*n + 8) }},
		{"globals", `static unsigned int state=3;unsigned int first(unsigned int n){return n+state;}`, `static unsigned int state=5;unsigned int second(unsigned int n){return n+state;}`, func(n uint32) uint16 { return uint16(2*n + 8) }},
		{"function_pointer", `static unsigned int local(unsigned int n){return n+3;}unsigned int first(unsigned int n){unsigned int(*p)(unsigned int)=local;return p(n);}`, `static unsigned int local(unsigned int n){return n+5;}unsigned int second(unsigned int n){unsigned int(*p)(unsigned int)=local;return p(n);}`, func(n uint32) uint16 { return uint16(2*n + 8) }},
		{"local_static", `static unsigned int local(unsigned int n){static unsigned int state=3;state++;return n+state;}unsigned int first(unsigned int n){return local(n);}`, `static unsigned int local(unsigned int n){static unsigned int state=5;state++;return n+state;}unsigned int second(unsigned int n){return local(n);}`, func(n uint32) uint16 { return uint16(2*n + 10) }},
		{"shadow", `static unsigned int state=3;unsigned int first(unsigned int state){return state+1;}`, `static unsigned int state=5;unsigned int second(unsigned int n){return n+state;}`, func(n uint32) uint16 { return uint16(2*n + 6) }},
		{"recursion", `static unsigned int local(unsigned int n){return n?local(n-1)+1:0;}unsigned int first(unsigned int n){return local(n%8);}`, `static unsigned int local(unsigned int n){return n+5;}unsigned int second(unsigned int n){return local(n);}`, func(n uint32) uint16 { return uint16(n + n%8 + 5) }},
		{"global_address", `static unsigned int state=3;unsigned int first(unsigned int n){unsigned int*p=&state;return n+*p;}`, `static unsigned int state=5;unsigned int second(unsigned int n){unsigned int*p=&state;return n+*p;}`, func(n uint32) uint16 { return uint16(2*n + 8) }},
	}
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65530, 65531, 65532, 65533, 65534, 65535}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				src := `#import "first.c"
#import "first.c"
#import "second.c"
unsigned int first(unsigned int);unsigned int second(unsigned int);
unsigned int execute(unsigned int n){return first(n)+second(n);}`
				for name, code := range map[string]string{"first.c": c.first, "second.c": c.second} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(code), 0600); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(dir, "entry.c")
				parse := func() *hir.Module {
					m, err := c89.CompileWithOpts(src, path, c89.CompileOpts{BaseDir: dir, Resolver: nanz.CSourceResolver(path, nanz.ParseOpts{})})
					if err != nil {
						t.Fatal(err)
					}
					return m
				}
				m := parse()
				for _, n := range values {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: "execute", Args: []int64{int64(n)}, Expected: int64(c.want(n)), Via: "z80"})
				}
				steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				for _, ir := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
					for _, n := range values {
						got, err := mir2.NewVM(ir).Call("execute", []mir2.Value{{I: int64(n)}})
						if err != nil || len(got) != 1 || got[0].I != int64(c.want(n)) {
							t.Fatalf("%d got=%v err=%v want=%d", n, got, err, c.want(n))
						}
					}
				}
				if root := os.Getenv("SYSTEM_C_PRIVATE_EXPORT"); root != "" {
					out := filepath.Join(root, c.name, mode)
					if err := os.MkdirAll(out, 0700); err != nil {
						t.Fatal(err)
					}
					a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
					if err != nil || len(a.Errors) > 0 {
						t.Fatal(err, a.Errors)
					}
					for name, data := range map[string][]byte{"first.c": []byte(c.first), "second.c": []byte(c.second), "entry.c": []byte(src), "output.a80": []byte("ORG 0x8000\n" + steps.Assembly), "output.bin": a.Binary, "module.hir": []byte(parse().Dump())} {
						if err := os.WriteFile(filepath.Join(out, name), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
			})
		}
	}
}

func TestCSourceConflictingPublicFunctions(t *testing.T) {
	dir := t.TempDir()
	for name, n := range map[string]int{"first.c": 1, "second.c": 2} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(fmt.Sprintf("unsigned int value(void){return %d;}", n)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	name := filepath.Join(dir, "entry.c")
	_, err := c89.CompileWithOpts("#import \"first.c\"\n#import \"second.c\"\nunsigned int main(void){return 0;}", name, c89.CompileOpts{BaseDir: dir, Resolver: nanz.CSourceResolver(name, nanz.ParseOpts{})})
	if err == nil || !strings.Contains(err.Error(), "conflicting source-module function") {
		t.Fatalf("missing conflict diagnostic: %v", err)
	}
}
