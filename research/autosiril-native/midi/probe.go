// Research runner: transport MIDI chunks, execute target code, compare Ruby.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const origin = 0x4000
const floor = 0xe000
const initialSP = 0xeffe
const returnPC = 0xf100

type note struct {
	Start  uint16 `json:"start"`
	Off    uint16 `json:"off"`
	Pitch  uint8  `json:"pitch"`
	Source uint8  `json:"source"`
}
type result struct {
	Engine  string `json:"engine"`
	Fault   uint16 `json:"fault"`
	Notes   []note `json:"notes"`
	Sources uint16 `json:"sources"`
	Peak    uint16 `json:"peak_active"`
	Active  uint16 `json:"active"`
	Bytes   uint32 `json:"accepted_bytes"`
	Events  uint32 `json:"events"`
	TStates int    `json:"t_states,omitempty"`
	Stack   int    `json:"stack_below_entry,omitempty"`
}
type binding struct {
	Source   string   `json:"source"`
	Symbol   string   `json:"symbol"`
	Args     []string `json:"args"`
	Result   string   `json:"result"`
	Fallible bool     `json:"fallible"`
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) []byte { b, e := os.ReadFile(p); check(e); return b }
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func build(source, music, meta []byte) (*hir.Module, error) {
	h, e := nanz.ParseWithOpts(string(source), "paired-midi", nanz.ParseOpts{StdlibDir: "../stdlib"})
	if e != nil {
		return nil, e
	}
	c, e := lanz.Compile(string(music), "actual-music")
	if e != nil {
		return nil, e
	}
	if len(c.Globals) != 0 {
		return nil, fmt.Errorf("unexpected Crystal globals")
	}
	var m struct {
		Functions []binding `json:"functions"`
	}
	if e = json.Unmarshal(meta, &m); e != nil {
		return nil, e
	}
	var selected *binding
	for i, b := range m.Functions {
		if b.Source == "crystal_quantize" {
			if selected != nil {
				return nil, fmt.Errorf("duplicate quantizer binding")
			}
			selected = &m.Functions[i]
		}
	}
	if selected == nil || selected.Fallible || selected.Result != "u32" || len(selected.Args) != 3 {
		return nil, fmt.Errorf("invalid quantizer manifest")
	}
	for _, a := range selected.Args {
		if a != "u32" {
			return nil, fmt.Errorf("invalid quantizer argument")
		}
	}
	var target *hir.Func
	for _, f := range c.Funcs {
		if f.Name == selected.Symbol && f.Body != nil {
			if target != nil {
				return nil, fmt.Errorf("duplicate actual quantizer")
			}
			target = f
		}
	}
	p := h.FuncByName("crystal_quantize")
	if p == nil || !p.IsExtern || target == nil || target.IsExtern || target.IsIO || target.IsGen || len(target.Captures) != 0 || len(target.RetTys) != 0 || target.RetTy != mir2.TyU32 || len(target.Params) != 3 || p.RetTy != mir2.TyU32 || len(p.Params) != 3 {
		return nil, fmt.Errorf("invalid actual quantizer ABI")
	}
	for _, facts := range hir.AnalyzeFacts(&hir.Module{Funcs: []*hir.Func{target}}).Functions {
		if facts.Name == target.Name && (len(facts.Calls) != 0 || facts.Direct&(hir.ReadsMemory|hir.WritesMemory|hir.DoesIO|hir.UnknownEffect) != 0) {
			return nil, fmt.Errorf("unsupported quantizer effects/dependencies")
		}
	}
	args := make([]hir.Expr, 3)
	for i, v := range p.Params {
		if v.Ty != mir2.TyU32 || target.Params[i].Ty != mir2.TyU32 {
			return nil, fmt.Errorf("invalid actual quantizer parameter")
		}
		args[i] = hir.Var(v.Name, v.Ty)
	}
	if h.FuncByName(target.Name) != nil {
		return nil, fmt.Errorf("quantizer symbol collision")
	}
	h.Funcs = append(h.Funcs, target)
	p.IsExtern = false
	p.Body = hir.Blk(&hir.ReturnStmt{Val: &hir.CallExpr{Fn: target.Name, Args: args, Ty: mir2.TyU32}})
	h.Target = hir.TargetCPM
	h.NonNullObjects = true
	if e = hir.ResolveErrorABI(h); e != nil {
		return nil, e
	}
	if target.FallibleCY {
		return nil, fmt.Errorf("actual quantizer is fallible")
	}
	for _, f := range h.Funcs {
		if f.IsExtern {
			return nil, fmt.Errorf("unresolved external %s", f.Name)
		}
	}
	return h, hir.Verify(h)
}

