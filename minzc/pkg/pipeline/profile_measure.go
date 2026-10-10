package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// LoopCostRow records entry-through-RET timings, excluding bootstrap instructions.
type LoopCostRow struct {
	N                       int    `json:"n"`
	Base                    int64  `json:"base"`
	Path                    string `json:"path"`
	Original                int    `json:"original_t_states"`
	Pointer                 int    `json:"pointer_t_states"`
	Result                  int64  `json:"result"`
	OriginalExecutionSHA256 string `json:"original_execution_sha256"`
	PointerExecutionSHA256  string `json:"pointer_execution_sha256"`
}

// LoopCostEvidence is the JSON payload hashed by the model's EvidenceID.
// Program bytes and per-case execution binary digests bind assembled output.
// Binary sizes cover whole modules. Coverage includes every u8 length at four
// address families, not every address or byte pattern; HIR establishes legality.
type LoopCostEvidence struct {
	Schema         string                    `json:"schema"`
	Compatibility  *hir.ProfileCompatibility `json:"compatibility"`
	Function       string                    `json:"function"`
	SiteID         string                    `json:"site_id"`
	CompilationID  string                    `json:"compilation_id"`
	Options        Options                   `json:"options"`
	AssemblerMode  string                    `json:"assembler_mode"`
	OriginalBinary []byte                    `json:"original_binary"`
	PointerBinary  []byte                    `json:"pointer_binary"`
	Rows           []LoopCostRow             `json:"rows"`
}

