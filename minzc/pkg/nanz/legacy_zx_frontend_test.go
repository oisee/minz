package nanz_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// Newlines separate statements; an opening parenthesis on the next line
// must not turn the previous assignment into a call. Explicit returns follow
// Language Book v7 §2.2; this does not add implicit value returns to Nanz.
func TestParenthesizedStatementAfterAssignment(t *testing.T) {
	h, err := nanz.Parse(`global value:u8=0
fun plus(a:u8,b:u8)->u8 { return a+b }
fun probe()->u8 {
 value=3
 (value+7)
 return plus(
 value,
 4)
}`, "newline.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.Verify(h); err != nil {
		t.Fatal(err)
	}
	b := h.FuncByName("probe").Body.Body
	if len(b) != 3 {
		t.Fatalf("statements=%d, want assignment/expression/return", len(b))
	}
	if _, ok := b[1].(*hir.ExprStmt); !ok {
		t.Fatalf("parenthesized statement: %T", b[1])
	}
	got, err := mir2.NewVM(hir.LowerModule(h)).Call("probe", nil)
	if err != nil || len(got) != 1 || got[0].I != 7 {
		t.Fatalf("probe=%v err=%v", got, err)
	}
	if _, err = nanz.Parse(`fun bad()->u8 { let x:u8=(1+2)(3) return x }`, "bad.nanz"); err == nil || !strings.Contains(err.Error(), "call target") {
		t.Fatalf("unsupported callee diagnostic: %v", err)
	}
}

// Compile exact standalone sources, then check their helpers against independent
// bitmap-address and LFSR models. This does not claim execution of their main.
func TestLegacyZXStandaloneHelperModels(t *testing.T) {
	root := findRepoRoot(t)
	for _, name := range []string{"10print_simple", "fire_simple", "life_simple"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "examples", "zx_demos", name+".minz")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var extra strings.Builder
			var want []byte
			global := "test_samples"
			if name == "10print_simple" {
				global = "test_addresses"
				extra.WriteString("\nglobal test_addresses:[u16;6]\nfun test_probe()->void {\n")
				for i, p := range [][2]uint16{{0, 0}, {255, 191}, {8, 8}, {137, 64}, {31, 127}, {240, 128}} {
					fmt.Fprintf(&extra, "test_addresses[%d]=screen_addr(%d,%d)\n", i, p[0], p[1])
					// ZX Spectrum bitmap wiring: 010TTSSS LLLCCCCC.
					addr := uint16(0x4000) | ((p[1] & 0xc0) << 5) | ((p[1] & 7) << 8) | ((p[1] & 0x38) << 2) | (p[0] >> 3)
					want = append(want, byte(addr), byte(addr>>8))
				}
				extra.WriteString("}\n")
			} else {
				extra.WriteString("\nglobal test_samples:[u8;64]\nfun test_probe()->void {\nvar i:u8=0\nwhile i<64 {\ntest_samples[i]=random_u8()\ni=i+1\n}\n}\n")
				seed := uint16(0xace1)
				for i := 0; i < 64; i++ {
					feedback := (seed ^ (seed >> 2) ^ (seed >> 3) ^ (seed >> 5)) & 1
					seed = (seed >> 1) | (feedback << 15)
					want = append(want, byte(seed>>8))
				}
			}
			h, err := nanz.Parse(string(source)+extra.String(), path)
			if err != nil {
				t.Fatal(err)
			}
			if err = hir.Verify(h); err != nil {
				t.Fatal(err)
			}
			raw := mir2.NewVM(hir.LowerModule(h))
			steps, err := pipeline.CompileHIRSteps(h)
			if err != nil {
				t.Fatal(err)
			}
			for mode, vm := range map[string]*mir2.VM{"raw": raw, "optimized": mir2.NewVM(steps.MIR2Module)} {
				if _, err = vm.Call("test_probe", nil); err != nil {
					t.Fatalf("%s: %v", mode, err)
				}
				got := vm.ReadHeap(vm.GlobalAddr(global), len(want))
				if string(got) != string(want) {
					t.Fatalf("%s %s=%x want %x", mode, global, got, want)
				}
			}
			asm := "ORG 0x8000\n LD SP,0xF000\n CALL test_probe\n test_done:\n DI\n HALT\n" + steps.Assembly
			a, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.MaxCycles = 3000000
			if err = z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			z.SetPC(0x8000)
			if err = z.Run(); err != nil {
				t.Fatal(err)
			}
			regs := z.GetRegisters()
			if regs.SP != 0xf000 || int(regs.PC) != a.Symbols["test_done"]+1 {
				t.Fatalf("PC/SP=%x/%x", regs.PC, regs.SP)
			}
			addr, ok := a.Symbols[global]
			if !ok {
				t.Fatal("missing output", global)
			}
			for i, v := range want {
				if got := z.GetMemory(uint16(addr + i)); got != v {
					t.Fatalf("native %s[%d]=%x want %x", global, i, got, v)
				}
			}
			t.Logf("source compiled: %d bytes; helper model: %d bytes; native PC/SP restored", len(a.Binary), len(want))
		})
	}
}
