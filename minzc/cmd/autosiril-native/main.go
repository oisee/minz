// autosiril-native is a verification runner for the actual bounded flatten stage.
// Input transport, target execution and output comparison are host work; all
// musical selection and iteration remain Nanz + exported Crystal HIR.
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
const stackFloor = 0xe000
const entrySP = 0xeffe
const returnPC = 0xf100

type config struct {
	Banks   int `json:"banks"`
	Outputs int `json:"outputs"`
}
type note struct {
	Start  uint32 `json:"start"`
	Off    uint32 `json:"off"`
	Pitch  uint32 `json:"pitch"`
	Source uint32 `json:"source"`
}
type event struct {
	Row   uint16 `json:"row"`
	Value uint16 `json:"value"`
}
type testCase struct {
	Name          string            `json:"name"`
	Notes         []note            `json:"notes"`
	Source        uint32            `json:"source"`
	Mode          string            `json:"mode"`
	Limit         uint32            `json:"limit"`
	Banked        bool              `json:"banked"`
	Churn         bool              `json:"churn"`
	Control       uint8             `json:"control"`
	DeclaredCount *uint32           `json:"declared_count,omitempty"`
	ExpectedFault uint16            `json:"expected_fault"`
	Expected      []event           `json:"expected"`
	ExpectedState map[string]uint32 `json:"expected_state,omitempty"`
}
type input struct {
	Config config     `json:"config"`
	Cases  []testCase `json:"cases"`
}
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
type outcome struct {
	Engine   string            `json:"engine"`
	Returned uint16            `json:"returned"`
	Events   []event           `json:"events"`
	State    map[string]uint32 `json:"state"`
	Steps    int               `json:"steps"`
	TStates  int               `json:"t_states,omitempty"`
	Stack    int               `json:"stack_below_entry_bytes,omitempty"`
	PC       uint16            `json:"pc,omitempty"`
	SP       uint16            `json:"sp,omitempty"`
}
type caseResult struct {
	Name    string    `json:"name"`
	Engines []outcome `json:"engines"`
}

var fields = map[string]int{"fault": 2, "output_count": 2, "peak_active": 2, "rows_visited": 4, "yielded": 4, "payload_reads": 4, "stream_busy": 1, "active_count": 2, "bank_switches": 4, "mapping_epoch": 4, "mapped_bank": 2, "yield_row": 2}

func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func checkedConfig(c config) error {
	if c.Banks < 1 || c.Banks > 3 || c.Outputs < 1 || c.Outputs > 1024 {
		return fmt.Errorf("native configuration requires 1..3 banks and 1..1024 outputs")
	}
	return nil
}
func render(source string, c config) (string, error) {
	if err := checkedConfig(c); err != nil {
		return "", err
	}
	capacity := c.Banks * 682
	sink := fmt.Sprintf(`fun emit_flat(row:u16,value:u16){if output_count>=%d{fault=200
return}
output_rows[output_count]=row
output_values[output_count]=value
output_count=output_count+1}`, c.Outputs)
	pairs := [][2]string{
		{"global note_bytes:[u8;49152]", fmt.Sprintf("global note_bytes:[u8;%d]", c.Banks*4096)},
		{"if bank >= 12", fmt.Sprintf("if bank >= %d", c.Banks)},
		{"mapped_bank >= 12", fmt.Sprintf("mapped_bank >= %d", c.Banks)},
		{"id > 8184", fmt.Sprintf("id > %d", capacity)},
		{"if note_count > 8184", fmt.Sprintf("if note_count > %d", capacity)},
		{"(bank + 1) % 12", fmt.Sprintf("(bank + 1) %% %d", c.Banks)},
		{"@extern fun emit_flat(row:u16,value:u16)", sink},
	}
	for _, p := range pairs {
		if strings.Count(source, p[0]) != 1 {
			return "", fmt.Errorf("stage configuration anchor changed: %q", p[0])
		}
		source = strings.Replace(source, p[0], p[1], 1)
	}
	return fmt.Sprintf(`global native_source:u8
global native_mode:u8
global native_limit:u32
global native_control:u8
global output_count:u16
global output_rows:[u16;%d]
global output_values:[u16;%d]
`, c.Outputs, c.Outputs) + source + `
fun native_main()->u16{
 mapped_bank=65535
 if native_control==1{factory_probe()
 return fault}
 if native_control==2{stale_probe()
 return fault}
 validate_input()
 if fault!=0{return fault}
 if native_control==3{return exclusive_probe()}
 if native_control==4{return exhaustion_probe(native_source,native_mode)}
 if native_control==5{
 var first=flattened(native_source,native_mode)
 var value:u16=0
 if pull_next(&first,&value){
 let before:u16=active_count
 consume(native_source,native_mode,1)
 if stream_busy!=1{return 201}
 if active_count!=before{return 202}}
 stream_busy=0
 return fault}
 consume(native_source,native_mode,native_limit)
 return fault
}
`, nil
}

