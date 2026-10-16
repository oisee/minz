package main

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

func TestStandaloneCSourceImportsNativeVM(t *testing.T) {
	hirProfileTestState(t)
	oldGrace, oldHIR := useGrace, useHIRGrace
	t.Cleanup(func() { useGrace, useHIRGrace = oldGrace, oldHIR })
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	const code = `#import "sub/bridge.c"
#include "api.h"
extern unsigned long calls;
unsigned long execute(unsigned long x){calls=0;return bridge(x)+calls;}
// assert execute(65535) == 65541 via z80
// assert execute(4294967295) == 5 via z80
`
	fixtures := map[string]string{
		"entry.c": code, "api.h": "unsigned long bridge(unsigned long);",
		"sub/bridge.c":   "#import \"leaf.nanz\"\n#include \"leaf.h\"\nunsigned long bridge(unsigned long x){return leaf(x);}",
		"sub/leaf.h":     "unsigned long leaf(unsigned long);",
		"sub/leaf.nanz":  "import other\nglobal calls:u32\nfun leaf(x:u32)->u32{calls=calls+1 return other.plus(x)}",
		"sub/other.nanz": "fun plus(x:u32)->u32{return x+5}",
	}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "entry.c")
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			useGrace, useHIRGrace, optSize = mode != "default", mode != "default", mode == "size"
			assertMode = "z80"
			outputFile = filepath.Join(dir, mode+".a80")
			if err := compile(source); err != nil {
				t.Fatal(err)
			}
			asm, err := os.ReadFile(outputFile)
			if err != nil || len(asm) == 0 || !strings.Contains(string(asm), "bridge:") {
				t.Fatal("missing actual CLI assembly", err)
			}
			image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(asm))
			if err != nil {
				t.Fatal(err)
			}
			if len(image.Errors) != 0 || len(image.Binary) == 0 {
				t.Fatalf("CLI assembly errors: %v", image.Errors)
			}
			parse := func() *hir.Module {
				m, err := c89.CompileWithOpts(code, source, c89.CompileOpts{BaseDir: dir, IncludePaths: []string{dir}, Resolver: nanz.CSourceResolver(source, nanz.ParseOpts{OptSize: optSize})})
				if err != nil {
					t.Fatal(err)
				}
				return m
			}
			m := parse()
			m.Asserts = nil
			values := []uint32{0, 1, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xfffffffa, 0xffffffff}
			for _, v := range values {
				m.Asserts = append(m.Asserts, hir.Assert{FuncName: "execute", Args: []int64{int64(v)}, Expected: int64(v + 6), Via: "z80"})
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: useGrace, UseHIRGrace: useHIRGrace, OptSize: optSize})
			if err != nil {
				t.Fatal(err)
			}
			for _, ir := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				vm := mir2.NewVM(ir)
				for _, v := range values {
					got, err := vm.Call("execute", []mir2.Value{{I: int64(v)}})
					want := int64(v + 6)
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("%08x got=%v err=%v want=%d", v, got, err, want)
					}
				}
			}
			if root := os.Getenv("SYSTEM_C_SOURCE_API_EXPORT"); root != "" {
				out := filepath.Join(root, mode)
				if err := os.MkdirAll(out, 0700); err != nil {
					t.Fatal(err)
				}
				exports := map[string][]byte{"cli.a80": asm, "cli-origin.a80": []byte("ORG 0x8000\n" + string(asm)), "cli.bin": image.Binary, "module.hir": []byte(parse().Dump())}
				for name, data := range fixtures {
					exports[name] = []byte(data)
				}
				for name, data := range exports {
					if err := os.MkdirAll(filepath.Dir(filepath.Join(out, name)), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(out, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestCImportWithoutResolverDiagnostic(t *testing.T) {
	_, err := c89.Compile("#import \"driver.nanz\"\nunsigned int main(void){return 0;}", "missing-resolver.c")
	if err == nil || !strings.Contains(err.Error(), "require CompileOpts.Resolver") {
		t.Fatalf("missing resolver error: %v", err)
	}
	if _, err := c89.Compile("unsigned int main(void){return 0;}", "ordinary.c"); err != nil {
		t.Fatal(fmt.Sprint(err))
	}
}