func collect(engine string, get func(string, int, int) []byte) result {
	u16 := func(name string) uint16 { return binary.LittleEndian.Uint16(get(name, 0, 2)) }
	u32 := func(name string) uint32 { return binary.LittleEndian.Uint32(get(name, 0, 4)) }
	r := result{Engine: engine, Fault: u16("fault"), Sources: u16("source_count"), Active: u16("active_count"), Peak: u16("peak_active"), Bytes: u32("accepted_bytes"), Events: u32("event_count")}
	count := int(u16("note_count"))
	if count > 2046 {
		panic("target count exceeds arena")
	}
	for i := 0; i < count; i++ {
		at := (i/682)*4096 + (i%682)*6
		b := get("note_bytes", at, 6)
		r.Notes = append(r.Notes, note{binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:]), b[4], b[5]})
	}
	return r
}

func transport(midi []byte, chunk int, put func(string, int, byte), call func() uint16) {
	set := func(name string, value uint32, n int) {
		for i := 0; i < n; i++ {
			put(name, i, byte(value>>uint(8*i)))
		}
	}
	for bank := 0; bank < 3; bank++ {
		for j := 4092; j < 4096; j++ {
			put("note_bytes", bank*4096+j, 0xda)
		}
	}
	set("per_beat", 8, 4)
	set("native_control", 0, 1)
	if call() != 0 {
		return
	}
	for at := 0; at < len(midi); at += chunk {
		end := at + chunk
		if end > len(midi) {
			end = len(midi)
		}
		for i, b := range midi[at:end] {
			put("input_chunk", i, b)
		}
		set("input_length", uint32(end-at), 2)
		set("native_control", 1, 1)
		if call() != 0 {
			return
		}
	}
	set("native_control", 2, 1)
	call()
}

func vmRun(m *mir2.Module, midi []byte, chunk int, name string) result {
	v := mir2.NewVM(m)
	v.MaxMemory = 65536
	v.MaxSteps = 1000000000
	transport(midi, chunk, func(name string, off int, b byte) { v.WriteHeap(v.GlobalAddr(name)+int64(off), b) }, func() uint16 {
		r, e := v.Call("native_main", nil)
		check(e)
		if len(r) != 1 {
			panic("invalid target return")
		}
		return uint16(r[0].I)
	})
	return collect(name, func(name string, off, n int) []byte { return v.ReadHeap(v.GlobalAddr(name)+int64(off), n) })
}

