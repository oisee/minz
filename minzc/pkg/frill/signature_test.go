package frill

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestCallUsesDeclaredReturnAndArgumentWidths(t *testing.T) {
	hm, err := Compile(`let identity (x : u16) : u16 = x
let caller (x : u16) : u16 = identity x
assert caller 300 == 300`, "widths")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(hm); err != nil {
		t.Fatal(err)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsZ80(hm, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Fatal(err)
	}
}

func TestVoidExternCallIsEffectNotByteValue(t *testing.T) {
	hm, err := Compile(`extern output (x : u8)
let main (x : u8) : IO u8 = do output x 0`, "effects")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(hm); err != nil {
		t.Fatal(err)
	}
	call := hm.Funcs[1].Body.Body[0].(*hir.ExprStmt).Expr.(*hir.CallExpr)
	if call.Ty != mir2.TyVoid {
		t.Fatalf("output call type = %v, want void", call.Ty)
	}
	invalid, err := Compile(`extern output (x : u8)
let main (x : u8) : IO u8 = output x`, "invalid")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(invalid); err == nil {
		t.Fatal("void output accepted as a byte return")
	}
}

func TestCPMExamplesImportActualSignatures(t *testing.T) {
	for _, name := range []string{"hello_cpm", "interactive"} {
		t.Run(name, func(t *testing.T) {
			path := "../../../examples/frill/" + name + ".frl"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hm, err := CompileWithOpts(string(src), path, CompileOpts{BaseDir: filepath.Dir(path)})
			if err != nil {
				t.Fatal(err)
			}
			if err := hir.Verify(hm); err != nil {
				t.Fatal(err)
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "none"
			steps, err := pipeline.CompileHIRSteps(hm, opts)
			if err != nil {
				t.Fatal(err)
			}
			assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + steps.Assembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(assembled.Errors) != 0 {
				t.Fatal(assembled.Errors)
			}
		})
	}
}
