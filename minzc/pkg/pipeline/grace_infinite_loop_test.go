package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Nontermination is observable: a reachable cycle must survive CFG cleanup,
// even when it contains no instructions or return block.
func TestGracePreservesInfiniteLoops(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		loops := []string{"for (;;) { %s }", "while (1) { %s }"}
		if language == "nanz" {
			loops = []string{"loop { %s }", "while true { %s }"}
		}
		for i, loop := range loops {
			for _, effect := range []bool{false, true} {
				for _, mode := range []string{"hir", "mir", "both"} {
					t.Run(fmt.Sprintf("%s/%d/effect%v/%s", language, i, effect, mode), func(t *testing.T) {
						body := ""
						if effect {
							body = "tick();"
						}
						src := "extern void tick(void); void spin(void) {" + fmt.Sprintf(loop, body) + "}"
						var h *hir.Module
						var err error
						if language == "c" {
							h, err = c89.Compile(src, "spin.c")
						} else {
							src = "@extern fun tick()->void\nfun spin()->void {" + fmt.Sprintf(loop, strings.TrimSuffix(body, ";")) + "}"
							h, err = nanz.Parse(src, "spin.nanz")
						}
						if err != nil {
							t.Fatal(err)
						}
						s, err := CompileHIRSteps(h, Options{UseGrace: mode != "hir", UseHIRGrace: mode != "mir", AssertMode: "none"})
						if err != nil {
							t.Fatal(err)
						}
						if err := mir2.Verify(s.MIR2Module); err != nil {
							t.Fatal(err)
						}
						f := s.MIR2Module.FuncByName("spin")
						// Every reachable successor must exist, and the finite CFG must end
						// in a cycle rather than a return. This also checks empty VM loops,
						// whose terminators currently do not consume VM gas.
						seen := map[string]bool{}
						var visit func(string)
						visit = func(label string) {
							if seen[label] {
								return
							}
							seen[label] = true
							b := f.BlockByLabel(label)
							if b == nil {
								t.Fatalf("missing reachable block %s", label)
							}
							j, ok := b.Term.(*mir2.TermJmp)
							if !ok {
								t.Fatalf("infinite loop acquired exit: %T", b.Term)
							}
							visit(j.Target)
						}
						visit(f.Blocks[0].Label)
						if effect {
							vm := mir2.NewVM(s.MIR2Module)
							vm.MaxSteps = 64
							calls := 0
							vm.Hosts["tick"] = func([]mir2.Value) ([]mir2.Value, error) { calls++; return nil, nil }
							_, err := vm.Call("spin", nil)
							if err == nil || !strings.Contains(err.Error(), "gas limit") || calls < 2 {
								t.Fatalf("side effect lost: calls=%d err=%v", calls, err)
							}
						}
						// Execute a bounded prefix on the real backend; returning would hit
						// the sentinel instead of remaining inside the assembled program.
						assembly := "ORG 0x8000\n" + s.Assembly
						if language == "c" {
							assembly += "\ntick:\n RET\n"
						} else {
							assembly = strings.Replace(assembly, "\ntick:\n", "\ntick:\n RET\n", 1)
						}
						r, err := z80asm.NewAssembler().AssembleString(assembly)
						if err != nil || len(r.Errors) != 0 {
							t.Fatalf("assemble: %v %v", err, r.Errors)
						}
						z := emulator.NewRemogattoZ80WithScreen().RemogattoZ80
						z.SetSMCTracker(nil)
						if err := z.LoadMemory(0x8000, r.Binary); err != nil {
							t.Fatal(err)
						}
						z.SetSP(0xff00)
						z.SetMemory(0xff00, 0)
						z.SetMemory(0xff01, 1)
						pc, ok := r.Symbols["spin"]
						if !ok {
							pc, ok = r.Symbols["SPIN"]
						}
						if !ok {
							t.Fatal("missing spin symbol")
						}
						z.SetPC(uint16(pc))
						for step := 0; step < 256; step++ {
							z.Step()
							if z.GetPC() < 0x8000 || int(z.GetPC()) >= 0x8000+len(r.Binary) {
								t.Fatalf("loop escaped at %04x", z.GetPC())
							}
						}
					})
				}
			}
		}
	}
	// Replay only the reported assert_matrix source, without running the matrix.
	for _, backend := range []string{"mir2", "z80"} {
		t.Run("c23_attributes/"+backend, func(t *testing.T) {
			source, err := os.ReadFile("../../../examples/c/c23_attributes.c")
			if err != nil {
				t.Fatal(err)
			}
			h, err := c89.Compile(string(source), "c23_attributes.c")
			if err != nil {
				t.Fatal(err)
			}
			if len(h.Asserts) != 9 {
				t.Fatalf("C23 assertion inventory changed: %d", len(h.Asserts))
			}
			// Source annotations say "via mir2"; force the selected backend so
			// the native replay cannot silently skip the nine assertions.
			for i := range h.Asserts {
				h.Asserts[i].Via = backend
			}
			h.AssertStats = &hir.AssertStats{}
			if _, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, AssertMode: backend}); err != nil {
				t.Fatal(err)
			}
			if *h.AssertStats != (hir.AssertStats{Executed: 9, Passed: 9}) {
				t.Fatalf("%s assertion receipt: %+v", backend, h.AssertStats)
			}
			t.Logf("%s assertion receipt: %+v", backend, h.AssertStats)
		})
	}
}
