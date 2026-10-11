package pipeline

import (
	"bytes"
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

// Book v7 §§3.1-3.4 and chapter8: typed modular arithmetic, signed ordering, structs.
// All26 original callables and assertions stay in the source module.
func TestOriginalWidemathNativeModel(t *testing.T) {
	const path = "../../../examples/nanz/widemath.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/widemath-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}

	var spec struct {
		SourceSHA256 string `json:"source_sha256"`
	}
	if e = json.Unmarshal(frozen, &spec); e != nil || spec.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("source freeze", e)
	}
	calls := widemathCalls(os.Getenv("MINZ_WIDEMATH_EXHAUSTIVE") == "1")

	save := func(name string, v any) {
		dir := os.Getenv("MINZ_WIDEMATH_ARTIFACTS")
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
		for _, hf := range h.Funcs {
			entry := hf.Name
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
			if steps.MIR2Module.FuncByName(c.Entry) == nil {
				t.Fatalf("original callable pruned: %s", c.Entry)
			}

			for label, m := range map[string]*mir2.Module{"raw": rawModule, "final": steps.MIR2Module} {
				vm := mir2.NewVM(m)
				var args []mir2.Value
				for _, a := range c.Args {
					arg := mir2.Value{I: a}
					if c.Entry == "op_mul" || c.Entry == "op_add" || c.Entry == "op_sub" {
						arg = vm.AllocHeap([]byte{byte(a)})
					}
					args = append(args, arg)
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
					got := v[j].I
					if c.Entry == "op_add" || c.Entry == "op_sub" {
						got = int64(vm.ReadHeap(got, 1)[0])
					}
					if got != w {
						t.Fatalf("%s %s call%d %+v got%v want%v", mode, label, i, c, v, wants)
					}
				}
				wantEvents := widemathMemoryValues(c)
				if len(events) != len(wantEvents) {
					t.Fatalf("%s %s memory traffic %s got%v want%v", mode, label, c.Entry, events, wantEvents)
				}
				for j, want := range wantEvents {
					if events[j].Op != want.Op || events[j].Value != want.Value {
						t.Fatalf("%s %s memory event%d got%v want%v", mode, label, j, events[j], want)
					}
				}
				if c.Entry == "newtype_area" && (events[0].Addr != events[2].Addr || events[1].Addr != events[3].Addr || events[0].Addr == events[1].Addr) {
					t.Fatal("newtype memory alias", events)
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
				if c.Entry == "op_mul" || c.Entry == "op_add" || c.Entry == "op_sub" {
					p := 0x6000 + j*16
					mem[p] = byte(arg)
					v = uint16(p)
				}
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
					mem[p] = byte(v)
					if mir2.NativeSpillWidth(f.Contract.Params[j].Ty) > 1 {
						mem[p+1] = byte(v >> 8)
					}
				}
			}
			z.SetRegisters(regs)
			before := append([]byte(nil), mem...)
			minSP := uint16(0xeff0)
			start := z.GetCycles()
			var nativeEvents []mir2.VMDataAccess
			done := false
			for j := 0; j < 1000000; j++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack bounds", c)
				}
				pc := z.GetPC()
				op := mem[pc]
				if op >= 0x46 && op <= 0x7e && op&7 == 6 && op != 0x76 {
					addr := z.GetRegisters().HL
					nativeEvents = append(nativeEvents, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(addr), Value: mem[addr]})
				}
				if op >= 0x70 && op <= 0x77 && op != 0x76 {
					r := z.GetRegisters()
					vals := []byte{byte(r.BC >> 8), byte(r.BC), byte(r.DE >> 8), byte(r.DE), byte(r.HL >> 8), byte(r.HL), 0, r.A}
					nativeEvents = append(nativeEvents, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(r.HL), Value: vals[op&7]})
				} else if op == 0x36 {
					addr := z.GetRegisters().HL
					nativeEvents = append(nativeEvents, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(addr), Value: mem[pc+1]})
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done {
				t.Fatal("native did not return", c)
			}
			wantEvents := widemathMemoryValues(c)
			if len(nativeEvents) != len(wantEvents) {
				t.Fatalf("%s native memory traffic %s got%v want%v", mode, c.Entry, nativeEvents, wantEvents)
			}
			for j, want := range wantEvents {
				if nativeEvents[j].Op != want.Op || nativeEvents[j].Value != want.Value {
					t.Fatalf("%s native memory event%d got%v want%v", mode, j, nativeEvents[j], want)
				}
			}
			if c.Entry == "newtype_area" && (nativeEvents[0].Addr != 0xefed || nativeEvents[1].Addr != 0xefec || nativeEvents[2].Addr != 0xefed || nativeEvents[3].Addr != 0xefec) {
				t.Fatal("native newtype alias/layout", nativeEvents)
			}
			if (c.Entry == "op_mul" || c.Entry == "op_add" || c.Entry == "op_sub") && (nativeEvents[0].Addr != 0x6000 || nativeEvents[1].Addr != 0x6010) {
				t.Fatal("native input addresses", nativeEvents)
			}

			if c.Entry == "op_add" || c.Entry == "op_sub" {
				regs := z.GetRegisters()
				pointer := regs.HL
				switch mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) {
				case "A":
					pointer = uint16(regs.A)
				case "DE":
					pointer = regs.DE
				case "BC":
					pointer = regs.BC
				}
				if int64(mem[pointer]) != c.Expected {
					t.Fatalf("%s native struct %s %v returnedptr%x byte%d want%d", mode, c.Entry, c.Args, pointer, mem[pointer], c.Expected)
				}
			} else if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Args: c.Args, Expected: c.Expected, ExpectedMulti: c.ExpectedMulti}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
				t.Fatalf("%s native call%d %+v %v", mode, i, c, e)
			}

			// Mask only the exact observed stack extent and declared spill
			// homes; compare every other byte of the full 64KiB image.
			copy(before[int(minSP):0xeff0], mem[int(minSP):0xeff0])
			for p := range allowedSpill {
				before[p] = mem[p]
			}
			if !bytes.Equal(mem, before) {
				for p := range mem {
					if mem[p] != before[p] {
						t.Fatalf("unexpected memory %s call%d %x %x->%x minSP%x", mode, i, p, before[p], mem[p], minSP)
					}
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
		t.Logf("%s %d original Widemath calls PASS; %d source assertions retained", mode, len(calls), len(h.Asserts))
	}
}

