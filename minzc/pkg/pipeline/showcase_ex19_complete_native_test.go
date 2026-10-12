package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Book v7 §3.1 defines byte width; chapter6 defines n..1 range, §7.3 tuples.
// Expected arithmetic and tuple order are computed independently here.
func TestOriginalEx19RemainingNativeModels(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-13/ex19_assert_forms.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/showcase-ex19-complete/workload.json")
	if e != nil {
		t.Fatal(e)
	}

	var spec struct {
		SourceSHA256 string `json:"source_sha256"`
	}
	if e = json.Unmarshal(frozen, &spec); e != nil || spec.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("source freeze", e)
	}
	type call struct {
		Entry         string
		Args          []int64
		Expected      int64
		ExpectedMulti []int64
	}
	var calls []call
	for x := 0; x < 256; x++ {
		for _, y := range []int{0, 1, 127, 128, 254, 255} {
			lo, hi := x, y
			if lo > hi {
				lo, hi = hi, lo
			}
			calls = append(calls, call{Entry: "abs_diff", Args: []int64{int64(x), int64(y)}, Expected: int64(hi - lo)}, call{Entry: "minmax", Args: []int64{int64(x), int64(y)}, ExpectedMulti: []int64{int64(lo), int64(hi)}}, call{Entry: "swap", Args: []int64{int64(x), int64(y)}, ExpectedMulti: []int64{int64(y), int64(x)}}, call{Entry: "lambda_0", Args: []int64{int64(x), int64(y)}, Expected: int64(byte(x + y))})
		}
		var sum byte
		for i := x; i > 0; i-- {
			sum += byte(i)
		}
		calls = append(calls, call{Entry: "sum_range", Args: []int64{int64(x)}, Expected: int64(sum)})
		for _, lo := range []int{0, 1, 127, 128, 254, 255} {
			for _, hi := range []int{0, 1, 127, 128, 254, 255} {
				if lo > hi {
					continue
				}
				v := x
				if v < lo {
					v = lo
				}
				if v > hi {
					v = hi
				}
				calls = append(calls, call{Entry: "clamp", Args: []int64{int64(x), int64(lo), int64(hi)}, Expected: int64(v)})
			}
		}
	}
	a, b := byte(0), byte(1)
	for n := 0; n < 15; n++ {
		calls = append(calls, call{Entry: "fib", Args: []int64{int64(n)}, Expected: int64(a)})
		a, b = b, a+b
	}

	save := func(name string, v any) {
		dir := os.Getenv("MINZ_EX19_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		default:
			data, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", src)
	save("calls.json", calls)
	for _, mode := range []string{"default", "grace-size", "grace-speed"} {
		parse := func() *hir.Module {
			h, e := nanz.Parse(string(src), path)
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		rawModule := hir.LowerModule(parse())
		h := parse()
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(mode, e)
		}
		save(mode+"/target.a80", steps.Assembly)
		save(mode+"/raw.mir", rawModule.Dump())
		save(mode+"/final.mir", steps.MIR2Module.Dump())
		save(mode+"/allocation.json", steps.Allocation)
		// Add only invocation drivers; keep every original function and assertion.
		allowedSpill := map[int]bool{}
		var drivers strings.Builder
		for _, entry := range []string{"abs_diff", "clamp", "fib", "sum_range", "minmax", "swap"} {
			drivers.WriteString(fmt.Sprintf("\noracle_%s:\n CALL %s\n RET\n", entry, entry))
		}

		assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly + drivers.String())
		if e != nil || len(assembled.Errors) > 0 {
			t.Fatal(e, assembled.Errors)
		}
		if len(assembled.Binary) > 0x6000 {
			t.Fatal("driver/image stack overlap")
		}
		save(mode+"/target.bin", assembled.Binary)
		for _, l := range assembled.Listing {
			if strings.HasPrefix(l.Label, "_spill_") {
				for j := range l.Bytes {
					allowedSpill[l.Address+j] = true
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
			t.Fatal(e)
		}
		type measure struct {
			Calls  int
			TotalT uint64
			MinSP  uint16
		}
		metrics := map[string]measure{}
		for i, c := range calls {
			if c.Entry == "lambda_0" {
				vm := mir2.NewVM(rawModule)
				v, e := vm.Call(c.Entry, []mir2.Value{{I: c.Args[0]}, {I: c.Args[1]}})
				if e != nil || len(v) != 1 || v[0].I != c.Expected {
					t.Fatal("raw generated callback", c, v, e)
				}
				if steps.MIR2Module.FuncByName(c.Entry) != nil {
					t.Fatal("unexpected generated callback final admission")
				}
				continue
			}
			for label, m := range map[string]*mir2.Module{"raw": rawModule, "final": steps.MIR2Module} {
				vm := mir2.NewVM(m)
				var args []mir2.Value
				for _, a := range c.Args {
					args = append(args, mir2.Value{I: a})
				}
				var events []mir2.VMDataAccess
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
				v, e := vm.Call(c.Entry, args)

				if e != nil {
					t.Fatal(mode, label, c, e)
				}
				wants := c.ExpectedMulti
				if len(wants) == 0 {
					wants = []int64{c.Expected}
				}
				if len(v) != len(wants) {
					t.Fatal("return shape", c, v)
				}
				for j, w := range wants {
					if v[j].I != w {
						t.Fatalf("%s %s call%d %+v got%v want%v", mode, label, i, c, v, wants)
					}
				}
				if len(events) != 0 {
					t.Fatalf("unexpected pure memory traffic %s %s %v", label, c.Entry, events)
				}

			}
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			mem[0xe000], mem[0xeff2] = 0x9b, 0xad
			regs := emulator.Registers{PC: uint16(assembled.Symbols["oracle_"+c.Entry]), SP: 0xeff0, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999, A: 0x42}
			f := steps.MIR2Module.FuncByName(c.Entry)
			for j, arg := range c.Args {
				loc := assertParamLocation(f.Contract.Params[j], steps.Allocation)
				v := uint16(arg)
				switch loc.Name {
				case "A":
					regs.A = byte(v)
				case "B":
					regs.BC = regs.BC&255 | v<<8
				case "C":
					regs.BC = regs.BC&0xff00 | v
				case "D":
					regs.DE = regs.DE&255 | v<<8
				case "E":
					regs.DE = regs.DE&0xff00 | v
				case "H":
					regs.HL = regs.HL&255 | v<<8
				case "L":
					regs.HL = regs.HL&0xff00 | v
				case "IXH":
					regs.IX = regs.IX&255 | v<<8
				case "IXL":
					regs.IX = regs.IX&0xff00 | v
				case "IYH":
					regs.IY = regs.IY&255 | v<<8
				case "IYL":
					regs.IY = regs.IY&0xff00 | v
				default:
					if loc.Kind != mir2.LocMem {
						t.Fatal("unhandled entry ABI", loc)
					}
					p := assembled.Symbols[mir2.Z80SpillLabel(f.Name, f.Contract.Params[j].Reg)]
					mem[p] = byte(arg)
				}
			}
			z.SetRegisters(regs)
			before := append([]byte(nil), mem...)
			minSP := uint16(0xeff0)
			start := z.GetCycles()
			done := false
			for j := 0; j < 1000000; j++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack bounds", c)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done {
				t.Fatal("native did not return", c)
			}
			if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Args: c.Args, Expected: c.Expected, ExpectedMulti: c.ExpectedMulti}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
				t.Fatalf("%s native call%d %+v %v", mode, i, c, e)
			}
			for p := 0; p < 65536; p++ {
				if mem[p] == before[p] {
					continue
				}
				if p >= int(minSP) && p < int(0xeff0) {
					continue
				}
				spill := allowedSpill[p]
				if !spill {
					t.Fatalf("unexpected memory %s call%d %x %x->%x minSP%x", mode, i, p, before[p], mem[p], minSP)
				}
			}

			metric := metrics[c.Entry]
			metric.Calls++
			metric.TotalT += uint64(z.GetCycles() - start)
			if metric.MinSP == 0 || minSP < metric.MinSP {
				metric.MinSP = minSP
			}
			metrics[c.Entry] = metric
			if i < 3 {
				t.Logf("%s %+v %dT native/raw/final model code/SP/RAM PASS", mode, c, z.GetCycles()-start)
			}
		}
		save(mode+"/evidence.json", map[string]any{"source_sha256": spec.SourceSHA256, "options": Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size"}, "native_calls": len(calls) - 1536, "raw_only_callback_calls": 1536, "source_assertions_retained": len(h.Asserts), "entries": metrics, "assembled_binary_sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))})
		t.Logf("%s %d workload calls PASS (1536 raw-only eliminated callback); %d source assertions retained", mode, len(calls), len(h.Asserts))
	}
}
