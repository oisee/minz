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
				// N1a (#251) scalarizes the 1-byte W8 return to a by-value A
				// return in the optimized module; the unoptimized raw module still
				// returns the address of a public result buffer. The two models
				// differ in shape, never in the verified value.
				scalarReturn := label == "final"
				for j, w := range wants {
					got := v[j].I
					if (c.Entry == "op_add" || c.Entry == "op_sub") && !scalarReturn {
						got = int64(vm.ReadHeap(got, 1)[0])
					}
					if got != w {
						t.Fatalf("%s %s call%d %+v got%v want%v", mode, label, i, c, v, wants)
					}
				}
				if err := widemathCheckMemory(c, label, events); err != nil {
					t.Fatalf("%s %s %s: %v", mode, label, c.Entry, err)
				}

			}
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			mem[0xe000], mem[0xeff2] = 0x9b, 0xad
			regs := emulator.Registers{PC: uint16(assembled.Symbols["oracle_"+c.Entry]), SP: 0xeff0, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999, A: 0x42}
			f := steps.MIR2Module.FuncByName(c.Entry)
			nativeArgs := append([]int64(nil), c.Args...)
			// N1a (#251) returns the 1-byte W8 result by value in A, so there is no
			// public result-buffer argument any more. The seeded bytes below stay:
			// a stray store into the old result window would fail the full-image
			// comparison at the end of the call.
			if len(f.Contract.StructReturnBuffers) != 0 {
				t.Fatalf("W8 result is no longer by-value: %+v", f.Contract.StructReturnBuffers)
			}
			mem[0x61ff], mem[0x6200], mem[0x6201] = 0x9b, 0xa5, 0xad
			for j, arg := range nativeArgs {
				loc := assertParamLocation(f.Contract.Params[j], steps.Allocation)
				v := uint16(arg)
				if j < len(c.Args) && (c.Entry == "op_mul" || c.Entry == "op_add" || c.Entry == "op_sub") {
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
				for _, ev := range widemathNativeByteAccess(mem, pc, z.GetRegisters()) {
					// Compiler spill homes are not source object traffic. N1a's
					// by-value W8 return keeps the byte in an allocated home that
					// the PBQP allocator spills for this function, and the mir2 VM
					// has no spill equivalent to observe.
					if !allowedSpill[int(ev.Addr)] {
						nativeEvents = append(nativeEvents, ev)
					}
				}

				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done {
				t.Fatal("native did not return", c)
			}
			if err := widemathCheckMemory(c, "final", nativeEvents); err != nil {
				t.Fatalf("%s native %s: %v", mode, c.Entry, err)
			}

			if (c.Entry == "op_mul" || c.Entry == "op_add" || c.Entry == "op_sub") && (nativeEvents[0].Addr != 0x6000 || nativeEvents[1].Addr != 0x6010) {
				t.Fatal("native input addresses", nativeEvents)
			}

			// op_add/op_sub now return the byte itself, so they are checked through
			// the same return-ABI path as every other scalar entry. The A register is
			// seeded with 0x42 before the call, so a stale pointer-style return would
			// not accidentally match (a+b)&0xFF.
			if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Args: c.Args, Expected: c.Expected, ExpectedMulti: c.ExpectedMulti}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
				t.Fatalf("%s native call%d %+v %v", mode, i, c, e)
			}
			if c.Entry == "op_add" || c.Entry == "op_sub" {
				if f.Contract.Returns[0].Ty != mir2.TyU8 || mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) != "A" {
					t.Fatalf("W8 result is not a by-value A return: %+v", f.Contract.Returns[0])
				}
				if len(nativeEvents) != 2 {
					t.Fatalf("by-value W8 result still publishes an output store: %v", nativeEvents)
				}
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
//
// kind distinguishes the two modules under test. "raw" is the unoptimized
// lowering, whose op_add/op_sub still materialize the W8 result through an
// owning temporary and a public result-buffer store (5 events, pointer return).
// "final" is the optimized module, where N1a (#251) scalarizes the 1-byte W8
// return to a by-value A return, leaving exactly the two ordered field reads
// (2 events, value return) — which is also what the native binary performs.
func widemathMemoryValues(c widemathCall, kind string) []mir2.VMDataAccess {
	ev := func(op mir2.Op, v int64) mir2.VMDataAccess { return mir2.VMDataAccess{Op: op, Value: byte(v)} }
	switch c.Entry {
	case "op_mul":
		return []mir2.VMDataAccess{ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1])}
	case "op_add", "op_sub":
		if kind == "raw" {
			return []mir2.VMDataAccess{ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1]), ev(mir2.OpStore, c.Expected), ev(mir2.OpLoad, c.Expected), ev(mir2.OpStore, c.Expected)}
		}
		return []mir2.VMDataAccess{ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1])}
	case "newtype_area":
		return []mir2.VMDataAccess{ev(mir2.OpStore, c.Args[0]), ev(mir2.OpStore, c.Args[1]), ev(mir2.OpLoad, c.Args[0]), ev(mir2.OpLoad, c.Args[1])}
	}
	return nil
}

