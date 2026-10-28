package pipeline

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// F6: the fused fold loop must not reserve a register pair for its two
// loop-invariant constants (0 for the fe_head counter test, 1 for the fe_body
// decrement).  The Z80 codegen already serves both uses from immediates
// (LD A,D / OR E and DEC rr), so the reserved pair is only ever saved and
// restored around the callback call.
func TestConstRematFusionFoldMaxHoldsNoConstantPair(t *testing.T) {
	root := filepath.Join("..", "..", "..", "research", "c89-benchmark", "fusion")
	src, err := os.ReadFile(filepath.Join(root, "nanz-idiomatic", "fusion_fold_max.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	var workload struct {
		Sizes   map[string]int `json:"sizes"`
		Samples []struct {
			ID       string            `json:"id"`
			Inputs   map[string][]byte `json:"inputs"`
			Expected uint16            `json:"expected"`
			After    map[string][]byte `json:"after"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(mustReadConstRemat(t, filepath.Join(root, "workloads", "fusion_fold_max.json")), &workload); err != nil {
		t.Fatal(err)
	}

	mod, err := nanz.Parse(string(src), "fusion_fold_max.nanz")
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(mod)
	steps, err := CompileHIRSteps(mod, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	// No loop-invariant 0/1 constant is set up into a register pair anywhere in
	// execute, and no pair is pushed inside the loop to protect one.
	pairConst := regexp.MustCompile(`(?m)^\s*LD (BC|DE|HL|IX|IY), [01]\s*$`)
	for _, bad := range pairConst.FindAllString(steps.Assembly, -1) {
		t.Errorf("constant materialised into a held pair: %q", strings.TrimSpace(bad))
	}
	for _, push := range []string{"PUSH IX", "PUSH IY", "PUSH BC"} {
		if strings.Contains(steps.Assembly, push) {
			t.Errorf("%s saves a reserved constant pair", push)
		}
	}
	if t.Failed() {
		t.Fatalf("fold_max assembly still reserves a constant pair:\n%s", steps.Assembly)
	}

	// Values: the frozen workload must still produce the frozen results on the
	// MIR2 VM (raw and final) and on the Z80 emulator.
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	for _, s := range workload.Samples {
		for tier, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
			vm := mir2.NewVM(m)
			for name, data := range s.Inputs {
				vm.WriteHeapSlice(vm.GlobalAddr(name), data)
			}
			vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0xef, 0xbe})
			if _, err := vm.Call("execute", nil); err != nil {
				t.Fatalf("%s %s: %v", s.ID, tier, err)
			}
			if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != s.Expected {
				t.Fatalf("%s %s: got %d want %d", s.ID, tier, got, s.Expected)
			}
		}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], a.Binary)
		for name, data := range s.Inputs {
			copy(mem[a.Symbols[name]:], data)
		}
		binary.LittleEndian.PutUint16(mem[a.Symbols["result"]:], 0xbeef)
		binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
		z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["execute"]), SP: 0xeffe})
		for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
			z.Step()
		}
		regs := z.GetRegisters()
		if regs.PC != 0xf100 || regs.SP != 0xf000 {
			t.Fatalf("%s: PC/SP %04x/%04x", s.ID, regs.PC, regs.SP)
		}
		if got := binary.LittleEndian.Uint16(mem[a.Symbols["result"]:]); got != s.Expected {
			t.Fatalf("%s native: got %d want %d", s.ID, got, s.Expected)
		}
	}
}

func mustReadConstRemat(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