func nativeRun(image []byte, symbols map[string]int, globals []mir2.Global, assembly string, midi []byte, chunk int) result {
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	p := emulator.NewProfiler()
	z.SetProfiler(p)
	mem := z.Memory()
	for i := range mem {
		mem[i] = 0xa5
	}
	copy(mem[origin:], image)
	allowed := make([]bool, 65536)
	for _, g := range globals {
		if g.At != nil {
			panic("absolute global")
		}
		at, ok := symbols[g.Name]
		if !ok {
			panic("missing global")
		}
		for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
			allowed[at+i] = true
		}
	}
	// Exact zero-initialized spill declarations, never an executable-code range.
	for _, line := range strings.Split(assembly, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		name, rest, ok := strings.Cut(line, ":")
		if !ok || !strings.HasPrefix(name, "_spill_") {
			continue
		}
		rest = strings.TrimSpace(rest)
		width := 1
		if strings.HasPrefix(rest, "DW ") {
			width = 2
		}
		if !strings.HasPrefix(rest, "DB ") && !strings.HasPrefix(rest, "DW ") {
			panic("unknown spill declaration")
		}
		values := strings.Split(strings.TrimSpace(rest[3:]), ",")
		n := len(values) * width
		if n > 4 {
			panic("spill width")
		}
		at, ok := symbols[name]
		if !ok || at < origin || at+n > origin+len(image) {
			panic("spill address")
		}
		for _, v := range values {
			if strings.TrimSpace(v) != "0" {
				panic("spill initializer")
			}
		}
		for i := 0; i < n; i++ {
			if image[at-origin+i] != 0 {
				panic("spill bytes")
			}
			allowed[at+i] = true
		}
	}
	for i := floor; i < initialSP; i++ {
		allowed[i] = true
	}
	minSP := uint16(initialSP)
	transport(midi, chunk, func(name string, off int, b byte) { mem[symbols[name]+off] = b }, func() uint16 {
		mem[initialSP] = 0
		mem[initialSP+1] = 0xf1
		z.SetRegisters(emulator.Registers{PC: uint16(symbols["native_main"]), SP: initialSP})
		for steps := 0; z.GetPC() != returnPC; steps++ {
			if steps >= 200000000 {
				panic("native step limit")
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
			if z.GetSP() < floor {
				panic("native stack floor")
			}
		}
		if z.GetSP() != initialSP+2 {
			panic("native return stack")
		}
		return z.GetRegisters().HL
	})
	if len(p.IORead) != 0 || len(p.IOWrite) != 0 {
		panic("native port I/O")
	}
	for i, w := range p.WriteCount {
		if w != 0 && !allowed[i] {
			panic(fmt.Sprintf("native write outside storage %04x", i))
		}
		if allowed[i] && i < floor && p.ExecCount[i] != 0 {
			panic("executed native data")
		}
	}
	for _, name := range []string{"input_chunk", "input_length", "native_control", "per_beat"} {
		at := symbols[name]
		width := map[string]int{"input_chunk": 256, "input_length": 2, "native_control": 1, "per_beat": 4}[name]
		for i := 0; i < width; i++ {
			if p.WriteCount[at+i] != 0 {
				panic("native wrote transport input")
			}
		}
	}
	for bank := 0; bank < 3; bank++ {
		for j := 4092; j < 4096; j++ {
			at := symbols["note_bytes"] + bank*4096 + j
			if mem[at] != 0xda || p.WriteCount[at] != 0 {
				panic("native wrote arena padding")
			}
		}
	}
	for _, span := range [][2]int{{origin - 64, origin}, {origin + len(image), origin + len(image) + 64}, {floor - 64, floor}} {
		for i := span[0]; i < span[1]; i++ {
			if mem[i] != 0xa5 {
				panic("native canary")
			}
		}
	}
	r := collect("z80", func(name string, off, n int) []byte { at := symbols[name] + off; return mem[at : at+n] })
	r.Stack = int(initialSP - minSP)
	r.TStates = z.GetCycles()
	return r
}

