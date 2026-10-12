package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Book v7 §21.4: last value form; explicit early return exits immediately.
func TestLizpLastFormEffectsNative(t *testing.T) {
	const source = `(defglobal effects u8 0)
 (defun choose ((n u8)) -> u8
  (setq effects (+ effects 1))
  (let ((x u8 (+ n 1)))
   (cond ((= n 0) (setq effects (+ effects 2)) (return 7))
         (t (setq effects (+ effects 3)) (+ x 2)))))
 (defun explicit_let ((n u8)) -> u8
  (return (let ((x u8 (+ n 4))) (progn (setq effects (+ effects 5)) x))))
 (defun early ((n u8)) -> u8
  (if (= n 0) (return 9)) (setq effects (+ effects 6)) (+ n 1))`
	saveLizpTailArtifact(t, "effects/source.lizp", source)
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			parse := func() *hir.Module {
				h, e := lizp.Compile(source, "tail")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := hir.LowerModule(parse())
			steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			saveLizpTailArtifact(t, "effects/"+mode+"/program.a80", steps.Assembly)
			saveLizpTailArtifact(t, "effects/"+mode+"/program.bin", a.Binary)
			saveLizpTailArtifact(t, "effects/"+mode+"/raw.mir", raw.Dump())
			saveLizpTailArtifact(t, "effects/"+mode+"/final.mir", steps.MIR2Module.Dump())
			for _, fn := range []string{"choose", "explicit_let", "early"} {
				for n := 0; n < 256; n++ {
					want, effects := (n+3)&255, 4
					if fn == "choose" && n == 0 {
						want, effects = 7, 3
					}
					if fn == "explicit_let" {
						want, effects = (n+4)&255, 5
					}
					if fn == "early" {
						want, effects = (n+1)&255, 6
						if n == 0 {
							want, effects = 9, 0
						}
					}
					for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(m)
						got, e := vm.Call(fn, []mir2.Value{{I: int64(n)}})
						if e != nil || len(got) != 1 || got[0].I != int64(want) || vm.ReadHeap(vm.GlobalAddr("effects"), 1)[0] != byte(effects) {
							t.Fatal(fn, n, got, want, e)
						}
					}
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, a.Binary)
					z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
					z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[fn]), SP: 0xeff0, A: byte(n)})
					for j := 0; j < 10000 && !(z.GetPC() == 0x7000 && z.GetSP() == 0xeff2); j++ {
						z.Step()
					}
					if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || z.GetRegisters().A != byte(want) || z.Memory()[uint16(a.Symbols["effects"])] != byte(effects) {
						t.Fatal(fn, n, want, effects, z.GetRegisters())
					}
				}
			}
			saveLizpTailArtifact(t, "effects/"+mode+"/result.json", map[string]any{"native_calls": 768, "raw_vm_calls": 768, "final_vm_calls": 768, "checks": "A result, effects global, exact return PC=0x7000/SP=0xeff2", "PASS": true})
		})
	}
}

func saveLizpTailArtifact(t *testing.T, name string, value any) {
	t.Helper()
	dir := os.Getenv("MINZ_LIZP_TAIL_ARTIFACTS")
	if dir == "" {
		return
	}
	var b []byte
	var err error
	switch v := value.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		b, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, b, 0644); err != nil {
		t.Fatal(err)
	}
}