// Constructor stores are private and may be scheduled independently. The two
// ordered operator reads must still read distinct objects initialized correctly.
func widemathCheckMemory(c widemathCall, kind string, got []mir2.VMDataAccess) error {
	want := widemathMemoryValues(c, kind)
	if len(got) != len(want) {
		return fmt.Errorf("memory traffic got%v want%v", got, want)
	}
	if c.Entry == "newtype_area" {
		if got[0].Op != mir2.OpStore || got[1].Op != mir2.OpStore || got[0].Addr == got[1].Addr {
			return fmt.Errorf("distinct constructor stores: %v", got)
		}
		for j := 0; j < 2; j++ {
			read := got[j+2]
			if read.Op != mir2.OpLoad || read.Value != byte(c.Args[j]) {
				return fmt.Errorf("ordered W8 read: %v", got)
			}
			found := false
			for _, store := range got[:2] {
				if store.Addr == read.Addr && store.Value == read.Value {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("uninitialized W8 read: %v", got)
			}
		}
		if got[2].Addr == got[3].Addr {
			return fmt.Errorf("aliased W8 objects: %v", got)
		}
		return nil
	}
	for j, w := range want {
		if got[j].Op != w.Op || got[j].Value != w.Value {
			return fmt.Errorf("memory event%d got%v want%v", j, got[j], w)
		}
	}
	if c.Entry == "op_add" || c.Entry == "op_sub" {
		if got[0].Addr == got[1].Addr {
			return fmt.Errorf("aliased W8 inputs: %v", got)
		}
		if kind == "raw" {
			if got[2].Addr != got[3].Addr || got[4].Addr == got[2].Addr || got[2].Addr == got[0].Addr || got[2].Addr == got[1].Addr || got[4].Addr == got[0].Addr || got[4].Addr == got[1].Addr {
				return fmt.Errorf("private snapshot/public output alias: %v", got)
			}
		}
	}
	return nil
}

// Decode byte data accesses, excluding instruction fetch, stack, word spills.
// Explicit forms cover the original W8 paths; index forms retain displacement.
func widemathNativeByteAccess(mem []byte, pc uint16, r emulator.Registers) []mir2.VMDataAccess {
	op := mem[pc]
	addr := r.HL
	vals := []byte{byte(r.BC >> 8), byte(r.BC), byte(r.DE >> 8), byte(r.DE), byte(r.HL >> 8), byte(r.HL), 0, r.A}
	event := func(op mir2.Op, addr uint16, v byte) []mir2.VMDataAccess {
		return []mir2.VMDataAccess{{Op: op, Addr: int64(addr), Value: v}}
	}
	if op == 0xdd || op == 0xfd {
		addr = r.IX
		if op == 0xfd {
			addr = r.IY
		}
		op = mem[pc+1]
		addr = uint16(int(addr) + int(int8(mem[pc+2])))
		pc += 2
	}
	if op >= 0x46 && op <= 0x7e && op&7 == 6 && op != 0x76 {
		return event(mir2.OpLoad, addr, mem[addr])
	}
	if op >= 0x70 && op <= 0x77 && op != 0x76 {
		return event(mir2.OpStore, addr, vals[op&7])
	}
	if op == 0x36 {
		return event(mir2.OpStore, addr, mem[pc+1])
	}
	switch op {
	case 0x02:
		return event(mir2.OpStore, r.BC, r.A)
	case 0x12:
		return event(mir2.OpStore, r.DE, r.A)
	case 0x0a:
		return event(mir2.OpLoad, r.BC, mem[r.BC])
	case 0x1a:
		return event(mir2.OpLoad, r.DE, mem[r.DE])
	}
	return nil
}

func TestWidemathMemoryObserverContracts(t *testing.T) {
	c := widemathCall{Entry: "newtype_area", Args: []int64{7, 19}}
	events := []mir2.VMDataAccess{{Op: mir2.OpStore, Addr: 20, Value: 19}, {Op: mir2.OpStore, Addr: 21, Value: 7}, {Op: mir2.OpLoad, Addr: 21, Value: 7}, {Op: mir2.OpLoad, Addr: 20, Value: 19}}
	if err := widemathCheckMemory(c, "final", events); err != nil {
		t.Fatal("independent private store schedule", err)
	}
	for _, name := range []string{"missing-store", "swapped-reads", "wrong-constructor", "aliased-inputs"} {
		bad := append([]mir2.VMDataAccess(nil), events...)
		switch name {
		case "missing-store":
			bad = bad[1:]
		case "swapped-reads":
			bad[2], bad[3] = bad[3], bad[2]
		case "wrong-constructor":
			bad[0].Value = 18
		case "aliased-inputs":
			bad[3].Addr = bad[2].Addr
		}
		if err := widemathCheckMemory(c, "final", bad); err == nil {
			t.Fatal("accepted", name, bad)
		}
	}
	// The unoptimized raw module still materializes the W8 result through an
	// owning temporary and a public result-buffer store.
	c = widemathCall{Entry: "op_add", Args: []int64{250, 19}, Expected: 13}
	raw := []mir2.VMDataAccess{{Op: mir2.OpLoad, Addr: 0x6000, Value: 250}, {Op: mir2.OpLoad, Addr: 0x6010, Value: 19}, {Op: mir2.OpStore, Addr: 20, Value: 13}, {Op: mir2.OpLoad, Addr: 20, Value: 13}, {Op: mir2.OpStore, Addr: 0x6200, Value: 13}}
	if err := widemathCheckMemory(c, "raw", raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"no-snapshot", "wrong-snapshot", "public-output-alias", "snapshot-input-alias"} {
		bad := append([]mir2.VMDataAccess(nil), raw...)
		switch name {
		case "no-snapshot":
			bad = append(bad[:3], bad[4:]...)
		case "wrong-snapshot":
			bad[3].Value = 12
		case "public-output-alias":
			bad[4].Addr = 20
		case "snapshot-input-alias":
			bad[2].Addr = 0x6000
			bad[3].Addr = 0x6000
		}
		if err := widemathCheckMemory(c, "raw", bad); err == nil {
			t.Fatal("accepted", name, bad)
		}
	}
	// The optimized module scalarizes the 1-byte W8 return: only the two ordered
	// field reads remain, and that is exactly what the native binary performs.
	byValue := []mir2.VMDataAccess{{Op: mir2.OpLoad, Addr: 0x6000, Value: 250}, {Op: mir2.OpLoad, Addr: 0x6010, Value: 19}}
	if err := widemathCheckMemory(c, "final", byValue); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"extra-output-store", "reordered-reads", "wrong-value", "aliased-inputs"} {
		bad := append([]mir2.VMDataAccess(nil), byValue...)
		switch name {
		case "extra-output-store":
			bad = append(bad, mir2.VMDataAccess{Op: mir2.OpStore, Addr: 0x6200, Value: 13})
		case "reordered-reads":
			bad[0], bad[1] = bad[1], bad[0]
		case "wrong-value":
			bad[1].Value = 18
		case "aliased-inputs":
			bad[1].Addr = bad[0].Addr
		}
		if err := widemathCheckMemory(c, "final", bad); err == nil {
			t.Fatal("accepted", name, bad)
		}
	}
	// The raw model must not be accepted for the optimized module and vice versa.
	if err := widemathCheckMemory(c, "final", raw); err == nil {
		t.Fatal("raw traffic accepted for the scalarized module")
	}
	if err := widemathCheckMemory(c, "raw", byValue); err == nil {
		t.Fatal("scalarized traffic accepted for the raw module")
	}
	mem := make([]byte, 65536)
	r := emulator.Registers{A: 77, BC: 0x6100, DE: 0x6200, HL: 0x6300, IX: 0x6400, IY: 0x6500}
	for _, tc := range []struct {
		code  []byte
		addr  uint16
		op    mir2.Op
		value byte
	}{{[]byte{0x12}, 0x6200, mir2.OpStore, 77}, {[]byte{0x02}, 0x6100, mir2.OpStore, 77}, {[]byte{0x1a}, 0x6200, mir2.OpLoad, 91}, {[]byte{0x0a}, 0x6100, mir2.OpLoad, 91}, {[]byte{0xdd, 0x77, 0xff}, 0x63ff, mir2.OpStore, 77}, {[]byte{0xfd, 0x7e, 0x02}, 0x6502, mir2.OpLoad, 91}, {[]byte{0xdd, 0x36, 0xff, 0x5b}, 0x63ff, mir2.OpStore, 91}} {
		copy(mem[0x8000:], tc.code)
		mem[tc.addr] = 91
		got := widemathNativeByteAccess(mem, 0x8000, r)
		if len(got) != 1 || got[0].Addr != int64(tc.addr) || got[0].Op != tc.op || got[0].Value != tc.value {
			t.Fatal("native decoder", tc, got)
		}
	}
	for _, op := range []byte{0xc5, 0xc1, 0xcd, 0xc9, 0x76} {
		mem[0x8000] = op
		if got := widemathNativeByteAccess(mem, 0x8000, r); len(got) != 0 {
			t.Fatal("stack/fetch counted as data", op, got)
		}
	}
}