type widemathCall struct {
	Entry         string
	Args          []int64
	Expected      int64
	ExpectedMulti []int64
}

func widemathCalls(exhaustive bool) []widemathCall {
	var out []widemathCall
	add := func(n string, w int, args ...int) {
		a := []int64{}
		for _, v := range args {
			a = append(a, int64(v))
		}
		out = append(out, widemathCall{Entry: n, Args: a, Expected: int64(w)})
	}
	bytes := []int{0, 1, 2, 3, 7, 15, 16, 63, 64, 127, 128, 129, 200, 254, 255}
	words := []int{0, 1, 2, 127, 128, 255, 256, 257, 511, 32767, 32768, 32769, 65280, 65534, 65535}
	unaryB, unaryW, pairB := bytes, words, bytes
	if exhaustive {
		unaryB = []int{}
		for i := 0; i < 256; i++ {
			unaryB = append(unaryB, i)
		}
		unaryW = []int{}
		for i := 0; i < 65536; i++ {
			unaryW = append(unaryW, i)
		}
		pairB = unaryB
	}
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	sign := func(x int) int {
		if x < 0 {
			return 255
		}
		if x > 0 {
			return 1
		}
		return 0
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	for _, x := range unaryB {
		add("abs8", abs(int(int8(x))), x)
		add("sign8", sign(int(int8(x))), x)
		add("negate8", int(byte(-x)), x)
	}
	for _, x := range unaryW {
		add("abs16", abs(int(int16(x))), x)
		add("sign16", sign(int(int16(x))), x)
	}
	for _, a := range pairB {
		for _, b := range pairB {
			add("min8", min(a, b), a, b)
			add("max8", max(a, b), a, b)
			add("abs_diff8", abs(a-b), a, b)
			add("sat_add8", min(a+b, 255), a, b)
			add("sat_sub8", max(a-b, 0), a, b)
			add("area", a*b, a, b)
			add("op_mul_u8_u8", a*b, a, b)
			add("newtype_area", a*b, a, b)
		}
	}
	for _, a := range words {
		for _, b := range words {
			add("min16", min(a, b), a, b)
			add("max16", max(a, b), a, b)
			add("abs_diff16", abs(a-b), a, b)
			add("sat_add16", min(a+b, 65535), a, b)
			add("sat_sub16", max(a-b, 0), a, b)
		}
	}
	for _, x := range unaryB {
		for _, lo := range bytes {
			for _, hi := range bytes {
				if lo > hi {
					continue
				}
				w := x
				if w < lo {
					w = lo
				}
				if w > hi {
					w = hi
				}
				add("clamp8", w, x, lo, hi)
			}
		}
	}
	for _, x := range words {
		for _, lo := range words {
			for _, hi := range words {
				if lo > hi {
					continue
				}
				w := x
				if w < lo {
					w = lo
				}
				if w > hi {
					w = hi
				}
				add("clamp16", w, x, lo, hi)
			}
		}
	}
	for _, a := range bytes {
		for _, b := range bytes {
			add("op_mul", a*b, a, b)
			add("op_add", int(byte(a+b)), a, b)
			add("op_sub", int(byte(a-b)), a, b)
			for _, mix := range unaryB {
				add("brightness_blend", (a*(255-mix)+b*mix)/256, a, b, mix)
			}
		}
	}
	coords := []int{0, 1, 127, 128, 254, 255}
	for _, x1 := range coords {
		for _, y1 := range coords {
			for _, x2 := range coords {
				for _, y2 := range coords {
					add("pixel_distance", abs(x1-x2)*abs(y1-y2), x1, y1, x2, y2)
				}
			}
		}
	}
	add("big_area", 40000)
	return out
}

// Source W8 field operations: input reads retain order; newtype construction
// writes two distinct local bytes before the two operator reads.
func widemathMemoryValues(c widemathCall) []mir2.VMDataAccess {
	ev := func(op mir2.Op, v int64) mir2.VMDataAccess { return mir2.VMDataAccess{Op: op, Value: byte(v)} }
	switch c.Entry {
	case "op_mul":
		return []mir2.VMDataAccess{ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1])}
	case "op_add", "op_sub":
		return []mir2.VMDataAccess{ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1]), ev(mir2.OpStore, c.Expected)}
	case "newtype_area":
		return []mir2.VMDataAccess{ev(mir2.OpStore, c.Args[0]), ev(mir2.OpStore, c.Args[1]), ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1])}
	}
	return nil
}
