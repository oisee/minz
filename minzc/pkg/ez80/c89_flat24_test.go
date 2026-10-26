package ez80

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// This is a scalar-only C bridge smoke test, not a 24-bit C pointer ABI.
// C still uses its existing int16/pointer16 model. First prove that its raw
// MIR has no pointer values or memory operations; only then select flat24 for
// the backend. No pointer-containing types are rewritten or widened here.
func TestC89Flat24ConstantKernels(t *testing.T) {
	cases := []struct {
		name, expr string
		want       byte
	}{
		{"zero", "0", 0},
		{"one", "1", 1},
		{"signed_boundary", "127", 127},
		{"upper_half", "128", 128},
		{"all_bits", "255", 255},
		{"cast_wrap", "(unsigned char)300U", 44},
		{"byte_sum_wrap", "(unsigned char)(250U+17U)", 11},
		{"shift", "((unsigned char)0x81U)>>1", 64},
		{"bitwise", "(0xA5U ^ 0x3CU) & 0x7FU", 25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("unsigned char main(void){return %s;}\n", tc.expr)
			h, err := c89.CompileWithOpts(source, tc.name+".c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			raw := hir.LowerModule(h)
			if len(raw.Globals) != 0 || len(h.Structs) != 0 || raw.Strings.Len() != 0 {
				t.Fatal("scalar-only corpus contains storage")
			}
			for _, f := range raw.Funcs {
				for _, p := range f.Contract.Params {
					if mir2.IsPointer(p.Ty) {
						t.Fatal("C pointer parameter cannot be relabeled flat24")
					}
				}
				for _, ret := range f.Contract.Returns {
					if mir2.IsPointer(ret.Ty) {
						t.Fatal("C pointer return cannot be relabeled flat24")
					}
				}
				for _, b := range f.Blocks {
					for _, i := range b.Insts {
						if mir2.IsPointer(i.Ty) {
							t.Fatalf("C pointer value cannot be relabeled flat24: %s", mir2.DumpInst(i))
						}
						switch i.Op {
						case mir2.OpAddrOf, mir2.OpField, mir2.OpPtrAdd, mir2.OpPtrBump, mir2.OpAlloca, mir2.OpLoad, mir2.OpStore:
							t.Fatalf("non-scalar instruction: %s", mir2.DumpInst(i))
						}
					}
				}
			}
			checkVM := func(m *mir2.Module) {
				t.Helper()
				out, err := mir2.NewVM(m).Call("main", nil)
				if err != nil || len(out) != 1 || out[0].I != int64(tc.want) {
					t.Fatalf("C VM result=%v want=%d err=%v", out, tc.want, err)
				}
			}
			checkVM(raw)
			h.PointerBits = 24
			h.Target = hir.TargetAgon
			m := hir.LowerModule(h)
			for _, f := range m.Funcs {
				for {
					propagated := mir2.PropagateConstants(f)
					folded := mir2.FoldConstants(f)
					if !propagated && !folded {
						break
					}
				}
				mir2.PropagateCopies(f)
				mir2.DeadStoreElim(f)
			}
			if err := mir2.Verify(m); err != nil {
				t.Fatal(err)
			}
			checkVM(m)
			f := m.FuncByName("main")
			ar := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), EZ80CostTable{})
			asm, err := MIR2CodegenChecked(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			a := z80asm.NewAssembler()
			a.SetCPUMode(z80asm.CPUModeEZ80ADL)
			result, err := a.AssembleString(asm)
			if err != nil {
				t.Fatalf("%v\n%s", err, asm)
			}
			if len(result.Errors) > 0 {
				t.Fatalf("%v\n%s", result.Errors, asm)
			}
			if len(result.Binary) == 0 {
				t.Fatal("empty native product")
			}
			t.Logf("C frontend -> checked native ADL assembly: %d bytes, expected A=%d", len(result.Binary), tc.want)
			if runner := os.Getenv("MINZ_EZ80_RUNNER"); runner != "" {
				binPath := filepath.Join(t.TempDir(), tc.name+".bin")
				if err := os.WriteFile(binPath, result.Binary, 0600); err != nil {
					t.Fatal(err)
				}
				out, err := exec.Command(runner, binPath, fmt.Sprintf("%d", tc.want), "--plain").CombinedOutput()
				if err != nil {
					t.Fatalf("native ADL CPU: %v: %s\n%s", err, out, asm)
				}
				t.Logf("%s", out)
			} else {
				t.Log("assembly only; set MINZ_EZ80_RUNNER for native ADL CPU execution")
			}
			// Optional deterministic artifacts let the CPU/differential harness consume
			// these exact compiler products; ordinary Go tests claim assembly only.
			if dir := os.Getenv("MINZ_EZ80_C89_OUT"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for ext, data := range map[string][]byte{"c": []byte(source), "asm": []byte(asm), "bin": result.Binary, "want": []byte(fmt.Sprintf("%d\n", tc.want))} {
					if err := os.WriteFile(filepath.Join(dir, tc.name+"."+ext), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
