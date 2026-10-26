// VM-first mixed Nanz/Crystal Autosiril experiment. Host code feeds bytes and
// serializes data; MIDI parsing, pairing and quantization execute inside MIR2.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

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
type lNote struct {
	Note     int16   `json:"note"`
	Sample   byte    `json:"sample"`
	Envelope byte    `json:"envelope"`
	Ornament uint32  `json:"ornament"`
	Volume   float64 `json:"volume"`
	Type     string  `json:"type"`
	Kind     string  `json:"kind"`
	ENote    int16   `json:"enote"`
}
type lCell struct {
	Row  uint32 `json:"row"`
	Note lNote  `json:"note"`
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
			return fmt.Errorf("symbol collision for Crystal %s", f.Name)
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
				return fmt.Errorf("parameter %d mismatch for Crystal binding %s", i, b.Source)
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
	lazyInput := flag.String("lazy-input", "", "paired-note JSON for experimental lazy flatten stage")
	lazyEngine := flag.String("lazy-engine", "research/autosiril-vm/lazy_flatten.nanz", "experimental generator source")
	arenaBackend := flag.String("arena-backend", "flat", "flat, banked or churn simulator")
	arenaBudget := flag.Uint("arena-budget", 128*1024, "VM stage memory cap, excluding native code and host I/O")
	lazyLimit := flag.Uint("lazy-limit", 4294967295, "maximum yielded entries per config")
	probePath := flag.String("probe-cases", "", "optional scalar Crystal test cases; bypass MIDI conversion")
	vtOut := flag.String("vt-out", "", "write actual VortexTracker II module text")
	headerPath := flag.String("header", "research/autosiril-vm/vt2-header.txt", "reference VT2 sample/header template")
	patternSize := flag.Uint("pattern-size", 0, "pattern rows; 0 uses Autosiril default")
	skipRows := flag.Uint("skip-lines", 64, "initial tracker rows to skip")
	engine := flag.String("engine", "research/autosiril-vm/engine.nanz", "Nanz engine")
	music := flag.String("music", "", "native Crystal frontend Lanz output")
	metaPath := flag.String("manifest", "", "native Crystal export manifest")
	midi := flag.String("midi", "", "SMF input")
	out := flag.String("out", "", "JSON output")
	beat := flag.Uint("per-beat", 8, "tracker rows per quarter note")
	maxOffset := flag.Uint("max-offset", 24, "maximum chord offset from median")
	repeats := flag.Uint("orn-repeat", 2, "ornament frames per chord note")
	mapping := flag.String("mapping", "2me,1p,1m", "Autosiril source/mode mapping")
	delayOne := flag.Uint("delay-one", 6, "first echo delay")
	delayTwo := flag.Uint("delay-two", 12, "second echo delay")
	delay := flag.Uint("delay-rows", 12, "maximum of the two echo delays for row extent")
	opt := flag.Bool("opt", false, "MZV optimization passes")
	flag.Parse()
	if *lazyInput != "" {
		return runLazy(*lazyInput, *lazyEngine, *music, *metaPath, *out, *arenaBackend, *arenaBudget, *lazyLimit, *opt)
	}
	if *music == "" || *metaPath == "" || (*midi == "" && *probePath == "") || *out == "" {
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
	if *probePath != "" {
		type probe struct {
			Source        string
			Args          []int64
			Expected      int64
			ErrorContains string
		}
		data, e := os.ReadFile(*probePath)
		if e != nil {
			return e
		}
		var cases []probe
		if e = json.Unmarshal(data, &cases); e != nil {
			return e
		}
		for _, tc := range cases {
			symbol := ""
			for _, b := range meta.Functions {
				if b.Source == tc.Source {
					if symbol != "" {
						return fmt.Errorf("ambiguous probe source %s", tc.Source)
					}
					symbol = b.Symbol
				}
			}
			if symbol == "" {
				return fmt.Errorf("unknown probe source %s", tc.Source)
			}
			args := make([]mir2.Value, len(tc.Args))
			for i, v := range tc.Args {
				args[i] = mir2.Value{I: v}
			}
			got, e := vm.Call(symbol, args)
			if tc.ErrorContains != "" {
				if e == nil || !strings.Contains(e.Error(), tc.ErrorContains) {
					return fmt.Errorf("probe %s: wanted %s, got %v", tc.Source, tc.ErrorContains, e)
				}
				continue
			}
			if e != nil || len(got) != 1 || got[0].I != tc.Expected {
				return fmt.Errorf("probe %s%v got%v error%v want%d", tc.Source, tc.Args, got, e, tc.Expected)
			}
		}
		data, e = json.MarshalIndent(map[string]any{"kernel_cases": len(cases), "optimized": *opt, "vm_steps": vm.StepsExecuted()}, "", "  ")
		if e != nil {
			return e
		}
		return os.WriteFile(*out, data, 0644)
	}
	if e = call("boot", mir2.Value{I: int64(*beat)}); e != nil {
		return e
	}
	inputData, e := os.ReadFile(*midi)
	if e != nil {
		return e
	}
	for at, b := range inputData {
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
	var samples, ornaments, envelopes, unvoiced, wide, groups, priorities []byte
	for groupIndex, group := range strings.Split(*mapping, ",") {
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
			sample, ornament := byte(2), byte(0)
			if start := strings.IndexByte(flags, '['); start >= 0 {
				end := strings.IndexByte(flags, ']')
				if end < start {
					return fmt.Errorf("unclosed sample specifier")
				}
				spec := strings.ToUpper(flags[start+1 : end])
				alphabet := ".123456789ABCDEFGHIJKLMNOPQRSTUV"
				if len(spec) > 0 {
					if v := strings.IndexByte(alphabet, spec[0]); v >= 0 {
						sample = byte(v)
					}
				}
				if len(spec) > 1 {
					if v := strings.IndexByte(alphabet, spec[1]); v >= 0 {
						ornament = byte(v)
					}
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
			bit := func(v bool) byte {
				if v {
					return 1
				}
				return 0
			}
			samples = append(samples, sample)
			ornaments = append(ornaments, ornament)
			envelopes = append(envelopes, bit(strings.Contains(flags, "e")))
			unvoiced = append(unvoiced, bit(strings.Contains(flags, "u")))
			wide = append(wide, bit(strings.Contains(flags, "w")))
			groups = append(groups, byte(groupIndex))
			priorities = append(priorities, bit(strings.Contains(flags, "+")))
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
		for name, values := range map[string][]byte{"config_sample": samples, "config_envelope": envelopes, "config_unvoiced": unvoiced, "config_wide": wide, "config_group": groups, "config_priority": priorities} {
			vm.WriteHeap(vm.GlobalAddr(name)+int64(i), values[i])
		}
		vm.WriteHeapSlice(vm.GlobalAddr("config_ornament")+int64(i*2), []byte{ornaments[i], 0})
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
	if *maxOffset > 127 || *repeats == 0 || *repeats > 32 {
		return fmt.Errorf("invalid ornament settings")
	}
	vm.WriteHeap(vm.GlobalAddr("max_offset"), byte(*maxOffset))
	vm.WriteHeapSlice(vm.GlobalAddr("orn_repeat"), []byte{byte(*repeats), 0})
	if e = call("build_lnotes"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("building LNotes: error%d", f)
	}
	readL := func(id uint32) lNote {
		word := func(field int) uint32 { return u32("l_words", int(id)*8+field) }
		typ := "s"
		if word(5) == 4 {
			typ = "r"
		}
		return lNote{Note: int16(word(0)), Sample: byte(word(1)), Envelope: byte(word(2)), Ornament: word(3), Volume: float64(word(4)) / 100, Type: typ, Kind: string([]byte{byte(word(6))}), ENote: int16(word(7))}
	}
	lnotes := make([][]lCell, len(sources))
	for channel := range sources {
		lnotes[channel] = []lCell{}
		for row := uint32(0); row < stride; row++ {
			if id := u32("l_grid", channel*int(stride)+int(row)); id != 0 {
				lnotes[channel] = append(lnotes[channel], lCell{Row: row, Note: readL(id)})
			}
		}
	}
	write32("delay_one", 0, uint32(*delayOne))
	write32("delay_two", 0, uint32(*delayTwo))
	if e = call("apply_echo"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("echo error%d", f)
	}
	mixStride := u32("mix_stride", 0)
	readGrid := func(name, lengthName string, channels int) [][]lCell {
		data := make([][]lCell, channels)
		for channel := 0; channel < channels; channel++ {
			data[channel] = []lCell{}
			for row := uint32(0); row < u32(lengthName, channel); row++ {
				if id := u32(name, channel*int(mixStride)+int(row)); id != 0 {
					data[channel] = append(data[channel], lCell{Row: row, Note: readL(id)})
				}
			}
		}
		return data
	}
	delayed := readGrid("d_grid", "d_length", len(sources))
	if e = call("downmix"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("downmix error%d", f)
	}
	mixed := readGrid("m_grid", "m_length", 3)
	if e = call("render_rows"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("render error%d", f)
	}
	rowsText := string(vm.ReadHeap(vm.GlobalAddr("render_bytes"), int(u32("render_used", 0))))
	ps := uint32(*patternSize)
	if ps == 0 {
		ps = uint32(*beat) * 64
		for ps > 127 && ps%2 == 0 {
			ps /= 2
		}
	}
	write32("pattern_size", 0, ps)
	write32("skip_lines", 0, uint32(*skipRows))
	if e = call("split_patterns"); e != nil {
		return e
	}
	if f := u16("fault", 0); f != 0 {
		return fmt.Errorf("patterns error%d", f)
	}
	patterns := make([]string, u32("pattern_count", 0))
	used := make([]bool, len(patterns))
	order := make([]string, len(patterns))
	for i := range patterns {
		start := u32("pattern_start", i)
		n := u32("pattern_length", i)
		patterns[i] = string(vm.ReadHeap(vm.GlobalAddr("render_bytes")+int64(start), int(n)))
		used[i] = u8("pattern_used", i) != 0
		order[i] = strconv.FormatUint(uint64(u32("pattern_order", i)), 10)
	}
	playOrder := ""
	if len(order) > 0 {
		playOrder = "L" + strings.Join(order, ",")
	}
	ornamentData := make([][]byte, u32("orn_count", 0))
	for i := range ornamentData {
		n := int(u16("orn_length", i))
		start := u32("orn_start", i)
		ornamentData[i] = vm.ReadHeap(vm.GlobalAddr("orn_bytes")+int64(start), n)
	}
	// Encode integer arrays, not base64 byte slices, for the independent oracle.
	ornamentsJSON := make([][]int, len(ornamentData))
	for i, v := range ornamentData {
		ornamentsJSON[i] = []int{}
		for _, b := range v {
			ornamentsJSON[i] = append(ornamentsJSON[i], int(b))
		}
	}
	var ornamentsText strings.Builder
	for i, v := range ornamentsJSON {
		if i == 0 {
			continue
		}
		parts := make([]string, len(v))
		for j, n := range v {
			parts[j] = strconv.Itoa(n)
		}
		fmt.Fprintf(&ornamentsText, "\n[Ornament%d]\nL%s\n", i, strings.Join(parts, ","))
	}
	header, e := os.ReadFile(*headerPath)
	if e != nil {
		return e
	}
	args := fmt.Sprintf("%s %s %d %d %d %d %d %d %d", *midi, *mapping, *beat, *delayOne, *delayTwo, *patternSize, *skipRows, *repeats, *maxOffset)
	head := strings.NewReplacer("{{TITLE}}", *mapping, "{{PLAY_ORDER}}", playOrder, "{{ORNAMENTS}}", ornamentsText.String(), "{{DATE}}", time.Now().Format("2006.01.02"), "{{ARGS}}", args).Replace(string(header))
	var module bytes.Buffer
	module.WriteString(head)
	if !strings.HasSuffix(head, "\n") {
		module.WriteByte('\n')
	}
	for i, text := range patterns {
		if used[i] && text != "" {
			fmt.Fprintf(&module, "\n[Pattern%d]\n", i)
			module.WriteString(text)
			if !strings.HasSuffix(text, "\n") {
				module.WriteByte('\n')
			}
		}
	}
	if *vtOut != "" {
		a, _ := filepath.Abs(*midi)
		b, _ := filepath.Abs(*vtOut)
		if a == b {
			return fmt.Errorf("output cannot overwrite MIDI input")
		}
		if e = os.WriteFile(*vtOut, module.Bytes(), 0644); e != nil {
			return e
		}
	}
	notes := make([]virtualNote, u32("note_count", 0))
	for i := range notes {
		id := i + 1
		notes[i] = virtualNote{Track: u16("note_track", id), Source: u32("note_source", id), Channel: u8("note_channel", id), Pitch: u8("note_pitch", id), StartTick: u32("note_start_tick", id), OffTick: u32("note_off_tick", id), Start: u32("note_start", id), Off: u32("note_off", id)}
	}
	data, e = json.MarshalIndent(map[string]any{"protocol": "autosiril-vm-notes-v1", "per_beat": *beat, "sources": u32("source_count", 0), "notes": notes, "flattened": flattened, "lnotes": lnotes, "delayed": delayed, "mixed": mixed, "rows_text": rowsText, "patterns": patterns, "pattern_used": used, "play_order": playOrder, "vt_text": module.String(), "ornaments": ornamentsJSON, "mapping": *mapping, "max_row": u32("max_row", 0), "vm_steps": vm.StepsExecuted(), "heap_bytes": vm.AllocHeap(nil).I, "max_access_address": maxAddress, "target": "mzv", "optimized": *opt, "language": "Nanz + native Crystal frontend HIR"}, "", "  ")
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
