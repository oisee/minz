package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestExternRedeclarationABI(t *testing.T) {
	decl := "@extern fun ext(a: u8, b: u8) -> u8\n"
	for _, mode := range []string{"repeated", "address", "imports"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := decl + decl + "fun twice(x: u8) -> u8 { return ext(x, x) }\nfun main() -> u8 { return twice(3) }"
			if mode == "address" {
				source = strings.ReplaceAll(source, "@extern fun", "@extern(0x1234) fun")
			}
			if mode == "imports" {
				for _, name := range []string{"moda", "modb"} {
					if err := os.WriteFile(filepath.Join(dir, name+".nanz"), []byte(strings.ReplaceAll(decl, "@extern fun", "@extern(0x1234) fun")+"fun twice_"+name+"(x: u8) -> u8 { return ext(x, x) }"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				source = "import moda\nimport modb\nfun main() -> u8 { return twice_moda(3) }"
			}
			hm, err := nanz.ParseWithOpts(source, filepath.Join(dir, "main.nanz"), nanz.ParseOpts{BaseDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, f := range hm.Funcs {
				if f.Name == "ext" {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("extern count=%d, want 1", n)
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "none"
			steps, err := pipeline.CompileHIRSteps(hm, opts)
			if err != nil {
				t.Fatal(err)
			}
			asm := steps.Assembly
			stub := "ext:\n ADD A,C\n RET\n"
			if mode == "address" || mode == "imports" {
				if !strings.Contains(asm, "JP 0x1234") || strings.Contains(asm, "JP ext\n") {
					t.Fatalf("lost extern address:\n%s", asm)
				}
				asm = strings.ReplaceAll(asm, "0x1234", "provided_ext")
				asm += "\nprovided_ext:\n ADD A,C\n RET\n"
			} else {
				asm = strings.Replace(asm, "ext:\n", stub, 1)
			}
			result, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n LD SP,0xFF00\n CALL main\n HALT\n" + asm)
			if err != nil || len(result.Errors) > 0 {
				t.Fatalf("assembly: %v %v", err, result.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, result.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
			for i := 0; !z.IsHalted(); i++ {
				if i == 10000 {
					t.Fatal("execution did not halt")
				}
				z.Step()
			}
			if got := z.GetRegisters().A; got != 6 {
				t.Fatalf("A=%d, want 6", got)
			}
		})
	}
}

func TestExternClobberSetRedeclaration(t *testing.T) {
	m, err := nanz.Parse("@extern(clobbers: \"A, HL, A\") fun ext()\n@extern(clobbers: \"HL, A\") fun ext()", "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 1 {
		t.Fatal("identical clobber sets not deduplicated")
	}
}

func TestExternEmittedSymbolCollision(t *testing.T) {
	hm, err := nanz.Parse("fun f(a: u8) -> u8 { return a + 1 }\n@extern(clobbers: \"\") fun v_f(a: u8) -> u8\nfun main() -> u8 { return v_f(3) + f(5) }", "test")
	if err != nil {
		t.Fatal(err)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	_, err = pipeline.CompileHIRSteps(hm, opts)
	if err == nil || !strings.Contains(err.Error(), "ambiguous Z80 symbol") {
		t.Fatalf("want emitted symbol error, got %v", err)
	}
}
