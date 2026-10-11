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
	"reflect"
	"strings"
	"testing"
)

type lanzAccess struct {
	Op      string `json:"op"`
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}
type lanzBaselineHost struct {
	ram                 []byte
	pos, len, node, str uint16
	events              []lanzAccess
}

func newLanzHost() *lanzBaselineHost {
	return &lanzBaselineHost{ram: make([]byte, 65536), node: 0x8000, str: 0x9000}
}
func (h *lanzBaselineHost) rd(p uint16) byte {
	v := h.ram[p]
	h.events = append(h.events, lanzAccess{"read", p, v})
	return v
}
func (h *lanzBaselineHost) wr(p uint16, v byte) {
	h.events = append(h.events, lanzAccess{"write", p, v})
	h.ram[p] = v
}
func lanzSpace(c byte) bool { return c == 32 || c == 9 || c == 10 || c == 13 }
func lanzDigit(c byte) bool { return c >= 48 && c <= 57 }
func lanzAtom(c byte) bool  { return c != 0 && !lanzSpace(c) && c != 40 && c != 41 }
func (h *lanzBaselineHost) cur() byte {
	if h.pos >= h.len {
		return 0
	}
	return h.rd(0xC000 + h.pos)
}
func (h *lanzBaselineHost) skip() {
	for lanzSpace(h.cur()) {
		h.pos++
	}
	if h.cur() == 59 {
		for h.cur() != 10 {
			if h.cur() == 0 {
				return
			}
			h.pos++
		}
		h.pos++
		h.skip()
	}
}
func (h *lanzBaselineHost) region(p, start uint16, n byte) bool {
	for i := byte(0); i < n; i++ {
		if h.rd(p+1+uint16(i)) != h.rd(0xC000+start+uint16(i)) {
			return false
		}
	}
	return true
}
func (h *lanzBaselineHost) intern(start uint16, n byte) uint16 {
	p := uint16(0x9000)
	for p < h.str {
		length := h.rd(p)
		if length == n && h.region(p, start, n) {
			return p
		}
		p += 1 + uint16(length)
	}
	p = h.str
	h.wr(h.str, n)
	for i := byte(0); i < n; i++ {
		v := h.rd(0xC000 + start + uint16(i))
		h.wr(h.str+1+uint16(i), v)
	}
	h.str += 1 + uint16(n)
	return p
}
func (h *lanzBaselineHost) makeNode(tag byte, d0, d1, d2 uint16) uint16 {
	p := h.node
	h.wr(p, tag)
	for j, v := range []uint16{d0, d1, d2} {
		h.wr(p+1+uint16(j*2), byte(v))
		h.wr(p+2+uint16(j*2), byte(v>>8))
	}
	h.wr(p+7, 0)
	h.node += 8
	return p
}
func (h *lanzBaselineHost) setD1(p, v uint16) { h.wr(p+3, byte(v)); h.wr(p+4, byte(v>>8)) }
func (h *lanzBaselineHost) parse() uint16 {
	h.skip()
	c := h.cur()
	if c == 0 {
		return 0
	}
	if c == 40 {
		h.pos++
		h.skip()
		if h.cur() == 41 {
			h.pos++
			return h.makeNode(2, 0, 0, 0)
		}
		first := h.parse()
		node := h.makeNode(2, first, 0, 0)
		prev := first
		h.skip()
		for h.cur() != 41 {
			if h.cur() == 0 {
				return node
			}
			next := h.parse()
			h.setD1(prev, next)
			prev = next
			h.skip()
		}
		h.pos++
		return node
	}
	start := h.pos
	for lanzAtom(h.cur()) {
		h.pos++
	}
	n := byte(h.pos - start)
	if n == 0 {
		return 0
	}
	if lanzDigit(h.rd(0xC000 + start)) {
		v := uint16(0)
		for k := byte(0); k < n; k++ {
			v = v*10 + uint16(h.rd(0xC000+start+uint16(k))) - 48
		}
		return h.makeNode(3, v, 0, 0)
	}
	s := h.intern(start, n)
	return h.makeNode(1, s, 0, 0)
}
func lanzBaselineInputs() []string {
	return []string{"", "()", "a", "a", "(a a)", "(fun add ((a u8) (b u8)) u8 (return (+ a b)))", "123", "65535", "65536", "12x", "(1 2 3)", "((a)(b)(c))", "; comment\na", ";one\n;two\n()", "(a ;comment\nb)", "(a", "((a)", ")", " a\t\r\n", "(alpha beta alpha)", "(a (b (c)))", "+", "-12", "0", "(123 alpha ())", "a\x00b", strings.Repeat("z", 255), strings.Repeat("q", 256), strings.Repeat("r", 257), "(a a a a)"}
}
func TestOriginalSelfLanzParserBaseline(t *testing.T) {
	path := "../../../examples/nanz/self_lanz_parser.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	factory := func() *hir.Module {
		m, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	dir := os.Getenv("MINZ_SELF_LANZ_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		if p, ok := v.([]byte); ok {
			b = p
		} else {
			var e error
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	write("source.nanz", source)
	inputs := lanzBaselineInputs()
	write("workload.json", inputs)
	host := newLanzHost()
	expected := []map[string]any{}
	roots := []uint16{}
	events := [][]lanzAccess{}
	nodes := [][]byte{}
	arenas := [][]byte{}
	globals := []map[string]uint16{}
	for _, s := range inputs {
		clear(host.ram[0xC000:0xD000])
		copy(host.ram[0xC000:], []byte(s))
		host.pos = 0
		host.len = uint16(len(s))
		host.node = 0x8000
		host.events = []lanzAccess{}
		root := host.parse()
		if host.node > 0x9000 || host.str > 0xB000 || len(s) >= 4096 {
			t.Fatal("independent workload resource bounds")
		}
		roots = append(roots, root)
		events = append(events, append([]lanzAccess{}, host.events...))
		nodes = append(nodes, append([]byte(nil), host.ram[0x8000:0x9000]...))
		arenas = append(arenas, append([]byte(nil), host.ram[0x9000:0xB000]...))
		g := map[string]uint16{"pos": host.pos, "src_len": host.len, "node_bump": host.node, "str_bump": host.str, "src_buf": 0xC000, "node_arena": 0x8000, "str_arena": 0x9000, "SEXP_ATOM": 1, "SEXP_LIST": 2, "SEXP_INT": 3}
		globals = append(globals, g)
		expected = append(expected, map[string]any{"root": root, "events": events[len(events)-1], "nodes": nodes[len(nodes)-1], "strings": arenas[len(arenas)-1], "globals": g})
	}
	write("host.json", expected)
	runVM := func(label string, m *mir2.Module) {
		vm := mir2.NewVM(m)
		vm.EnsureHeap(65536)
		vm.MaxSteps = 10000000
		vm.Hosts = map[string]mir2.HostFunc{}
		observed := []lanzAccess{}
		vm.Hosts["@peek"] = func(args []mir2.Value) ([]mir2.Value, error) {
			p := uint16(args[0].I)
			v := vm.ReadHeap(int64(p), 1)[0]
			observed = append(observed, lanzAccess{"read", p, v})
			return []mir2.Value{{I: int64(v)}}, nil
		}
		vm.Hosts["@poke"] = func(args []mir2.Value) ([]mir2.Value, error) {
			p := uint16(args[0].I)
			v := byte(args[1].I)
			observed = append(observed, lanzAccess{"write", p, v})
			vm.WriteHeap(int64(p), v)
			return nil, nil
		}
		putGlobal := func(name string, v uint16) { vm.WriteHeapSlice(vm.GlobalAddr(name), []byte{byte(v), byte(v >> 8)}) }
		putGlobal("str_bump", 0x9000)
		records := []map[string]any{}
		for j, s := range inputs {
			vm.WriteHeapSlice(0xC000, make([]byte, 4096))
			vm.WriteHeapSlice(0xC000, []byte(s))
			putGlobal("src_len", uint16(len(s)))
			putGlobal("pos", 0)
			putGlobal("node_bump", 0x8000)
			observed = []lanzAccess{}
			r, e := vm.Call("parse_sexpr", nil)
			record := map[string]any{"results": r, "error": fmt.Sprint(e), "events": observed, "nodes": vm.ReadHeap(0x8000, 4096), "strings": vm.ReadHeap(0x9000, 8192)}
			records = append(records, record)
			write(label+".json", records)
			if e != nil || len(r) != 1 || uint16(r[0].I) != roots[j] || !reflect.DeepEqual(observed, events[j]) || !reflect.DeepEqual(vm.ReadHeap(0x8000, 4096), nodes[j]) || !reflect.DeepEqual(vm.ReadHeap(0x9000, 8192), arenas[j]) {
				t.Errorf("%s call%d %q root%v err%v want%d traffic=%t nodes=%t strings=%t", label, j, s, r, e, roots[j], reflect.DeepEqual(observed, events[j]), reflect.DeepEqual(vm.ReadHeap(0x8000, 4096), nodes[j]), reflect.DeepEqual(vm.ReadHeap(0x9000, 8192), arenas[j]))
			}
			gotGlobals := map[string]uint16{}
			for name, v := range globals[j] {
				width := 2
				if strings.HasPrefix(name, "SEXP_") {
					width = 1
				}
				b := vm.ReadHeap(vm.GlobalAddr(name), width)
				got := uint16(b[0])
				if width == 2 {
					got |= uint16(b[1]) << 8
				}
				gotGlobals[name] = got
				if got != v {
					t.Error(label, j, "global", name, b, v)
				}
			}
			record["globals"] = gotGlobals
			write(label+".json", records)
		}
	}
	original := factory()
	write("original.hir", []byte(original.Dump()))
	runVM("raw", hir.LowerModule(original))
	candidate := factory()
	steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: false})
	write("candidate.hir", []byte(candidate.Dump()))
	write("candidate.mir", []byte(steps.MIR2Opt))
	write("unadapted.a80", []byte(steps.Assembly))
	if e != nil {
		t.Fatal(e)
	}
	runVM("optimized-mir", steps.MIR2Module)
	// Test-only extern implementations use the allocation's real ABI.
	// Explicit test-only adapter ABI contract. Refuse allocation drift rather
	// than assuming register names remain fixed.
	peek := steps.MIR2Module.FuncByName("peek")
	poke := steps.MIR2Module.FuncByName("poke")
	if peek == nil || poke == nil || len(peek.Contract.Params) != 1 || len(peek.Contract.Returns) != 1 || len(poke.Contract.Params) != 2 || len(poke.Contract.Returns) != 0 {
		t.Fatal("externABIshape")
	}
	if assertParamLocation(peek.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[1], steps.Allocation).Name != "C" || peek.Contract.Returns[0].Class != mir2.ClassAcc {
		t.Fatal("unsupportedexternallocationABI", peek.Contract, poke.Contract)
	}
	assembly := strings.Replace(steps.Assembly, "peek:\n", "peek:\n    LD A,(HL)\n    RET\n", 1)
	assembly = strings.Replace(assembly, "poke:\n", "poke:\n    LD (HL),C\n    RET\n", 1)
	write("adapter.a80", []byte(assembly))
	assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x0400\n" + assembly)
	if e != nil || len(assembled.Errors) > 0 {
		t.Fatal(e, assembled.Errors)
	}
	if assembled.Origin != 0x0400 || int(assembled.Origin)+len(assembled.Binary) >= 0x8000 {
		t.Fatal("code overlaps node arena")
	}
	write("adapter.bin", assembled.Binary)
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	z := platform.RemogattoZ80
	z.SetSMCTracker(nil)
	z.LoadMemory(0x400, assembled.Binary)
	put := func(name string, v uint16) {
		address, ok := assembled.Symbols[name]
		if !ok {
			address, ok = assembled.Symbols[strings.ToUpper(name)]
		}
		if !ok {
			t.Fatal("missingglobalsym", name)
		}
		z.SetMemory(uint16(address), byte(v))
		z.SetMemory(uint16(address+1), byte(v>>8))
	}
	put("str_bump", 0x9000)
	records := []map[string]any{}
	total := int64(0)
	entry := uint16(assembled.Symbols["parse_sexpr"])
	if entry == 0 {
		entry = uint16(assembled.Symbols["PARSE_SEXPR"])
	}
	for j, s := range inputs {
		z.LoadMemory(0xC000, make([]byte, 4096))
		z.LoadMemory(0xC000, []byte(s))
		put("src_len", uint16(len(s)))
		put("pos", 0)
		put("node_bump", 0x8000)
		z.SetMemory(0xEFEE, 0)
		z.SetMemory(0xEFEF, 1)
		z.SetRegisters(emulator.Registers{PC: entry, SP: 0xEFEE})
		profiler := emulator.NewProfiler()
		z.SetProfiler(profiler)
		start := z.GetCycles()
		observed := []lanzAccess{}
		peekPC := uint16(assembled.Symbols["peek"])
		if peekPC == 0 {
			peekPC = uint16(assembled.Symbols["PEEK"])
		}
		pokePC := uint16(assembled.Symbols["poke"])
		if pokePC == 0 {
			pokePC = uint16(assembled.Symbols["POKE"])
		}
		for n := 0; z.GetPC() != 0x100; n++ {
			if n >= 2000000 {
				write("target-failure.json", map[string]any{"call": j, "pc": z.GetPC(), "sp": z.GetSP(), "events": observed})
				t.Fatal("target didnotreturn", j, z.GetPC(), z.GetSP())
			}
			pc := z.GetPC()
			regs := z.GetRegisters()
			if regs.SP < 0xE000 || regs.SP > 0xEFEE {
				t.Fatal("target stack bounds", j, regs.SP)
			}
			if pc == peekPC {
				observed = append(observed, lanzAccess{"read", regs.HL, z.GetMemory(regs.HL)})
			}
			if pc == pokePC {
				observed = append(observed, lanzAccess{"write", regs.HL, byte(regs.BC)})
			}
			z.Step()
		}
		gotGlobals := map[string]uint16{}
		for name, want := range globals[j] {
			address, ok := assembled.Symbols[name]
			if !ok {
				address, ok = assembled.Symbols[strings.ToUpper(name)]
			}
			if !ok {
				t.Fatal("missingglobalsym", name)
			}
			got := uint16(z.GetMemory(uint16(address)))
			if !strings.HasPrefix(name, "SEXP_") {
				got |= uint16(z.GetMemory(uint16(address+1))) << 8
			}
			gotGlobals[name] = got
			if got != want {
				t.Error("targetglobal", j, name, got, want)
			}
		}
		cycles := z.GetCycles() - start
		total += int64(cycles)
		root := z.GetRegisters().HL
		record := map[string]any{"call": j, "root": root, "t_states": cycles, "globals": gotGlobals, "events": observed, "nodes": append([]byte(nil), z.Memory()[0x8000:0x9000]...), "strings": append([]byte(nil), z.Memory()[0x9000:0xB000]...)}
		records = append(records, record)
		write("target.json", records)
		if root != roots[j] || !reflect.DeepEqual(observed, events[j]) || !reflect.DeepEqual(z.Memory()[0x8000:0x9000], nodes[j]) || !reflect.DeepEqual(z.Memory()[0x9000:0xB000], arenas[j]) {
			t.Error("actualZ80", j, s, root, roots[j], "traffic", reflect.DeepEqual(observed, events[j]), "nodes", reflect.DeepEqual(z.Memory()[0x8000:0x9000], nodes[j]), "strings", reflect.DeepEqual(z.Memory()[0x9000:0xB000], arenas[j]))
		}
	}
	write("cost.json", map[string]any{"calls": len(inputs), "bytes": len(assembled.Binary), "t_states": total, "binary_sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary)), "source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "adapter": "peek HL->A LD A,(HL);RET / poke HL,C LD(HL),C;RET"})
	t.Logf("unchangedoriginalSelfLanz%d calls %dT/%dB", len(inputs), total, len(assembled.Binary))
}