func buildHIR(source, music, meta string) (*hir.Module, error) {
	h, e := nanz.Parse(source, "autosiril-native-bounded.nanz")
	if e != nil {
		return nil, e
	}
	c, e := lanz.Compile(music, "autosiril_native_music")
	if e != nil {
		return nil, e
	}
	if len(c.Globals) != 0 {
		return nil, fmt.Errorf("selector export must not contain global storage")
	}
	var m manifest
	if e = json.Unmarshal([]byte(meta), &m); e != nil {
		return nil, e
	}
	defs := map[string]*hir.Func{}
	calls := map[string][]string{}
	direct := map[string]hir.Effects{}
	for _, f := range c.Funcs {
		if f.Body != nil {
			if defs[f.Name] != nil {
				return nil, fmt.Errorf("duplicate Crystal definition %s", f.Name)
			}
			defs[f.Name] = f
		}
	}
	for _, f := range hir.AnalyzeFacts(c).Functions {
		calls[f.Name] = f.Calls
		direct[f.Name] = f.Direct
	}
	added := map[string]bool{}
	var add func(string) error
	add = func(name string) error {
		if added[name] {
			return nil
		}
		f := defs[name]
		if f == nil || f.IsExtern || f.IsGen || len(f.Captures) != 0 || len(f.RetTys) != 0 || f.IsIO || direct[name]&(hir.ReadsMemory|hir.WritesMemory|hir.DoesIO|hir.UnknownEffect) != 0 {
			return fmt.Errorf("unsupported Crystal dependency %s", name)
		}
		if h.FuncByName(name) != nil {
			return fmt.Errorf("selector symbol collision %s", name)
		}
		added[name] = true
		h.Funcs = append(h.Funcs, f)
		for _, callee := range calls[name] {
			if e := add(callee); e != nil {
				return e
			}
		}
		return nil
	}
	seen := map[string]bool{}
	for _, b := range m.Functions {
		p := h.FuncByName(b.Source)
		if p == nil {
			continue
		}
		if seen[b.Source] {
			return nil, fmt.Errorf("duplicate binding %s", b.Source)
		}
		seen[b.Source] = true
		target := defs[b.Symbol]
		if !p.IsExtern || target == nil || b.Fallible || b.Result != "u8" || len(b.Args) != len(p.Params) || !reflect.DeepEqual(p.RetTy, target.RetTy) || len(p.Params) != len(target.Params) || len(p.RetTys) != 0 || len(target.RetTys) != 0 {
			return nil, fmt.Errorf("incompatible selector binding %s", b.Source)
		}
		args := make([]hir.Expr, len(p.Params))
		for i, v := range p.Params {
			if b.Args[i] != "u8" || v.Ty != mir2.TyU8 || !reflect.DeepEqual(v.Ty, target.Params[i].Ty) {
				return nil, fmt.Errorf("selector parameter mismatch %s", b.Source)
			}
			args[i] = hir.Var(v.Name, v.Ty)
		}
		if e := add(target.Name); e != nil {
			return nil, e
		}
		p.IsExtern = false
		p.Body = hir.Blk(&hir.ReturnStmt{Val: &hir.CallExpr{Fn: target.Name, Args: args, Ty: target.RetTy}})
	}
	for _, f := range h.Funcs {
		if f.IsExtern {
			return nil, fmt.Errorf("unresolved external %s", f.Name)
		}
	}
	for _, f := range hir.AnalyzeFacts(h).Functions {
		for _, call := range f.Calls {
			if strings.HasPrefix(call, "@host.") {
				return nil, fmt.Errorf("host callback forbidden: %s", call)
			}
		}
	}
	h.Target = hir.TargetCPM
	h.NonNullObjects = true
	if e := hir.ResolveErrorABI(h); e != nil {
		return nil, fmt.Errorf("native selector error ABI: %w", e)
	}
	for name := range added {
		if h.FuncByName(name).FallibleCY {
			return nil, fmt.Errorf("fallible actual selector/dependency %s", name)
		}
	}
	return h, hir.Verify(h)
}

