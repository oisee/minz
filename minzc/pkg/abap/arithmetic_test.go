package abap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCheckedArithmeticReferenceAndFaults(t *testing.T) {
	src, err := os.ReadFile("testdata/arithmetic-reference/arithmetic.abap")
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := os.ReadFile("testdata/arithmetic-reference/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/arithmetic-reference/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		SourceSHA string `json:"source_sha256"`
		InputsSHA string `json:"inputs_sha256"`
		Cases     []struct {
			Input  [2]int32         `json:"input"`
			Values map[string]int32 `json:"values"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.SourceSHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.InputsSHA != fmt.Sprintf("%x", sha256.Sum256(inputs)) {
		t.Fatal("reference inputs changed")
	}
	if len(ref.Cases) != 13 {
		t.Fatal("missing division reference cases")
	}
	m, err := Compile(string(src), "zref")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, asm.Errors)
	}
	modules := []*mir2.Module{hir.LowerModule(m), steps.MIR2Module}
	run := func(t *testing.T, fn string, a, b int32, want map[string]int32, fault byte) {
		t.Helper()
		for _, module := range modules {
			v := mir2.NewVM(module)
			for name, val := range map[string]int32{"x": a, "y": b} {
				var buf [4]byte
				binary.LittleEndian.PutUint32(buf[:], uint32(val))
				v.WriteHeapSlice(v.GlobalAddr(name), buf[:])
			}
			_, err := v.Call(fn, nil)
			if (err != nil) != (fault != 0) {
				t.Fatalf("VM error %v want fault %d", err, fault)
			}
			if code := v.ReadHeap(v.GlobalAddr("_abap_error_code"), 1)[0]; code != fault {
				t.Fatalf("VM code %d want %d", code, fault)
			}
			for name, val := range want {
				if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr(name), 4))); got != val {
					t.Fatalf("VM %s=%d want %d", name, got, val)
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], asm.Binary)
		for name, val := range map[string]int32{"x": a, "y": b} {
			addr := asm.Symbols[name]
			binary.LittleEndian.PutUint32(mem[addr:addr+4], uint32(val))
		}
		mem[0xeffe], mem[0xefff] = 0, 0xf1
		mem[0xe000] = 0xa5
		z.SetRegisters(emulator.Registers{PC: uint16(asm.Symbols[fn]), SP: 0xeffe})
		for i := 0; i < 1000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if fault == 0 {
			if r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatalf("return PC=%04x SP=%04x", r.PC, r.SP)
			}
		} else {
			if !z.IsHalted() || r.PC == 0xf100 || r.SP > 0xeffe || r.SP <= 0xe000 || (mem[r.PC] != 0x76 && mem[r.PC-1] != 0x76) || z.GetIFF1() {
				t.Fatalf("fault did not stop: regs=%v halted=%v", r, z.IsHalted())
			}
			t.Logf("native fault=%d PC=%04x SP=%04x (terminal halt; caller stack remains active)", fault, r.PC, r.SP)
		}
		if mem[0xe000] != 0xa5 || binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
			t.Fatal("stack canary/outer return address damaged")
		}
		if code := mem[asm.Symbols["_abap_error_code"]]; code != fault {
			t.Fatalf("native code %d want %d", code, fault)
		}
		for name, val := range want {
			addr := asm.Symbols[name]
			if got := int32(binary.LittleEndian.Uint32(mem[addr : addr+4])); got != val {
				t.Fatalf("native %s=%d want %d", name, got, val)
			}
		}
	}
	for _, c := range ref.Cases {
		t.Run(fmt.Sprintf("division/%d/%d", c.Input[0], c.Input[1]), func(t *testing.T) { run(t, "probe", c.Input[0], c.Input[1], c.Values, 0) })
	}
	// Expectations follow signed 32-bit bounds and terminal error semantics,
	// not the reference runtime whose Integer.set overflow guard is disabled.
	for _, c := range []struct {
		fn   string
		a, b int32
		code byte
	}{
		{"plus", 2147483647, 1, 2}, {"plus", -2147483648, -1, 2},
		{"minus", -2147483648, 1, 2}, {"minus", 2147483647, -1, 2},
		{"product", 2147483647, 2, 2}, {"product", -46341, 46341, 2},
		{"negate", -2147483648, 0, 2}, {"probe", -2147483648, -1, 2},
		{"probe", 1, 0, 1}, {"probe", -1, 0, 1},
	} {
		t.Run(fmt.Sprintf("fault/%s/%d/%d", c.fn, c.a, c.b), func(t *testing.T) { run(t, c.fn, c.a, c.b, map[string]int32{"x": c.a, "y": c.b, "seen": 31}, c.code) })
	}
	for _, c := range []struct {
		fn      string
		a, b, w int32
	}{
		{"plus", -2147483648, 2147483647, -1}, {"minus", 2147483647, 2147483647, 0},
		{"product", -2147483648, 1, -2147483648}, {"product", -2147483648, 0, 0},
		{"product", -46340, 46340, -2147395600}, {"negate", -2147483647, 0, 2147483647},
	} {
		t.Run(fmt.Sprintf("valid/%s/%d/%d", c.fn, c.a, c.b), func(t *testing.T) { run(t, c.fn, c.a, c.b, map[string]int32{"x": c.w, "y": c.b, "seen": 99}, 0) })
	}
}