func main() {
	midiPath := flag.String("midi", "", "SMF input")
	refPath := flag.String("reference", "", "original Ruby oracle JSON")
	musicPath := flag.String("music", "../research/autosiril-native/fixtures/music.lanz", "actual Crystal export")
	manifestPath := flag.String("manifest", "../research/autosiril-native/fixtures/music-manifest.json", "actual Crystal manifest")
	out := flag.String("out", "", "artifact directory")
	chunk := flag.Int("chunk", 256, "transport chunk size 1..256")
	flag.Parse()
	if *midiPath == "" || *refPath == "" || *out == "" || *chunk < 1 || *chunk > 256 {
		panic("-midi -reference -out and chunk1..256 required")
	}
	check(os.MkdirAll(*out, 0755))
	source := read("../research/autosiril-native/midi/pair.nanz")
	music := read(*musicPath)
	meta := read(*manifestPath)
	midi := read(*midiPath)
	reference := read(*refPath)
	h, e := build(source, music, meta)
	check(e)
	raw := hir.LowerModule(h)
	check(mir2.Verify(raw))
	h, e = build(source, music, meta)
	check(e)
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	opts.DisableVariantSearch = true
	steps, e := pipeline.CompileHIRSteps(h, opts)
	check(e)
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + steps.Assembly)
	check(e)
	if len(a.Errors) != 0 {
		panic(a.Errors)
	}
	for name, b := range map[string][]byte{"program.a80": []byte(steps.Assembly), "program.bin": a.Binary, "raw.mir": []byte(raw.Dump()), "final.mir": []byte(steps.MIR2Module.Dump())} {
		check(os.WriteFile(filepath.Join(*out, name), b, 0644))
	}
	fmt.Printf("compiled image: %d bytes\n", len(a.Binary))
	if origin+len(a.Binary)+64 >= floor {
		panic("native image exceeds stack window")
	}
	var expected struct {
		Notes   []note `json:"notes"`
		Sources uint16 `json:"sources"`
	}
	check(json.Unmarshal(reference, &expected))
	results := []result{}
	for _, entry := range []struct {
		name string
		m    *mir2.Module
	}{{"raw-mir2", raw}, {"final-mir2", steps.MIR2Module}} {
		r := vmRun(entry.m, midi, *chunk, entry.name)
		results = append(results, r)
		fmt.Printf("%s: fault%d notes%d sources%d events%d bytes%d\n", r.Engine, r.Fault, len(r.Notes), r.Sources, r.Events, r.Bytes)
	}
	r := nativeRun(a.Binary, a.Symbols, steps.MIR2Module.Globals, steps.Assembly, midi, *chunk)
	results = append(results, r)
	for _, r := range results {
		if r.Fault != 0 || r.Sources != expected.Sources || r.Active != 0 || r.Bytes != uint32(len(midi)) || len(r.Notes) != len(expected.Notes) {
			panic(fmt.Sprintf("%s counters differ: fault%d notes%d/%d sources%d/%d active%d bytes%d", r.Engine, r.Fault, len(r.Notes), len(expected.Notes), r.Sources, expected.Sources, r.Active, r.Bytes))
		}
		for i, n := range r.Notes {
			if n != expected.Notes[i] {
				panic(fmt.Sprintf("%s note%d: %+v != %+v", r.Engine, i, n, expected.Notes[i]))
			}
		}
	}
	dataBytes := 0
	for _, g := range steps.MIR2Module.Globals {
		dataBytes += mir2.ByteWidth(g.Ty)
	}
	receipt := map[string]any{"protocol": "autosiril-native-midi-probe/v1", "image_bytes": len(a.Binary), "module_globals_bytes": dataBytes, "chunk_size": *chunk, "midi_sha256": hash(midi), "ruby_output_sha256": hash(reference), "source_sha256": hash(source), "music_sha256": hash(music), "manifest_sha256": hash(meta), "assembly_sha256": hash([]byte(steps.Assembly)), "binary_sha256": hash(a.Binary), "results": results, "scope": "streamed MIDI -> paired records on actual shared decoder and Crystal quantizer; separate executable from flatten stage; not combined converter or physical banking"}
	b, e := json.MarshalIndent(receipt, "", "  ")
	check(e)
	check(os.WriteFile(filepath.Join(*out, "receipt.json"), append(b, '\n'), 0644))
	fmt.Printf("PASS %d notes on raw/final MIR2 and Z80; %dT stack%d\n", len(r.Notes), r.TStates, r.Stack)
}
