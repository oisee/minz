// VM-first mixed Nanz/Crystal Autosiril experiment. Host code feeds bytes and
// serializes data; MIDI parsing, pairing and quantization execute inside MIR2.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

type binding struct {
	Source   string   `json:"source"`
	Symbol   string   `json:"symbol"`
	Args     []string `json:"args"`
	Result   string   `json:"result"`
	Fallible bool     `json:"fallible"`
}
type manifest struct {
	Functions []binding `json:"functions"`
}
type flatCell struct {
	Row   uint32  `json:"row"`
	Notes [][]any `json:"notes"`
}
type virtualNote struct {
	Track     uint16 `json:"track"`
	Source    uint32 `json:"source"`
	Channel   byte   `json:"channel"`
	Pitch     byte   `json:"pitch"`
	StartTick uint32 `json:"start_tick"`
	OffTick   uint32 `json:"off_tick"`
	Start     uint32 `json:"start"`
	Off       uint32 `json:"off"`
}

// Link only explicitly declared, signature-identical, non-fallible scalar ABI
// entries. The generated Crystal functions remain MIR2 code, never Hosts entries.
func linkMusic(h *hir.Module, crystal *hir.Module, meta manifest) error {
	definitions := map[string]*hir.Func{}
	for _, f := range crystal.Funcs {
		if f.Body != nil {
			if definitions[f.Name] != nil {
				return fmt.Errorf("duplicate Crystal definition %s", f.Name)
			}
			definitions[f.Name] = f
		}
	}
	for _, f := range crystal.Funcs {
		if f.Body == nil {
			continue
		}
		if h.FuncByName(f.Name) != nil {
			return fmt.Errorf("Crystal symbol collision %s", f.Name)
		}
		h.Funcs = append(h.Funcs, f)
	}
	for _, b := range meta.Functions {
		placeholder := h.FuncByName(b.Source)
		if placeholder == nil {
			continue
		}
		target := definitions[b.Symbol]
		if !placeholder.IsExtern || target == nil || b.Fallible || len(placeholder.Params) != len(target.Params) || !reflect.DeepEqual(placeholder.RetTy, target.RetTy) {
			return fmt.Errorf("incompatible Crystal binding %s", b.Source)
		}
		args := make([]hir.Expr, len(placeholder.Params))
		for i, p := range placeholder.Params {
			if !reflect.DeepEqual(p.Ty, target.Params[i].Ty) {
				return fmt.Errorf("Crystal parameter %d mismatch: %s", i, b.Source)
			}
			args[i] = hir.Var(p.Name, p.Ty)
		}
		placeholder.IsExtern = false
		placeholder.Body = hir.Blk(&hir.ReturnStmt{Val: &hir.CallExpr{Fn: target.Name, Args: args, Ty: target.RetTy}})
	}
	for _, f := range h.Funcs {
		if f.IsExtern {
			return fmt.Errorf("unresolved external function %s", f.Name)
		}
	}
	return hir.Verify(h)
}
func optimize(m *mir2.Module) {
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for i := 0; i < 100; i++ {
			a := mir2.PropagateConstants(f)
			b := mir2.FoldConstants(f)
			c := mir2.SimplifyIdentities(f)
			d := mir2.ConstantCallElim(m, f)
			if !a && !b && !c && !d {
				break
			}
		}
		mir2.DeadStoreElim(f)
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f)
		}
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
	}
	if mir2.InlineTrivial(m, 4) {
		for _, f := range m.Funcs {
			mir2.PropagateCopies(f)
			mir2.DeadStoreElim(f)
		}
	}
}
func run() error {
	engine := flag.String("engine", "research/autosiril-vm/engine.nanz", "Nanz engine")
	music := flag.String("music", "", "native Crystal frontend Lanz output")
	metaPath := flag.String("manifest", "", "native Crystal export manifest")
	midi := flag.String("midi", "", "SMF input")
	out := flag.String("out", "", "JSON output")
	beat := flag.Uint("per-beat", 8, "tracker rows per quarter note")
	mapping := flag.String("mapping", "2me,1p,1m", "Autosiril source/mode mapping")
	delay := flag.Uint("delay-rows", 12, "maximum of the two echo delays for row extent")
	opt := flag.Bool("opt", false, "MZV optimization passes")
	flag.Parse()
	if *music == "" || *metaPath == "" || *midi == "" || *out == "" {
		return fmt.Errorf("need -music, -manifest, -midi and -out")
	}
	source, e := os.ReadFile(*engine)
	if e != nil {
		return e
	}
	h, e := nanz.ParseWithOpts(string(source), *engine, nanz.ParseOpts{BaseDir: filepath.Dir(*engine), StdlibDir: "stdlib"})
	if e != nil {
		return e
	}
	data, e := os.ReadFile(*music)
	if e != nil {
		return e
	}
	c, e := lanz.Compile(string(data), "autosiril_music")
	if e != nil {
		return e
	}
	data, e = os.ReadFile(*metaPath)
	if e != nil {
		return e
	}
	var meta manifest
	if e = json.Unmarshal(data, &meta); e != nil {
		return e
	}
	if e = linkMusic(h, c, meta); e != nil {
		return e
	}
	h.Target = hir.TargetMZV
	h.NonNullObjects = true
	m := hir.LowerModule(h)
	if *opt {
		optimize(m)
	}
	if e = mir2.Verify(m); e != nil {
		return e
	}
	vm := mir2.NewVM(m)
	vm.MaxMemory = 256 << 20
	vm.MaxSteps = 1000000000
	var maxAddress int64
	vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
		if a.Addr > maxAddress {
			maxAddress = a.Addr
		}
	}
	for name, width := range map[string]int64{"midi_payload": 512, "midi_buffer": 6, "midi_decoder": 44, "midi_event": 18} {
		if at := vm.GlobalAddr(name); at < 0 || at+width > 65536 {
			return fmt.Errorf("16-bit decoder descriptor out of range: %s", name)
		}
	}
	call := func(name string, args ...mir2.Value) error { _, e := vm.Call(name, args); return e }
	read := func(name string, i, width int) []byte { return vm.ReadHeap(vm.GlobalAddr(name)+int64(i*width), width) }
	u32 := func(name string, i int) uint32 { return binary.LittleEndian.Uint32(read(name, i, 4)) }
	u16 := func(name string, i int) uint16 { return binary.LittleEndian.Uint16(read(name, i, 2)) }
	u8 := func(name string, i int) byte { return read(name, i, 1)[0] }
	if e = call("boot", mir2.Value{I: int64(*beat)}); e != nil {
		return e
	}
	bytes, e := os.ReadFile(*midi)
	if e != nil {
		return e
	}
	for at, b := range bytes {
		vm.WriteHeap(vm.GlobalAddr("input_byte"), b)
		if e = call("accept"); e != nil {
			return fmt.Errorf("byte%d: %w", at, e)
		}
		if fault := u16("fault", 0); fault != 0 {
			return fmt.Errorf("byte%d: Autosiril error%d", at, fault)
		}
		r := u16("feed_result", 0)
		if r != 256 && r != 257 {
			return fmt.Errorf("byte%d: SMF status%d", at, r)
		}
	}
	if e = call("finish"); e != nil {
		return e
	}
	if r := u16("feed_result", 0); r != 4 {
		return fmt.Errorf("SMF EOF status%d", r)
	}
	// CLI syntax decoding only; note/grid selection runs in MIR2.
	var sources []uint32
	var modes []byte
	for _, group := range strings.Split(*mapping, ",") {
		for _, token := range strings.Split(group, "-") {
			n := 0
			for n < len(token) && token[n] >= '0' && token[n] <= '9' {
				n++
			}
			if n == 0 {
				return fmt.Errorf("missing source index in %q", token)
			}
			source, e := strconv.ParseUint(token[:n], 10, 32)
			if e != nil || source == 0 {
				return fmt.Errorf("invalid source index in %q", token)
			}
			flags := token[n:]
			if start := strings.IndexByte(flags, '['); start >= 0 {
				end := strings.IndexByte(flags, ']')
				if end < start {
					return fmt.Errorf("unclosed sample specifier")
				}
				flags = flags[:start] + flags[end+1:]
			}
			mode := byte('m')
			if strings.Contains(flags, "p") {
				mode = 'p'
			} else if strings.Contains(flags, "m") {
				mode = 'm'
			} else if strings.Contains(flags, "d") {
				mode = 'd'
			}
			sources = append(sources, uint32(source-1))
			modes = append(modes, mode)
		}
	}
	if len(sources) > 32 {
		return fmt.Errorf("too many source mappings")
	}
	write32 := func(name string, i int, value uint32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], value)
		vm.WriteHeapSlice(vm.GlobalAddr(name)+int64(i*4), b[:])
	}
	write32("config_count", 0, uint32(len(sources)))
	write32("delay_rows", 0, uint32(*delay))
	for i, v := range sources {
		write32("config_source", i, v)
		vm.WriteHeap(vm.GlobalAddr("config_mode")+int64(i), modes[i])
	}
	if e = call("flatten"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("flatten error%d", f)
	}
	stride := u32("row_stride", 0)
	flattened := make([][]flatCell, len(sources))
	for channel := range sources {
		flattened[channel] = []flatCell{}
		for row := uint32(0); row < stride; row++ {
			id := u32("flat_head", channel*int(stride)+int(row))
			if id == 0 {
				continue
			}
			cell := flatCell{Row: row, Notes: [][]any{}}
			for id != 0 {
				kind := "s"
				if u8("flat_type", int(id)) == 4 {
					kind = "r"
				}
				cell.Notes = append(cell.Notes, []any{u8("flat_pitch", int(id)), kind})
				id = u32("flat_next", int(id))
			}
			flattened[channel] = append(flattened[channel], cell)
		}
	}
	notes := make([]virtualNote, u32("note_count", 0))
	for i := range notes {
		id := i + 1
		notes[i] = virtualNote{Track: u16("note_track", id), Source: u32("note_source", id), Channel: u8("note_channel", id), Pitch: u8("note_pitch", id), StartTick: u32("note_start_tick", id), OffTick: u32("note_off_tick", id), Start: u32("note_start", id), Off: u32("note_off", id)}
	}
	data, e = json.MarshalIndent(map[string]any{"protocol": "autosiril-vm-notes-v1", "per_beat": *beat, "sources": u32("source_count", 0), "notes": notes, "flattened": flattened, "mapping": *mapping, "max_row": u32("max_row", 0), "vm_steps": vm.StepsExecuted(), "heap_bytes": vm.AllocHeap(nil).I, "max_access_address": maxAddress, "target": "mzv", "optimized": *opt, "language": "Nanz + native Crystal frontend HIR"}, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(*out, data, 0644); e != nil {
		return e
	}
	fmt.Printf("%s: %d notes, %d sources; Nanz→Crystal calls; %d MIR2 steps\n", filepath.Base(*midi), len(notes), u32("source_count", 0), vm.StepsExecuted())
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
