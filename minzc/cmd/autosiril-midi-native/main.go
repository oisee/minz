// autosiril-midi-native transports chunks, executes target code and checks Ruby.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

type expectation struct {
	Notes   []note  `json:"notes"`
	Sources uint16  `json:"sources"`
	Fault   uint16  `json:"fault"`
	Active  uint16  `json:"active"`
	Peak    *uint16 `json:"peak_active,omitempty"`
	Bytes   *uint32 `json:"accepted_bytes,omitempty"`
	Events  *uint32 `json:"events,omitempty"`
}
type testCase struct {
	Name     string      `json:"name"`
	MIDI     []byte      `json:"midi"`
	Beat     uint32      `json:"per_beat"`
	Chunks   []int       `json:"chunks"`
	Expected expectation `json:"expected"`
}
type runConfig struct{ Source, Music, Manifest, Stdlib, Input, Out string }

func check(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) []byte { b, e := os.ReadFile(p); check(e); return b }
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func build(source, music, meta []byte, stdlib string) (*hir.Module, error) {
	h, e := nanz.ParseWithOpts(string(source), "paired-midi", nanz.ParseOpts{StdlibDir: stdlib})
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
	// Verify before linking: an undeclared Lanz identifier must not capture a
	// receiving Nanz global. Keep parameter/local shadowing lexical and valid.
	closed := &hir.Module{Funcs: []*hir.Func{target}}
	if e := hir.VerifyClosedNames(closed); e != nil {
		return nil, fmt.Errorf("quantizer must be closed scalar HIR: %w", e)
	}
	for _, facts := range hir.AnalyzeFacts(closed).Functions {
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
		return nil, fmt.Errorf("native quantizer error ABI: %w", e)
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
	r := result{Engine: engine, Notes: make([]note, 0), Fault: u16("fault"), Sources: u16("source_count"), Active: u16("active_count"), Peak: u16("peak_active"), Bytes: u32("accepted_bytes"), Events: u32("event_count")}
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

func transport(midi []byte, chunk int, beat uint32, put func(string, int, byte), call func() uint16) {
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
	set("per_beat", beat, 4)
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

func vmRun(m *mir2.Module, midi []byte, chunk int, beat uint32, name string) result {
	v := mir2.NewVM(m)
	v.MaxMemory = 65536
	v.MaxSteps = 1000000000
	transport(midi, chunk, beat, func(name string, off int, b byte) { v.WriteHeap(v.GlobalAddr(name)+int64(off), b) }, func() uint16 {
		r, e := v.Call("native_main", nil)
		check(e)
		if len(r) != 1 {
			panic("invalid target return")
		}
		if uint16(r[0].I) != binary.LittleEndian.Uint16(v.ReadHeap(v.GlobalAddr("fault"), 2)) {
			panic("VM return differs from fault")
		}
		return uint16(r[0].I)
	})
	return collect(name, func(name string, off, n int) []byte { return v.ReadHeap(v.GlobalAddr(name)+int64(off), n) })
}

type dataSpan struct {
	Name    string `json:"name"`
	Address int    `json:"address"`
	Bytes   int    `json:"bytes"`
}

func backendSpills(assembly string, symbols map[string]int, image []byte) (spans []dataSpan) {
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
		}
		spans = append(spans, dataSpan{name, at, n})
	}
	return
}

func nativeRun(image []byte, symbols map[string]int, globals []mir2.Global, assembly string, midi []byte, chunk int, beat uint32) result {
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
		if !ok || at < origin || at+mir2.ByteWidth(g.Ty) > origin+len(image) {
			panic("missing global")
		}
		for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
			allowed[at+i] = true
		}
	}
	for _, span := range backendSpills(assembly, symbols, image) {
		for i := 0; i < span.Bytes; i++ {
			allowed[span.Address+i] = true
		}
	}
	for i := floor; i < initialSP; i++ {
		allowed[i] = true
	}
	minSP := uint16(initialSP)
	transport(midi, chunk, beat, func(name string, off int, b byte) { mem[symbols[name]+off] = b }, func() uint16 {
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
		if z.GetRegisters().HL != binary.LittleEndian.Uint16(mem[symbols["fault"]:symbols["fault"]+2]) {
			panic("native return differs from fault")
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

// run confines verifier failures to a controlled error; target faults are data
// and are compared with the independently supplied expectations below.
func run(cfg runConfig) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("native MIDI verifier: %v", r)
		}
	}()
	if cfg.Out == "" {
		return fmt.Errorf("output directory required")
	}
	check(os.MkdirAll(cfg.Out, 0755))
	source, music, meta := read(cfg.Source), read(cfg.Music), read(cfg.Manifest)
	input := read(cfg.Input)
	var matrix struct {
		Cases []testCase `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	check(decoder.Decode(&matrix))
	if len(matrix.Cases) == 0 {
		return fmt.Errorf("no cases")
	}
	names := map[string]bool{}
	for _, tc := range matrix.Cases {
		if tc.Name == "" || names[tc.Name] || len(tc.Chunks) == 0 {
			return fmt.Errorf("invalid/duplicate case name or missing chunks")
		}
		names[tc.Name] = true
		for _, chunk := range tc.Chunks {
			if chunk < 1 || chunk > 256 {
				return fmt.Errorf("transport chunks require 1..256 bytes")
			}
		}
		if len(tc.Expected.Notes) > 2046 || tc.Expected.Sources > 64 || tc.Expected.Active > 512 {
			return fmt.Errorf("reference exceeds bounded target fields")
		}
		for _, n := range tc.Expected.Notes {
			if n.Pitch > 127 || n.Source >= 64 {
				return fmt.Errorf("reference note field out of range")
			}
		}
	}
	h, e := build(source, music, meta, cfg.Stdlib)
	check(e)
	raw := hir.LowerModule(h)
	check(mir2.Verify(raw))
	h, e = build(source, music, meta, cfg.Stdlib)
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
		check(os.WriteFile(filepath.Join(cfg.Out, name), b, 0644))
	}
	if origin+len(a.Binary)+64 >= floor {
		return fmt.Errorf("native image exceeds stack window")
	}
	dataBytes := 0
	for _, g := range steps.MIR2Module.Globals {
		dataBytes += mir2.ByteWidth(g.Ty)
	}
	spills := backendSpills(steps.Assembly, a.Symbols, a.Binary)
	spillBytes := 0
	for _, span := range spills {
		spillBytes += span.Bytes
	}
	rows := []map[string]any{}
	for _, tc := range matrix.Cases {
		for _, chunk := range tc.Chunks {
			results := []result{vmRun(raw, tc.MIDI, chunk, tc.Beat, "raw-mir2"), vmRun(steps.MIR2Module, tc.MIDI, chunk, tc.Beat, "final-mir2"), nativeRun(a.Binary, a.Symbols, steps.MIR2Module.Globals, steps.Assembly, tc.MIDI, chunk, tc.Beat)}
			summaries := []map[string]any{}
			for _, r := range results {
				want := tc.Expected
				if r.Fault != want.Fault || r.Sources != want.Sources || r.Active != want.Active || len(r.Notes) != len(want.Notes) {
					return fmt.Errorf("%s/chunk%d/%s state differs: fault%d/%d notes%d/%d sources%d/%d active%d/%d", tc.Name, chunk, r.Engine, r.Fault, want.Fault, len(r.Notes), len(want.Notes), r.Sources, want.Sources, r.Active, want.Active)
				}
				if want.Peak != nil && r.Peak != *want.Peak {
					return fmt.Errorf("%s/%s peak%d want%d", tc.Name, r.Engine, r.Peak, *want.Peak)
				}
				if want.Bytes != nil && r.Bytes != *want.Bytes {
					return fmt.Errorf("%s/%s accepted%d want%d", tc.Name, r.Engine, r.Bytes, *want.Bytes)
				}
				if want.Events != nil && r.Events != *want.Events {
					return fmt.Errorf("%s/%s events%d want%d", tc.Name, r.Engine, r.Events, *want.Events)
				}
				if r.Bytes > uint32(len(tc.MIDI)) || (want.Fault == 0 && r.Bytes != uint32(len(tc.MIDI))) {
					return fmt.Errorf("%s/%s consumed invalid byte count", tc.Name, r.Engine)
				}
				for i, n := range r.Notes {
					if n != want.Notes[i] {
						return fmt.Errorf("%s/%s note%d: %+v != %+v", tc.Name, r.Engine, i, n, want.Notes[i])
					}
				}
				comparable, first := r, results[0]
				comparable.Engine = ""
				first.Engine = ""
				comparable.TStates = 0
				first.TStates = 0
				comparable.Stack = 0
				first.Stack = 0
				if !reflect.DeepEqual(comparable, first) {
					return fmt.Errorf("%s/%s cross-engine state differs", tc.Name, r.Engine)
				}
				b, e := json.Marshal(r.Notes)
				check(e)
				summaries = append(summaries, map[string]any{"engine": r.Engine, "note_count": len(r.Notes), "notes_sha256": hash(b), "fault": r.Fault, "sources": r.Sources, "active": r.Active, "peak_active": r.Peak, "accepted_bytes": r.Bytes, "events": r.Events, "t_states": r.TStates, "stack_below_entry": r.Stack})
			}
			rows = append(rows, map[string]any{"name": tc.Name, "chunk_size": chunk, "midi_sha256": hash(tc.MIDI), "engines": summaries})
			fmt.Printf("PASS %s/chunk%d: %d notes fault%d, %dT stack%d\n", tc.Name, chunk, len(results[2].Notes), results[2].Fault, results[2].TStates, results[2].Stack)
		}
	}
	receipt := map[string]any{"protocol": "autosiril-native-midi/v2", "image_bytes": len(a.Binary), "module_globals_bytes": dataBytes, "backend_static_spill_bytes": spillBytes, "code_bytes": len(a.Binary) - dataBytes - spillBytes, "backend_spills": spills, "source_sha256": hash(source), "music_sha256": hash(music), "manifest_sha256": hash(meta), "input_sha256": hash(input), "assembly_sha256": hash([]byte(steps.Assembly)), "binary_sha256": hash(a.Binary), "target_executions": len(rows) * 3, "cases": rows, "scope": "streamed MIDI -> paired records on actual shared decoder and Crystal quantizer; separate executable from flatten, no combined converter or physical banking"}
	b, e := json.MarshalIndent(receipt, "", "  ")
	check(e)
	check(os.WriteFile(filepath.Join(cfg.Out, "receipt.json"), append(b, '\n'), 0644))
	return nil
}

func main() {
	root := flag.String("root", "..", "repository root")
	input := flag.String("input", "", "case matrix JSON")
	out := flag.String("out", "", "artifact directory")
	source := flag.String("source", "", "Nanz pairing stage override")
	music := flag.String("music", "", "actual Crystal export override")
	manifest := flag.String("manifest", "", "actual Crystal manifest override")
	flag.Parse()
	if *source == "" {
		*source = filepath.Join(*root, "research/autosiril-native/midi/pair.nanz")
	}
	if *music == "" {
		*music = filepath.Join(*root, "research/autosiril-native/fixtures/music.lanz")
	}
	if *manifest == "" {
		*manifest = filepath.Join(*root, "research/autosiril-native/fixtures/music-manifest.json")
	}
	if *input == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "-input and -out required")
		os.Exit(1)
	}
	if e := run(runConfig{*source, *music, *manifest, filepath.Join(*root, "stdlib"), *input, *out}); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