// MeasureLoopCostModel measures one guarded scalar u8 sum idiom
// recognized by the HIR profiler, keeping every other site original.
// Independent models do not validate simultaneous rewrites of multiple sites.
// Factories must produce independent equivalent input HIR. Each variant is compiled once using the supplied options and default
// Z80 assembler. All 1022 cases must agree with an independent sum oracle, raw
// MIR2 VM, actual Z80 memory and guard branches, and a fitted linear cost model.
func MeasureLoopCostModel(factory func() (*hir.Module, error), function string, workload hir.ProfileWorkload, options Options) (hir.LoopCostModel, *LoopCostEvidence, error) {
	fail := func(err error) (hir.LoopCostModel, *LoopCostEvidence, error) {
		return hir.LoopCostModel{}, nil, fmt.Errorf("measure loop cost: %w", err)
	}
	if factory == nil || function == "" || (options.Backend != "" && options.Backend != "z80") || options.SkipZ80Emission || options.ProfileSelection != nil {
		return fail(fmt.Errorf("unsupported factory/function/backend/configuration"))
	}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	modules := make([]*hir.Module, 3)
	seen := map[uintptr]bool{}
	var key *hir.ProfileCompatibility
	var context hir.ProfileContext
	for i := range modules {
		m, e := factory()
		if e != nil {
			return fail(e)
		}
		if m == nil {
			return fail(fmt.Errorf("nil factory module"))
		}
		if m.PointerWalkPolicy != nil || m.LoopLineage != nil {
			return fail(fmt.Errorf("factory must return input HIR without policy/lineage"))
		}
		current := map[uintptr]bool{}
		traversed := map[uintptr]bool{}
		var walk func(reflect.Value)
		walk = func(v reflect.Value) {
			if !v.IsValid() {
				return
			}
			if v.Kind() == reflect.Interface {
				if !v.IsNil() {
					walk(v.Elem())
				}
				return
			}
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					return
				}
				if traversed[v.Pointer()] {
					return
				}
				traversed[v.Pointer()] = true
				ast := false
				switch v.Interface().(type) {
				case *hir.Module, *hir.Func, hir.Stmt, hir.Expr:
					ast = true
				}
				if ast {
					if current[v.Pointer()] {
						return
					}
					current[v.Pointer()] = true
				}
				walk(v.Elem())
				return
			}
			switch v.Kind() {
			case reflect.Struct:
				for j := 0; j < v.NumField(); j++ {
					if v.Field(j).CanInterface() {
						walk(v.Field(j))
					}
				}
			case reflect.Slice, reflect.Array:
				for j := 0; j < v.Len(); j++ {
					walk(v.Index(j))
				}
			}
		}
		walk(reflect.ValueOf(m))
		for p := range current {
			if seen[p] {
				return fail(fmt.Errorf("factory returned shared HIR nodes"))
			}
			seen[p] = true
		}
		c, e := hir.BuildWorkloadProfileContext(m, workload)
		if e != nil {
			return fail(e)
		}
		k, e := hir.BuildProfileCompatibility(m, c)
		if e != nil {
			return fail(e)
		}
		if i == 0 {
			key = k
			context = c
		} else {
			a, _ := json.Marshal(key)
			b, _ := json.Marshal(k)
			if string(a) != string(b) {
				return fail(fmt.Errorf("factory returned non-equivalent HIR/context"))
			}
		}
		modules[i] = m
	}
	p, err := hir.NewLoopProfiler(modules[0])
	if err != nil {
		return fail(err)
	}
	snapshot := p.Snapshot()
	if snapshot.Sites.UnmappedLoops != 0 {
		return fail(fmt.Errorf("requires mapped input sites"))
	}
	var selectedSite string
	for _, site := range snapshot.Sites.Sites {
		if site.Function == function {
			if selectedSite != "" {
				return fail(fmt.Errorf("requires one loop in measured function"))
			}
			selectedSite = site.ID
		}
	}
	eligible := false
	for _, guard := range snapshot.Guards {
		if guard.SiteID == selectedSite {
			eligible = true
		}
	}
	if selectedSite == "" || !eligible {
		return fail(fmt.Errorf("requires eligible mapped guarded sum site"))
	}
	f := modules[0].FuncByName(function)
	if f == nil || len(f.Params) != 2 || len(f.Captures) != 0 || f.RetTy != mir2.TyU8 || len(f.RetTys) != 0 || f.Params[0].Ty != mir2.TyPtr || f.Params[1].Ty != mir2.TyU8 || f.Params[0].SMC || f.Params[1].SMC {
		return fail(fmt.Errorf("requires pointer,u8 parameters and u8 sum result"))
	}
	// The guard recognizer allows arbitrary returns; require the exact sum return.
	decl, ok := f.Body.Body[0].(*hir.VarDeclStmt)
	if !ok {
		return fail(fmt.Errorf("missing accumulator"))
	}
	ret, ok := f.Body.Body[3].(*hir.ReturnStmt)
	if !ok {
		return fail(fmt.Errorf("missing sum return"))
	}
	ref, ok := ret.Val.(*hir.VarRefExpr)
	if !ok || ref.Name != decl.Name || ref.Ty != mir2.TyU8 {
		return fail(fmt.Errorf("requires exact accumulator return"))
	}
	site := selectedSite
	evidence := &LoopCostEvidence{Schema: "minz.measured-u8-sum/v1", Compatibility: key, Function: function, SiteID: site, CompilationID: config, Options: options, AssemblerMode: "default", Rows: []LoopCostRow{}}
	evidence.Options.GraceStats = nil
	variants := make([]measuredLoopVariant, 2)
	for i, variant := range []string{"original", "pointer-walk"} {
		m := modules[i+1]
		decisions := make([]hir.ProfileDecision, 0, len(snapshot.Sites.Sites))
		for _, inputSite := range snapshot.Sites.Sites {
			choice := "original"
			if inputSite.ID == site {
				choice = variant
			}
			decisions = append(decisions, hir.ProfileDecision{Compatibility: key, SiteID: inputSite.ID, Variant: choice})
		}
		if err := hir.ApplyProfileDecisions(m, decisions); err != nil {
			return fail(err)
		}
		raw := hir.LowerModule(m)
		if err := mir2.Verify(raw); err != nil {
			return fail(err)
		}
		steps, err := CompileHIRSteps(m, options)
		if err != nil {
			return fail(err)
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			return fail(err)
		}
		if len(assembled.Errors) > 0 || len(assembled.Binary) == 0 {
			return fail(fmt.Errorf("program assembly failed: %v", assembled.Errors))
		}
		variants[i] = measuredLoopVariant{module: m, raw: raw, steps: steps}
		if i == 0 {
			evidence.OriginalBinary = assembled.Binary
		} else {
			evidence.PointerBinary = assembled.Binary
		}
	}
	for _, path := range []string{"fast", "equality", "fallback", "wrap"} {
		for n := 0; n < 256; n++ {
			if n == 0 && (path == "fallback" || path == "wrap") {
				continue
			}
			base := int64(0x4000)
			switch path {
			case "equality":
				base = 65535 - int64(n)
			case "fallback":
				base = 65536 - int64(n)
			case "wrap":
				base = 65535
			}
			mem := map[uint16]byte{}
			want := int64(0)
			for j := 0; j < n; j++ {
				b := byte(j*37 + 11)
				mem[uint16(base+int64(j))] = b
				want = (want + int64(b)) & 255
			}
			row := LoopCostRow{N: n, Base: base, Path: path, Result: want}
			for i, v := range variants {
				vm := mir2.NewVM(v.raw)
				vm.Hosts = map[string]mir2.HostFunc{}
				vm.EnsureHeap(65536)
				for a, b := range mem {
					vm.WriteHeap(int64(a), b)
				}
				result, err := vm.Call(function, []mir2.Value{{I: base}, {I: int64(n)}})
				if err != nil || len(result) != 1 || result[0].I != want {
					return fail(fmt.Errorf("%s n=%d variant=%d raw VM sum mismatch: %v %v", path, n, i, result, err))
				}
				for a, b := range mem {
					if vm.ReadHeap(int64(a), 1)[0] != b {
						return fail(fmt.Errorf("raw VM changed input"))
					}
				}
				var executionDigest string
				cycles, err := measureLoopExecution(v, function, []int64{base, int64(n)}, mem, want, i == 1, &executionDigest)
				if err != nil {
					return fail(fmt.Errorf("%s n=%d variant=%d: %w", path, n, i, err))
				}
				if i == 0 {
					row.Original = cycles
					row.OriginalExecutionSHA256 = executionDigest
				} else {
					row.Pointer = cycles
					row.PointerExecutionSHA256 = executionDigest
				}
			}
			evidence.Rows = append(evidence.Rows, row)
		}
	}
	model, err := fitMeasuredLoopRows(evidence.Rows)
	if err != nil {
		return fail(err)
	}
	model.CompilerID = context.CompilerID
	model.SemanticsID = context.SemanticsID
	model.TargetID = context.TargetID
	model.Function = function
	model.HIRSHA256 = key.HIRSHA256
	model.CompilationID = config
	model.OriginalBytes = uint64(len(evidence.OriginalBinary))
	model.PointerBytes = uint64(len(evidence.PointerBinary))
	data, err := json.Marshal(evidence)
	if err != nil {
		return fail(err)
	}
	model.EvidenceID = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	return model, evidence, nil
}