func checkCase(tc testCase, c config) error {
	if tc.Name == "" || tc.Source > 255 || (tc.Mode != "p" && tc.Mode != "m" && tc.Mode != "d") || tc.Limit > uint32(c.Outputs) || tc.Control > 5 {
		return fmt.Errorf("invalid case fields: %s", tc.Name)
	}
	if len(tc.Notes) > c.Banks*682 {
		return fmt.Errorf("note capacity %d exceeded", c.Banks*682)
	}
	if tc.DeclaredCount != nil && *tc.DeclaredCount > 65535 {
		return fmt.Errorf("declared count outside u16")
	}
	for i, n := range tc.Notes {
		if n.Start > 65535 || n.Off > 65535 || n.Pitch > 127 || n.Source > 255 {
			return fmt.Errorf("note%d outside compact field range", i)
		}
	}
	for name := range tc.ExpectedState {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("unknown expected state field %s", name)
		}
	}
	if len(tc.Expected) > c.Outputs {
		return fmt.Errorf("expected output exceeds sink capacity")
	}
	return nil
}
func initialize(tc testCase, c config, write func(string, int, byte)) []byte {
	put := func(name string, value uint32, width int) {
		for i := 0; i < width; i++ {
			write(name, i, byte(value>>uint(i*8)))
		}
	}
	count := uint32(len(tc.Notes))
	if tc.DeclaredCount != nil {
		count = *tc.DeclaredCount
	}
	put("note_count", count, 2)
	put("native_source", tc.Source, 1)
	put("native_mode", uint32(tc.Mode[0]), 1)
	put("native_limit", tc.Limit, 4)
	put("native_control", uint32(tc.Control), 1)
	if tc.Banked {
		put("banked", 1, 1)
	}
	if tc.Churn {
		put("churn", 1, 1)
	}
	backing := make([]byte, c.Banks*4096)
	for bank := 0; bank < c.Banks; bank++ {
		for j := 4092; j < 4096; j++ {
			backing[bank*4096+j] = 0xda
		}
	}
	for i, n := range tc.Notes {
		at := (i/682)*4096 + (i%682)*6
		binary.LittleEndian.PutUint16(backing[at:], uint16(n.Start))
		binary.LittleEndian.PutUint16(backing[at+2:], uint16(n.Off))
		backing[at+4] = byte(n.Pitch)
		backing[at+5] = byte(n.Source)
	}
	for i, v := range backing {
		write("note_bytes", i, v)
	}
	return backing
}
func collect(engine string, ret uint16, c config, read func(string, int, int) []byte) (outcome, error) {
	o := outcome{Engine: engine, Returned: ret, Events: []event{}, State: map[string]uint32{}}
	for name, width := range fields {
		b := read(name, 0, width)
		var v uint32
		for i, x := range b {
			v |= uint32(x) << uint(i*8)
		}
		o.State[name] = v
	}
	if o.State["output_count"] > uint32(c.Outputs) {
		return o, fmt.Errorf("sink count exceeds capacity")
	}
	for i := 0; i < int(o.State["output_count"]); i++ {
		row := binary.LittleEndian.Uint16(read("output_rows", i*2, 2))
		value := binary.LittleEndian.Uint16(read("output_values", i*2, 2))
		o.Events = append(o.Events, event{row, value})
	}
	return o, nil
}
func executeVM(m *mir2.Module, tc testCase, c config, engine string) (outcome, error) {
	v := mir2.NewVM(m)
	v.MaxMemory = 65536
	v.MaxSteps = 200000000
	backing := initialize(tc, c, func(name string, off int, b byte) { v.WriteHeap(v.GlobalAddr(name)+int64(off), b) })
	result, e := v.Call("native_main", nil)
	if e != nil {
		return outcome{}, e
	}
	if len(result) != 1 {
		return outcome{}, fmt.Errorf("invalid VM result")
	}
	if !bytes.Equal(v.ReadHeap(v.GlobalAddr("note_bytes"), len(backing)), backing) {
		return outcome{}, fmt.Errorf("VM mutated read-only note arena")
	}
	o, e := collect(engine, uint16(result[0].I), c, func(name string, off, n int) []byte { return v.ReadHeap(v.GlobalAddr(name)+int64(off), n) })
	o.Steps = int(v.StepsExecuted())
	return o, e
}

