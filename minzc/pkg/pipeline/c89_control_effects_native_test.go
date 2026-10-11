package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ISO C90 6.6.4/6.6.5: selected arms, repeated loop controls and continue
// targets; postfix's old value is sequenced before the subsequent statement.
func TestC89ControlEffectsNative(t *testing.T) {
	src, e := os.ReadFile("../../../reports/c89-control-effects/source.c")
	if e != nil {
		t.Fatal(e)
	}
	freezeBytes, e := os.ReadFile("../../../reports/c89-control-effects/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze map[string]any
	if e = json.Unmarshal(freezeBytes, &freeze); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze["source_sha256"] {
		t.Fatal("source freeze")
	}
	save := func(name string, v any) {
		if dir := os.Getenv("MINZ_C89_CONTROL_ARTIFACTS"); dir != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			p := filepath.Join(dir, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := c89.CompileWithOpts(string(src), "control-effects.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	type observation struct {
		Function string
		Argument int
		Result   byte
		State    byte
	}
	// Arithmetic recurrence is independent of frontend lowering or generated IR.
	model := func(f string, n int) (byte, byte, bool) {
		sum, state := 0, n+1
		write := true
		switch f {
		case "choose":
			sum = 7
			state = 7
			write = n == 0
		case "choose_else":
			sum = 7
			if n == 0 {
				sum = 8
			}
			write = false
		case "while_post", "for_post", "do_post":
			sum = n * (n + 1) / 2
		case "for_continue":
			sum = n * (n - 1) / 2
			if n > 2 {
				sum -= 2
			}
			state = n
		case "do_continue":
			sum = n * (n + 1) / 2
			if n >= 2 {
				sum -= 2
			}
		case "while_call":
			if n > 0 {
				sum = n * (n - 1) / 2
			}
			if n > 2 {
				sum -= 2
			}
			state = n
			if n == 0 {
				state = 1
			}
		}
		return byte(sum), byte(state), write
	}
	gccBytes, err := os.ReadFile("../../../reports/c89-control-effects/gcc-results.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(gccBytes)), "\n") {
		var f string
		var n, value, state int
		if _, err := fmt.Sscanf(line, "%s %d %d %d", &f, &n, &value, &state); err != nil {
			t.Fatal(err)
		}
		want, s, w := model(f, n)
		if !w {
			s = 99
		}
		if byte(value) != want || byte(state) != s {
			t.Fatal("independent model vs GCC", line, want, s)
		}
	}
	functions := []string{"while_post", "do_post", "for_post", "choose", "choose_else", "for_continue", "do_continue", "while_call"}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseHIRGrace: mode != "ordinary", UseGrace: mode != "ordinary", OptSize: mode == "grace-size", ContractOpt: true})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + steps.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", steps.MIR2Module.Dump())
		save(mode+"/target.a80", steps.Assembly)
		save(mode+"/target.bin", a.Binary)
		var observations []observation
		for _, fn := range functions {
			max := 32
			if fn == "choose" || fn == "choose_else" {
				max = 255
			}
			for n := 0; n <= max; n++ {
				want, state, write := model(fn, n)
				for label, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
					vm := mir2.NewVM(m)
					vm.MaxSteps = 100000
					vm.EnsureHeap(65536)
					p, e := vm.ResolveSymbol("state")
					if e != nil {
						t.Fatal(e)
					}
					vm.WriteHeap(p, 99)
					before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
					var es []mir2.VMDataAccess
					vm.ObserveDataAccess = func(v mir2.VMDataAccess) {
						if v.Addr == p {
							es = append(es, v)
						}
					}
					v, e := vm.Call(fn, []mir2.Value{{I: int64(n)}})
					if e != nil || len(v) != 1 || byte(v[0].I) != want {
						t.Fatal(mode, label, fn, n, v, want, e)
					}
					if write {
						before[p] = state
						if len(es) != 1 || es[0].Op != mir2.OpStore || es[0].Value != state {
							t.Fatal("VM ordered state", mode, label, fn, n, es, state)
						}
					} else if len(es) != 0 {
						t.Fatal("unselected effect", es)
					}
					if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
						t.Fatal("VM RAM", mode, label, fn, n)
					}
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
				z.LoadMemory(0x4000, a.Binary)
				p := a.Symbols["state"]
				mem[p] = 99
				mem[0xeff0], mem[0xeff1] = 0, 0x70
				before := append([]byte(nil), mem...)
				r := emulator.Registers{PC: uint16(a.Symbols[fn]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777}
				f := steps.MIR2Module.FuncByName(fn)
				loc := assertParamLocation(f.Contract.Params[0], steps.Allocation)
				switch loc.Name {
				case "A":
					r.A = byte(n)
				case "C":
					r.BC = r.BC&0xff00 | uint16(n)
				case "B":
					r.BC = r.BC&0xff | uint16(n)<<8
				default:
					t.Fatal("ABI", loc)
				}
				z.SetRegisters(r)
				minSP := uint16(0xeff0)
				var es []mir2.VMDataAccess
				done := false
				for i := 0; i < 100000; i++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					for _, v := range lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters()) {
						if v.Addr == int64(p) {
							es = append(es, v)
						}
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("SP", fn, n)
					}
				}
				if !done || z.GetRegisters().A != want {
					t.Fatal("native result", mode, fn, n, z.GetRegisters(), want)
				}
				if write {
					before[p] = state
					if len(es) != 1 || es[0].Op != mir2.OpStore || es[0].Value != state {
						t.Fatal("native ordered state", mode, fn, n, es, state)
					}
				} else if len(es) != 0 {
					t.Fatal("native unselected effect", es)
				}
				copy(before[minSP:0xeff0], mem[minSP:0xeff0])
				for _, l := range a.Listing {
					if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
						for j := range l.Bytes {
							before[l.Address+j] = mem[l.Address+j]
						}
					}
				}
				if !bytes.Equal(before, mem) {
					for j, b := range mem {
						if b != before[j] {
							t.Fatal("native RAM", mode, fn, n, j)
						}
					}
				}
				if len(z.GetOutput()) != 0 {
					t.Fatal("output")
				}
				observations = append(observations, observation{fn, n, want, mem[p]})
			}
		}
		save(mode+"/observations.json", observations)
		t.Log(mode, len(observations), "raw/final/native sequenced controls RAM/code/SP PASS")
	}
}