type measuredLoopVariant struct {
	module *hir.Module
	raw    *mir2.Module
	steps  Steps
}

func measureLoopExecution(v measuredLoopVariant, name string, args []int64, memory map[uint16]byte, want int64, pointer bool, executionDigest *string) (int, error) {
	f := v.steps.MIR2Module.FuncByName(name)
	if f == nil {
		return 0, fmt.Errorf("compiled function missing")
	}
	a := hir.Assert{FuncName: name, Args: args, Expected: want, Via: "z80"}
	boot := buildAssertBootstrap(assertLoadAddr, a, f, v.steps.Allocation)
	r, err := z80asm.NewAssembler().AssembleString(boot + "\n" + v.steps.Assembly)
	if err != nil {
		return 0, err
	}
	if len(r.Errors) > 0 {
		return 0, fmt.Errorf("execution assembly: %v", r.Errors)
	}
	*executionDigest = fmt.Sprintf("%x", sha256.Sum256(r.Binary))
	pc, ok := r.Symbols[name]
	if !ok {
		pc, ok = r.Symbols[strings.ToUpper(name)]
	}
	if !ok {
		return 0, fmt.Errorf("function symbol missing")
	}
	for addr := range memory {
		if int(addr) >= assertLoadAddr && int(addr) < assertLoadAddr+len(r.Binary) || addr >= 0xFDF0 && addr < 0xFF00 {
			return 0, fmt.Errorf("input overlaps code/stack at %04x", addr)
		}
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	z := platform.RemogattoZ80
	if err := z.LoadMemory(assertLoadAddr, r.Binary); err != nil {
		return 0, err
	}
	for addr, b := range memory {
		z.SetMemory(addr, b)
	}
	z.SetPC(assertLoadAddr)
	for i := 0; z.GetPC() != uint16(pc); i++ {
		if i >= 1000 {
			return 0, fmt.Errorf("bootstrap did not reach function")
		}
		z.Step()
	}
	sp := z.GetSP()
	returnPC := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
	start := z.GetCycles()
	visited := map[uint16]bool{}
	for i := 0; ; i++ {
		if i >= 100000 {
			return 0, fmt.Errorf("function did not return")
		}
		visited[z.GetPC()] = true
		z.Step()
		if z.GetPC() == returnPC && z.GetSP() == sp+2 {
			break
		}
	}
	cycles := z.GetCycles() - start
	if pointer {
		then, otherwise := false, false
		for symbol, address := range r.Symbols {
			lower := strings.ToLower(symbol)
			prefix := strings.ToLower(name) + "_if_"
			fast := args[0] <= 65535-args[1]
			if strings.Contains(lower, prefix+"then") {
				then = true
				if visited[uint16(address)] != fast {
					return 0, fmt.Errorf("fast guard branch mismatch")
				}
			}
			if strings.Contains(lower, prefix+"else") {
				otherwise = true
				if visited[uint16(address)] == fast {
					return 0, fmt.Errorf("fallback guard branch mismatch")
				}
			}
		}
		if !then || !otherwise {
			return 0, fmt.Errorf("guard branch symbols unavailable")
		}
	}
	for addr, b := range memory {
		if z.GetMemory(addr) != b {
			return 0, fmt.Errorf("Z80 changed input at %04x", addr)
		}
	}
	if err := checkAssertZ80Result(z, a, f, map[string]*hir.Func{name: v.module.FuncByName(name)}); err != nil {
		return 0, err
	}
	return cycles, nil
}
func fitMeasuredLoopRows(rows []LoopCostRow) (hir.LoopCostModel, error) {
	var m hir.LoopCostModel
	if len(rows) != 1022 {
		return m, fmt.Errorf("incomplete measured domain")
	}
	find := func(path string, n int) LoopCostRow {
		for _, r := range rows {
			if r.Path == path && r.N == n {
				return r
			}
		}
		return LoopCostRow{}
	}
	zero, one := find("fast", 0), find("fast", 1)
	f1, f2 := find("fallback", 1), find("fallback", 2)
	coefficients := []int{zero.Original, one.Original - zero.Original, zero.Pointer, one.Pointer - zero.Pointer, find("equality", 0).Pointer - zero.Pointer, f1.Pointer - (f2.Pointer - f1.Pointer), f2.Pointer - f1.Pointer}
	for _, c := range coefficients {
		if c < 0 {
			return m, fmt.Errorf("negative fitted coefficient")
		}
	}
	m.OriginalEntry = uint64(coefficients[0])
	m.OriginalIteration = uint64(coefficients[1])
	m.FastEntry = uint64(coefficients[2])
	m.FastIteration = uint64(coefficients[3])
	m.EqualEntryExtra = uint64(coefficients[4])
	m.FallbackEntry = uint64(coefficients[5])
	m.FallbackIteration = uint64(coefficients[6])
	m.FallbackMeasured = true
	seen := map[string]bool{}
	for _, r := range rows {
		id := fmt.Sprintf("%s/%d", r.Path, r.N)
		if seen[id] || r.N < 0 || r.N > 255 {
			return hir.LoopCostModel{}, fmt.Errorf("invalid measured domain")
		}
		seen[id] = true
		want := m.FastEntry + m.FastIteration*uint64(r.N)
		switch r.Path {
		case "equality":
			want += m.EqualEntryExtra
		case "fallback", "wrap":
			if r.N == 0 {
				return hir.LoopCostModel{}, fmt.Errorf("invalid fallback zero")
			}
			want = m.FallbackEntry + m.FallbackIteration*uint64(r.N)
		case "fast":
		default:
			return hir.LoopCostModel{}, fmt.Errorf("unknown measured path")
		}
		if r.Original < 0 || r.Pointer < 0 || uint64(r.Original) != m.OriginalEntry+m.OriginalIteration*uint64(r.N) || uint64(r.Pointer) != want {
			return hir.LoopCostModel{}, fmt.Errorf("nonlinear measured cost at %s", id)
		}
	}
	return m, nil
}