type dataSpan struct {
	Name    string `json:"name"`
	Address int    `json:"address"`
	Bytes   int    `json:"bytes"`
}

// Backend spills are explicit zero-byte declarations interspersed with code.
// Admit only their exact declared extents, never a blanket writable code range.
func backendSpills(assembly string, symbols map[string]int, image []byte) ([]dataSpan, error) {
	spans := []dataSpan{}
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
			return nil, fmt.Errorf("unknown backend spill declaration %s", name)
		}
		values := strings.Split(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(rest, "DB "), "DW ")), ",")
		if len(values) < 1 || len(values)*width > 4 {
			return nil, fmt.Errorf("invalid backend spill width %s", name)
		}
		for _, v := range values {
			if strings.TrimSpace(v) != "0" {
				return nil, fmt.Errorf("nonzero backend spill initializer %s", name)
			}
		}
		at, ok := symbols[name]
		if !ok || at < origin || at+len(values)*width > origin+len(image) {
			return nil, fmt.Errorf("invalid backend spill address %s", name)
		}
		for i := 0; i < len(values)*width; i++ {
			if image[at-origin+i] != 0 {
				return nil, fmt.Errorf("backend spill bytes disagree with declaration")
			}
		}
		spans = append(spans, dataSpan{name, at, len(values) * width})
	}
	return spans, nil
}

func executeNative(binaryImage []byte, symbols map[string]int, globals []mir2.Global, spills []dataSpan, tc testCase, c config) (outcome, error) {
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	p := emulator.NewProfiler()
	z.SetProfiler(p)
	mem := z.Memory()
	for i := range mem {
		mem[i] = 0xa5
	}
	copy(mem[origin:], binaryImage)
	backing := initialize(tc, c, func(name string, off int, b byte) { mem[symbols[name]+off] = b })
	mem[entrySP] = 0
	mem[entrySP+1] = 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(symbols["native_main"]), SP: entrySP})
	minSP := uint16(entrySP)
	steps := 0
	for ; steps < 200000000 && z.GetPC() != returnPC; steps++ {
		z.Step()
		if z.GetSP() < minSP {
			minSP = z.GetSP()
		}
		if z.GetSP() < stackFloor {
			return outcome{}, fmt.Errorf("native stack exceeded reserved region")
		}
	}
	r := z.GetRegisters()
	if len(p.IORead) != 0 || len(p.IOWrite) != 0 {
		return outcome{}, fmt.Errorf("unexpected native port I/O")
	}
	if r.PC != returnPC || r.SP != entrySP+2 {
		return outcome{}, fmt.Errorf("native return/stack PC=%04x SP=%04x", r.PC, r.SP)
	}
	for i := origin - 64; i < origin; i++ {
		if mem[i] != 0xa5 {
			return outcome{}, fmt.Errorf("lower image canary changed")
		}
	}
	for i := origin + len(binaryImage); i < origin+len(binaryImage)+64; i++ {
		if mem[i] != 0xa5 {
			return outcome{}, fmt.Errorf("upper image canary changed")
		}
	}
	for i := stackFloor - 64; i < stackFloor; i++ {
		if mem[i] != 0xa5 {
			return outcome{}, fmt.Errorf("stack floor canary changed")
		}
	}
	at := symbols["note_bytes"]
	if !bytes.Equal(mem[at:at+len(backing)], backing) {
		return outcome{}, fmt.Errorf("native mutated read-only note arena")
	}
	for i := range backing {
		if p.WriteCount[at+i] != 0 {
			return outcome{}, fmt.Errorf("native wrote read-only note arena")
		}
	}
	allowed := make([]bool, 65536)
	for _, g := range globals {
		if g.At != nil {
			return outcome{}, fmt.Errorf("unexpected absolute global")
		}
		at, ok := symbols[g.Name]
		if !ok {
			return outcome{}, fmt.Errorf("missing native global %s", g.Name)
		}
		for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
			allowed[at+i] = true
			if p.ExecCount[at+i] != 0 {
				return outcome{}, fmt.Errorf("executed declared global %s", g.Name)
			}
		}
	}
	for _, span := range spills {
		for i := 0; i < span.Bytes; i++ {
			allowed[span.Address+i] = true
			if p.ExecCount[span.Address+i] != 0 {
				return outcome{}, fmt.Errorf("executed backend data %s", span.Name)
			}
		}
	}
	for i := stackFloor; i < entrySP; i++ {
		allowed[i] = true
	}
	for i, writes := range p.WriteCount {
		if writes != 0 && !allowed[i] {
			return outcome{}, fmt.Errorf("native write outside declared globals/stack at %04x", i)
		}
	}
	o, e := collect("z80", r.HL, c, func(name string, off, n int) []byte { return mem[symbols[name]+off : symbols[name]+off+n] })
	o.Steps = steps
	o.TStates = z.GetCycles()
	o.Stack = int(entrySP - minSP)
	o.PC = r.PC
	o.SP = r.SP
	return o, e
}

