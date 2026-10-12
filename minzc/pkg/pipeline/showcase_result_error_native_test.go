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

// Book v7 §16.2 encodes tagged Result as tag high / payload low.
// Expected overflow, tag and payload values are host computations.
func TestOriginalResultErrorNativeModel(t *testing.T) {
	const path = "../../../examples/nanz/13_result_error.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/result-error-native/workload.json")
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
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			want := int64(a + b)
			if a+b > 255 {
				want = 0x0101
			}
			calls = append(calls, call{Entry: "safe_add", Args: []int64{int64(a), int64(b)}, Expected: want, ExpectedMulti: []int64{want}})
		}
	}
	for word := 0; word < 65536; word++ {
		ok := int64(0)
		if word>>8 == 0 {
			ok = 1
		}
		calls = append(calls, call{Entry: "is_ok", Args: []int64{int64(word)}, Expected: ok, ExpectedMulti: []int64{ok}}, call{Entry: "unwrap", Args: []int64{int64(word)}, Expected: int64(word & 255), ExpectedMulti: []int64{int64(word & 255)}})
	}
	for _, wrapper := range []struct {
		Name string
		Want int64
	}{{"test_add_ok_tag", 0}, {"test_add_ok_val", 30}, {"test_add_overflow_tag", 1}, {"test_add_overflow_code", 1}, {"test_is_ok_pass", 1}, {"test_is_ok_fail", 0}} {
		calls = append(calls, call{Entry: wrapper.Name, Expected: wrapper.Want, ExpectedMulti: []int64{wrapper.Want}})
	}

	save := func(name string, v any) {
		dir := os.Getenv("MINZ_RESULT_ERROR_ARTIFACTS")
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
		for _, entry := range []string{"safe_add", "is_ok", "unwrap", "test_add_ok_tag", "test_add_ok_val", "test_add_overflow_tag", "test_add_overflow_code", "test_is_ok_pass", "test_is_ok_fail"} {
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
				case "HL":
					regs.HL = v
				case "DE":
					regs.DE = v
				case "BC":
					regs.BC = v
				case "IX":
					regs.IX = v
				case "IY":
					regs.IY = v
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
		save(mode+"/evidence.json", map[string]any{"source_sha256": spec.SourceSHA256, "options": Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size"}, "native_calls": len(calls), "source_assertions_retained": len(h.Asserts), "entries": metrics, "assembled_binary_sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))})
		t.Logf("%s %d original tagged-result calls PASS; %d source assertions retained", mode, len(calls), len(h.Asserts))
	}
}

// Generated helpers are not exported in final native code: ordinary callers
// inline them. Check their complete raw meaning without inventing an export.
func TestOriginalResultGeneratedHelpersRaw(t *testing.T) {
	const path = "../../../examples/nanz/13_result_error.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.Parse(string(src), path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/result-error-native/workload.json")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		SourceSHA256 string `json:"source_sha256"`
	}
	if e = json.Unmarshal(frozen, &spec); e != nil || spec.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("source freeze", e)
	}

	raw := hir.LowerModule(h)
	vm := mir2.NewVM(raw)
	vm.MaxSteps = 10000000
	for word := 0; word < 65536; word++ {
		for _, entry := range []string{"__tag", "__payload"} {
			want := word & 255
			if entry == "__tag" {
				want = word >> 8
			}
			v, e := vm.Call(entry, []mir2.Value{{I: int64(word)}})
			if e != nil || len(v) != 1 || v[0].I != int64(want) {
				t.Fatal(entry, word, v, e)
			}
		}
	}
	t.Log("131072 generated helper calls PASS raw VM; native meaning checked via unchanged is_ok/unwrap/wrappers")
}