func run(engine, musicPath, metaPath, inputPath, outDir string) error {
	if outDir == "" {
		return fmt.Errorf("-out is required")
	}
	if e := os.MkdirAll(outDir, 0755); e != nil {
		return e
	}
	source, e := os.ReadFile(engine)
	if e != nil {
		return e
	}
	music, e := os.ReadFile(musicPath)
	if e != nil {
		return e
	}
	meta, e := os.ReadFile(metaPath)
	if e != nil {
		return e
	}
	data, e := os.ReadFile(inputPath)
	if e != nil {
		return e
	}
	var in input
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e = d.Decode(&in); e != nil {
		return e
	}
	if e = checkedConfig(in.Config); e != nil {
		return e
	}
	if len(in.Cases) == 0 {
		return fmt.Errorf("no cases")
	}
	names := map[string]bool{}
	for _, tc := range in.Cases {
		if names[tc.Name] {
			return fmt.Errorf("duplicate case name")
		}
		names[tc.Name] = true
		if e = checkCase(tc, in.Config); e != nil {
			return e
		}
	}
	rendered, e := render(string(source), in.Config)
	if e != nil {
		return e
	}
	makeHIR := func() (*hir.Module, error) { return buildHIR(rendered, string(music), string(meta)) }
	h, e := makeHIR()
	if e != nil {
		return e
	}
	raw := hir.LowerModule(h)
	if e = mir2.Verify(raw); e != nil {
		return e
	}
	h, e = makeHIR()
	if e != nil {
		return e
	}
	opt := pipeline.DefaultOptions()
	opt.AssertMode = "none"
	opt.DisableVariantSearch = true
	steps, e := pipeline.CompileHIRSteps(h, opt)
	if e != nil {
		return e
	}
	a, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x%x\n", origin) + steps.Assembly)
	if e != nil || len(a.Errors) != 0 {
		return fmt.Errorf("native assembly: %v %v", e, a.Errors)
	}
	if origin+len(a.Binary)+64 >= stackFloor {
		return fmt.Errorf("native image does not fit below reserved stack")
	}
	spills, e := backendSpills(steps.Assembly, a.Symbols, a.Binary)
	if e != nil {
		return e
	}
	spillBytes := 0
	for _, span := range spills {
		spillBytes += span.Bytes
	}
	dataBytes := 0
	for _, g := range steps.MIR2Module.Globals {
		dataBytes += mir2.ByteWidth(g.Ty)
	}
	// Preserve exact compiler products even when a runtime guard fails.
	for name, b := range map[string][]byte{"stage.nanz": []byte(rendered), "program.a80": []byte(steps.Assembly), "program.bin": a.Binary, "raw.mir": []byte(raw.Dump()), "final.mir": []byte(steps.MIR2Module.Dump())} {
		if e = os.WriteFile(filepath.Join(outDir, name), b, 0644); e != nil {
			return e
		}
	}
	results := []caseResult{}
	for _, tc := range in.Cases {
		r := caseResult{Name: tc.Name}
		for _, entry := range []struct {
			name string
			m    *mir2.Module
		}{{"raw-mir2", raw}, {"final-mir2", steps.MIR2Module}} {
			o, e := executeVM(entry.m, tc, in.Config, entry.name)
			if e != nil {
				return fmt.Errorf("%s/%s: %w", tc.Name, entry.name, e)
			}
			r.Engines = append(r.Engines, o)
		}
		o, e := executeNative(a.Binary, a.Symbols, steps.MIR2Module.Globals, spills, tc, in.Config)
		if e != nil {
			return fmt.Errorf("%s/z80: %w", tc.Name, e)
		}
		r.Engines = append(r.Engines, o)
		for _, o := range r.Engines {
			if o.Returned != tc.ExpectedFault || o.State["fault"] != uint32(tc.ExpectedFault) {
				return fmt.Errorf("%s/%s returned%d fault%d want%d", tc.Name, o.Engine, o.Returned, o.State["fault"], tc.ExpectedFault)
			}
			if len(o.Events) != len(tc.Expected) {
				return fmt.Errorf("%s/%s events%d want%d", tc.Name, o.Engine, len(o.Events), len(tc.Expected))
			}
			for i, v := range o.Events {
				if v != tc.Expected[i] {
					return fmt.Errorf("%s/%s event%d %+v want%+v", tc.Name, o.Engine, i, v, tc.Expected[i])
				}
			}
			for name, want := range tc.ExpectedState {
				if got := o.State[name]; got != want {
					return fmt.Errorf("%s/%s independent state %s=%d want%d", tc.Name, o.Engine, name, got, want)
				}
			}
			if !reflect.DeepEqual(o.State, r.Engines[0].State) {
				return fmt.Errorf("%s/%s state mismatch: %v vs %v", tc.Name, o.Engine, o.State, r.Engines[0].State)
			}
		}
		results = append(results, r)
		fmt.Printf("PASS %s: %d events, %dT, stack%d\n", tc.Name, len(o.Events), o.TStates, o.Stack)
	}
	receipt := map[string]any{"protocol": "autosiril-native-flat/v1", "config": in.Config, "note_capacity": in.Config.Banks * 682, "active_capacity": 512, "origin": origin, "image_end": origin + len(a.Binary), "stack_floor": stackFloor, "entry_sp": entrySP, "image_bytes": len(a.Binary), "module_globals_bytes": dataBytes, "code_bytes": len(a.Binary) - dataBytes - spillBytes, "backend_static_spill_bytes": spillBytes, "backend_spills": spills, "engine_sha256": digest(source), "rendered_sha256": digest([]byte(rendered)), "music_sha256": digest(music), "manifest_sha256": digest(meta), "input_sha256": digest(data), "assembly_sha256": digest([]byte(steps.Assembly)), "binary_sha256": digest(a.Binary), "options": map[string]any{"UseGrace": opt.UseGrace, "ContractOpt": opt.ContractOpt, "DisableVariantSearch": true, "AssertMode": "none"}, "scope": "bounded paired notes -> flattened events, actual Nanz generator/Crystal selectors; no MIDI parser, VT2 writer, whole-song claim or physical bank mapping; code_bytes excludes declared MIR globals and exact backend spill declarations", "cases": results}
	b, e := json.MarshalIndent(receipt, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(outDir, "receipt.json"), append(b, '\n'), 0644)
}
func main() {
	engine := flag.String("engine", "research/autosiril-vm/lazy_flatten.nanz", "actual stage source")
	music := flag.String("music", "", "actual exported Crystal LANZ")
	meta := flag.String("manifest", "", "native exporter manifest")
	inputs := flag.String("input", "", "case matrix including Ruby expected events")
	out := flag.String("out", "", "external output directory")
	flag.Parse()
	if e := run(*engine, *music, *meta, *inputs, *out); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
