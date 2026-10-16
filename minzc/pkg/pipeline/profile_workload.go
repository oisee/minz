package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// Z80ProfileInvocationEvidence measures entry through RET, excluding bootstrap,
// and owns the declared memory observed at that return.
type Z80ProfileInvocationEvidence struct {
	Access   *hir.ProfileAccessSnapshot `json:"access,omitempty"`
	Function string                     `json:"function"`
	TStates  int64                      `json:"t_states"`
	Results  []mir2.Value               `json:"results"`
	Snapshot hir.ProfileMemorySnapshot  `json:"snapshot"`
}

// Z80ProfileWorkloadEvidence exists only after the complete execution agrees with
// the VM oracle. The program and ordered ABI bootstraps have separate digests.
type Z80ProfileWorkloadEvidence struct {
	Compatibility     *hir.ProfileCompatibility      `json:"compatibility"`
	BinarySHA256      string                         `json:"binary_sha256"`
	BootstrapSHA256   []string                       `json:"bootstrap_sha256"`
	Invocations       []Z80ProfileInvocationEvidence `json:"invocations"`
	Globals           map[string][]byte              `json:"globals"`
	Resources         map[string][]byte              `json:"resources"`
	ResourceAddresses map[string]int                 `json:"resource_addresses"`
	GlobalAddresses   map[string]int                 `json:"global_addresses"`
	BootstrapTStates  int64                          `json:"bootstrap_t_states"`
	TotalTStates      int64                          `json:"total_t_states"` // caller entry→RET; bootstrap costs separate
	ExecutedSteps     int64                          `json:"executed_steps"`
}

const (
	workloadProgram      = 0x8000
	workloadBootstrap    = 0x100
	workloadBootstrapEnd = 0x400
	workloadStack        = 0xf000 // bounded stack and backend absolute-local storage
	workloadStackEnd     = 0xff00
	workloadStepLimit    = 10000000
)

// RunZ80ProfileWorkload runs compiled production assembly in persistent RAM.
// source must be independent, unmodified input HIR; steps may contain transforms.
// MaxSteps bounds cumulative Z80 instructions including bootstraps. Requests above
// the 10M-instruction harness ceiling are refused, never silently capped.
// MaxMemory charges program, resources, and reserved bootstrap/stack storage.
// Numeric pointers, pointer-bearing globals/results, hosts, ports, ASM and SMC
// are unsupported. Unreachable extern declarations retain their full source ABI;
// only the exact direct-call closure has native instruction-fetch authority. Resources are opaque byte buffers, not pointer tables.
// The supplied expected run is a caller-owned VM oracle, not an attestation;
// callers must obtain it from RunLoopProfile for the same input HIR/workload.
// A complete memory snapshot is required for every call, including void calls;
// legacy final-state-only oracles are refused.
// Any failure, including bounds or oracle mismatch, returns nil evidence.
func RunZ80ProfileWorkload(steps Steps, source *hir.Module, workload hir.ProfileWorkload, expected hir.ProfileRun) (*Z80ProfileWorkloadEvidence, error) {
	fail := func(err error) (*Z80ProfileWorkloadEvidence, error) {
		return nil, fmt.Errorf("Z80 profile workload: %w", err)
	}
	if source == nil || source.OrderedByteRWPolicy != nil || source.OrderedBytePolicy != nil || source.RegionComparePolicy != nil || source.ForwardCopyPolicy != nil || source.PointerWalkPolicy != nil || source.LoopLineage != nil || source.GraceRulePolicy != nil || source.InvariantBasePolicy != nil || steps.MIR2Module == nil || steps.Allocation == nil || strings.TrimSpace(steps.Assembly) == "" {
		return fail(fmt.Errorf("requires unmodified input HIR and compiled Z80 steps/allocation"))
	}
	owned, err := json.Marshal(workload)
	if err != nil {
		return fail(err)
	}
	var ownedWorkload hir.ProfileWorkload
	if err := json.Unmarshal(owned, &ownedWorkload); err != nil {
		return fail(err)
	}
	// Own the oracle independently as well as the workload.
	oracleBytes, err := json.Marshal(expected)
	if err != nil {
		return fail(err)
	}
	var ownedOracle hir.ProfileRun
	if err := json.Unmarshal(oracleBytes, &ownedOracle); err != nil {
		return fail(err)
	}
	expected = ownedOracle
	workload = ownedWorkload
	context, err := hir.BuildWorkloadProfileContext(source, workload)
	if err != nil {
		return fail(err)
	}
	if workload.MaxSteps > workloadStepLimit {
		return fail(fmt.Errorf("MaxSteps exceeds harness instruction limit"))
	}
	data, err := json.Marshal(expected.Profile)
	if err != nil {
		return fail(err)
	}
	profile, err := hir.LoadLoopProfile(bytes.NewReader(data), source, context)
	if err != nil {
		return fail(err)
	}
	if profile.FailedCalls != 0 || profile.CompletedCalls != uint64(len(workload.Calls)) || len(expected.Results) != len(workload.Calls) {
		return fail(fmt.Errorf("incomplete oracle run"))
	}
	if workload.ExactDataAccess && len(expected.Accesses) != len(workload.Calls) {
		return fail(fmt.Errorf("incomplete oracle access snapshots"))
	}
	if len(expected.Snapshots) != len(workload.Calls) {
		return fail(fmt.Errorf("incomplete oracle invocation snapshots"))
	}
	if err := hir.ValidateProfileWorkloadMIR(source, steps.MIR2Module, workload); err != nil {
		return fail(err)
	}
	sourceReachable, err := hir.ProfileWorkloadReachableFunctions(hir.LowerModule(source), workload)
	if err != nil {
		return fail(err)
	}
	candidateReachable, err := hir.ProfileWorkloadReachableFunctions(steps.MIR2Module, workload)
	if err != nil {
		return fail(err)
	}
	for _, module := range []*mir2.Module{hir.LowerModule(source), steps.MIR2Module} {
		for _, g := range module.Globals {
			if g.Storage != mir2.StorageNormal || g.At != nil || len(g.VtableSyms) != 0 || len(g.Relocs) != 0 || workloadPointerStorage(g.Ty) {
				return fail(fmt.Errorf("unsupported global storage/pointer %q", g.Name))
			}
		}
		for _, f := range module.Funcs {
			deadRuntime := !sourceReachable[f.Name] && !candidateReachable[f.Name] && hir.ProfileDeadPascalRuntime(source, f)
			if f.Attrs.HasAsm && !deadRuntime {
				return fail(fmt.Errorf("hosts/ASM unsupported: %s", f.Name))
			}
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					switch i.Op {
					case mir2.OpAsm:
						if !deadRuntime {
							return fail(fmt.Errorf("noncanonical/reachable ASM unsupported"))
						}
						continue
					case mir2.OpIn8, mir2.OpIn16, mir2.OpOut8, mir2.OpOut16, mir2.OpPatchSlot, mir2.OpLoadPatched, mir2.OpPatch, mir2.OpCallIndirect:
						return fail(fmt.Errorf("ASM/ports/SMC/indirect calls unsupported"))
					case mir2.OpCall:
						if module.FuncByName(i.Sym) == nil {
							return fail(fmt.Errorf("host call unsupported: %s", i.Sym))
						}
					}
				}
			}
		}
	}
	for _, f := range source.Funcs {
		for _, p := range f.Params {
			if p.SMC {
				return fail(fmt.Errorf("SMC parameter unsupported"))
			}
		}
	}
	program, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		return fail(err)
	}
	if len(program.Errors) != 0 || len(program.Binary) == 0 || program.Origin != workloadProgram || len(program.Binary) > workloadStack-workloadProgram {
		return fail(fmt.Errorf("assembly failed or program overlaps stack: %v", program.Errors))
	}
	out := &Z80ProfileWorkloadEvidence{Compatibility: profile.Compatibility, BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(program.Binary)), Globals: map[string][]byte{}, Resources: map[string][]byte{}, ResourceAddresses: map[string]int{}, GlobalAddresses: map[string]int{}}
	reachable, err := hir.ProfileWorkloadReachableFunctions(steps.MIR2Module, workload)
	if err != nil {
		return fail(err)
	}
	ranges := []memoryHostFunctionRange{}
	for _, f := range steps.MIR2Module.Funcs {
		addr, ok := workloadSymbol(program.Symbols, f.Name)
		if !ok {
			if reachable[f.Name] {
				return fail(fmt.Errorf("reachable function symbol missing: %s", f.Name))
			}
			continue
		}
		ranges = append(ranges, memoryHostFunctionRange{f.Name, addr})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	var readable, writable, executable, codeBytes [65536]bool
	mark := func(dst *[65536]bool, start, size int) {
		for a := start; a < start+size; a++ {
			dst[a] = true
		}
	}
	mark(&readable, workloadProgram, len(program.Binary))
	mark(&readable, workloadBootstrap, workloadBootstrapEnd-workloadBootstrap)
	mark(&readable, workloadStack, workloadStackEnd-workloadStack)
	mark(&writable, workloadStack, workloadStackEnd-workloadStack)
	// Instructions are executable, DB/DW/DS storage is not. In particular, inline
	// TSMC spill immediates remain code and cannot be patched by controlled calls.
	for _, line := range program.Listing {
		text := strings.TrimSpace(strings.SplitN(line.SourceLine, ";", 2)[0])
		if p := strings.Index(text, ":"); p >= 0 {
			text = strings.TrimSpace(text[p+1:])
		}
		fields := strings.Fields(text)
		if len(fields) == 0 || len(line.Bytes) == 0 {
			continue
		}
		op := strings.ToUpper(fields[0])
		if op == "DB" || op == "DW" || op == "DS" || op == "DEFB" || op == "DEFW" || op == "DEFS" || op == "ALIGN" {
			continue
		}
		if line.Address < workloadProgram || line.Address+len(line.Bytes) > workloadProgram+len(program.Binary) {
			return fail(fmt.Errorf("noncontiguous assembly layout"))
		}
		executable[line.Address] = memoryHostReachableInstruction(ranges, reachable, line.Address)
		mark(&codeBytes, line.Address, len(line.Bytes))
	}
	for name, addr := range program.Symbols {
		if strings.HasPrefix(strings.ToLower(name), "_tsmc_") {
			return fail(fmt.Errorf("compiler-generated SMC unsupported: %s", name))
		}
		if !strings.HasPrefix(strings.ToLower(name), "_spill_") {
			continue
		}
		found := false
		for _, line := range program.Listing {
			if line.Address == addr && len(line.Bytes) > 0 {
				if addr < workloadProgram || addr+len(line.Bytes) > workloadProgram+len(program.Binary) {
					return fail(fmt.Errorf("spill exceeds program bounds"))
				}
				text := strings.ToUpper(line.SourceLine)
				if strings.Contains(text, "DB ") || strings.Contains(text, "DW ") || strings.Contains(text, "DS ") {
					mark(&writable, addr, len(line.Bytes))
					found = true
					break
				}
			}
		}
		if !found {
			return fail(fmt.Errorf("missing bounds for spill %q", name))
		}
	}
	if len(expected.Globals) != len(source.Globals) {
		return fail(fmt.Errorf("oracle global set mismatch"))
	}
	for _, g := range source.Globals {
		size := max(mir2.ByteWidth(g.Ty), len(g.Init))
		cg := false
		for _, other := range steps.MIR2Module.Globals {
			if other.Name == g.Name && reflect.DeepEqual(other.Ty, g.Ty) && max(mir2.ByteWidth(other.Ty), len(other.Init)) == size {
				cg = true
				break
			}
		}
		addr, ok := workloadSymbol(program.Symbols, g.Name)
		want, present := expected.Globals[g.Name]
		if !cg || !ok || !present || len(want) != size || addr < workloadProgram || addr+size > workloadProgram+len(program.Binary) {
			return fail(fmt.Errorf("missing bounds/symbol/oracle for global %q", g.Name))
		}
		for a := addr; a < addr+size; a++ {
			if codeBytes[a] || writable[a] {
				return fail(fmt.Errorf("global overlaps code/storage %q", g.Name))
			}
		}
		out.GlobalAddresses[g.Name] = addr
		mark(&writable, addr, size)
	}
	names := make([]string, 0, len(workload.Resources))
	for name := range workload.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(expected.Resources) != len(names) {
		return fail(fmt.Errorf("oracle resource set mismatch"))
	}
	used := len(program.Binary) + (workloadBootstrapEnd - workloadBootstrap) + (workloadStackEnd - workloadStack)
	next := workloadBootstrapEnd
	for _, name := range names {
		resource := workload.Resources[name]
		want, ok := expected.Resources[name]
		if !ok || len(want) != len(resource) {
			return fail(fmt.Errorf("oracle resource bounds mismatch"))
		}
		if len(resource) > workloadProgram-next {
			return fail(fmt.Errorf("resource overlaps program"))
		}
		out.ResourceAddresses[name] = next
		mark(&readable, next, len(resource))
		mark(&writable, next, len(resource))
		next += len(resource)
		used += len(resource)
	}
	if used > workload.MaxMemory {
		return fail(fmt.Errorf("program/resources/bootstrap/stack exceed memory limit"))
	}
	// Require complete declared state at every boundary, even when there are no
	// globals/resources. Never substitute final memory for a missing snapshot.
	for ci, snapshot := range expected.Snapshots {
		if snapshot.Globals == nil || len(snapshot.Globals) != len(source.Globals) {
			return fail(fmt.Errorf("invocation %d: oracle snapshot global set mismatch", ci))
		}
		for _, g := range source.Globals {
			want, ok := snapshot.Globals[g.Name]
			if !ok || len(want) != len(expected.Globals[g.Name]) {
				return fail(fmt.Errorf("invocation %d: oracle snapshot global bounds mismatch: %s", ci, g.Name))
			}
		}
		if snapshot.Resources == nil || len(snapshot.Resources) != len(names) {
			return fail(fmt.Errorf("invocation %d: oracle snapshot resource set mismatch", ci))
		}
		for _, name := range names {
			want, ok := snapshot.Resources[name]
			if !ok || want == nil || len(want) != len(workload.Resources[name]) {
				return fail(fmt.Errorf("invocation %d: oracle snapshot resource bounds mismatch: %s", ci, name))
			}
		}
	}
	// Resolve actual production symbols without moving the shared program.
	var equ strings.Builder
	symbolNames := make([]string, 0, len(program.Symbols))
	for name := range program.Symbols {
		symbolNames = append(symbolNames, name)
	}
	sort.Strings(symbolNames)
	for _, name := range symbolNames {
		fmt.Fprintf(&equ, "%s EQU %d\n", name, program.Symbols[name])
	}
	boots := make([][]byte, len(workload.Calls))
	asserts := make([]hir.Assert, len(boots))
	for ci, call := range workload.Calls {
		hf := source.FuncByName(call.Function)
		mf := steps.MIR2Module.FuncByName(call.Function)
		if mf == nil || len(mf.Contract.Params) != len(hf.Params) {
			return fail(fmt.Errorf("compiled invocation ABI missing"))
		}
		if len(hf.RetTys) != 0 || (hf.RetTy != mir2.TyVoid && hf.RetTy != mir2.TyU8 && hf.RetTy != mir2.TyU16) {
			return fail(fmt.Errorf("pointer/unsupported return requires normalization"))
		}
		count := 0
		if hf.RetTy != mir2.TyVoid {
			count = 1
		}
		if len(expected.Results[ci]) != count || len(mf.Contract.Returns) != count {
			return fail(fmt.Errorf("oracle/compiled result shape mismatch"))
		}
		a := hir.Assert{FuncName: call.Function, Via: "z80"}
		for i, arg := range call.Arguments {
			ty := hf.Params[i].Ty
			compiledTy := mf.Contract.Params[i].Ty
			if ranged, ok := ty.(*mir2.RangedTy); ok {
				// LUTGen erases only the range refinement; physical scalar ABI
				// remains the declared base. The workload retains source bounds.
				if compiledTy != ranged.Base && !reflect.DeepEqual(compiledTy, ranged) {
					return fail(fmt.Errorf("compiled parameter type mismatch"))
				}
				ty = ranged.Base
			} else if compiledTy != ty {
				return fail(fmt.Errorf("compiled parameter type mismatch"))
			}
			loc := assertParamLocation(mf.Contract.Params[i], steps.Allocation)
			if loc.Name == "" && loc.Kind != mir2.LocMem {
				return fail(fmt.Errorf("missing parameter ABI location"))
			}
			if arg.Value != nil {
				limit := int64(255)
				if ty == mir2.TyU16 {
					limit = 65535
				} else if ty != mir2.TyU8 {
					return fail(fmt.Errorf("literal must be scalar u8/u16; numeric pointers unsupported"))
				}
				if *arg.Value < 0 || *arg.Value > limit {
					return fail(fmt.Errorf("literal outside scalar range"))
				}
				a.Args = append(a.Args, *arg.Value)
			} else {
				a.Args = append(a.Args, int64(out.ResourceAddresses[arg.Resource]+arg.Offset))
			}
		}
		if count == 1 {
			if mf.Contract.Returns[0].Ty != hf.RetTy {
				return fail(fmt.Errorf("compiled return type mismatch"))
			}
			a.Expected = expected.Results[ci][0].I
		}
		boot, err := z80asm.NewAssembler().AssembleString(equ.String() + buildAssertBootstrap(workloadBootstrap, a, mf, steps.Allocation))
		if err != nil {
			return fail(err)
		}
		if len(boot.Errors) != 0 || boot.Origin != workloadBootstrap || len(boot.Binary) > workloadBootstrapEnd-workloadBootstrap {
			return fail(fmt.Errorf("bootstrap assembly/bounds: %v", boot.Errors))
		}
		boots[ci] = boot.Binary
		asserts[ci] = a
		out.BootstrapSHA256 = append(out.BootstrapSHA256, fmt.Sprintf("%x", sha256.Sum256(boot.Binary)))
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	z := platform.RemogattoZ80
	if err := z.LoadMemory(workloadProgram, program.Binary); err != nil {
		return fail(err)
	}
	for _, g := range source.Globals {
		initial := make([]byte, max(mir2.ByteWidth(g.Ty), len(g.Init)))
		copy(initial, g.Init)
		if override, ok := workload.Globals[g.Name]; ok {
			initial = override
		}
		if err := z.LoadMemory(uint16(out.GlobalAddresses[g.Name]), initial); err != nil {
			return fail(err)
		}
	}
	for _, name := range names {
		if err := z.LoadMemory(uint16(out.ResourceAddresses[name]), workload.Resources[name]); err != nil {
			return fail(err)
		}
	}
	prof := emulator.NewProfiler()
	z.SetProfiler(prof)
	var violation error
	tracker := func(addr uint16, old, new byte) {
		if !writable[addr] {
			violation = fmt.Errorf("write outside bounded data/stack (SMC) at %04x", addr)
		}
	}
	step := func(boot bool, bootLen int) error {
		if out.ExecutedSteps >= workload.MaxSteps {
			return fmt.Errorf("cumulative instruction limit exceeded")
		}
		pc := int(z.GetPC())
		if !(boot && pc >= workloadBootstrap && pc < workloadBootstrap+bootLen) && !executable[pc] {
			return fmt.Errorf("execution outside instruction bounds at %04x", pc)
		}
		cycles := z.Step()
		out.ExecutedSteps++
		out.TotalTStates += int64(cycles)
		if cycles <= 0 {
			return fmt.Errorf("invalid emulator cycle count")
		}
		if violation != nil {
			return violation
		}
		if int(z.GetSP()) < workloadStack || int(z.GetSP()) > workloadStackEnd {
			return fmt.Errorf("stack bounds exceeded")
		}
		if z.IsHalted() {
			return fmt.Errorf("unexpected HALT")
		}
		return nil
	}
	for ci, call := range workload.Calls {
		// Only the private bootstrap is loaded on subsequent calls.
		z.SetSMCTracker(nil)
		z.SetProfiler(nil)
		if err := z.LoadMemory(workloadBootstrap, boots[ci]); err != nil {
			return fail(err)
		}
		z.SetSMCTracker(tracker)
		z.SetProfiler(prof)
		z.Unhalt()
		z.SetPC(workloadBootstrap)
		entry, ok := workloadSymbol(program.Symbols, call.Function)
		if !ok || !executable[entry] {
			return fail(fmt.Errorf("function entry symbol missing"))
		}
		for z.GetPC() != uint16(entry) {
			if err := step(true, len(boots[ci])); err != nil {
				return fail(err)
			}
		}
		sp := z.GetSP()
		ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
		if int(ret) < workloadBootstrap || int(ret) >= workloadBootstrap+len(boots[ci]) {
			return fail(fmt.Errorf("bootstrap return address invalid"))
		}
		var beforeReads, beforeWrites [65536]uint32
		if workload.ExactDataAccess {
			beforeReads = prof.ReadCount
			beforeWrites = prof.WriteCount
		}
		start := out.TotalTStates
		for {
			if err := step(false, 0); err != nil {
				return fail(err)
			}
			if z.GetPC() == ret && z.GetSP() == sp+2 {
				break
			}
		}
		mf := steps.MIR2Module.FuncByName(call.Function)
		results := []mir2.Value{}
		if len(expected.Results[ci]) > 0 {
			// Multi-result checking uses the actual return ABI location even for one
			// value, avoiding the legacy single-result A-only fallback.
			a := asserts[ci]
			a.ExpectedMulti = []int64{a.Expected}
			if err := checkAssertZ80Result(z, a, mf, map[string]*hir.Func{call.Function: source.FuncByName(call.Function)}); err != nil {
				return fail(fmt.Errorf("invocation %d: %w", ci, err))
			}
			results = append(results, mir2.Value{I: a.Expected})
			if !reflect.DeepEqual(results, expected.Results[ci]) {
				return fail(fmt.Errorf("noninteger oracle result unsupported"))
			}
		}
		snapshot := hir.ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}}
		for _, g := range source.Globals {
			addr := out.GlobalAddresses[g.Name]
			want := expected.Snapshots[ci].Globals[g.Name]
			got := append([]byte{}, z.Memory()[addr:addr+len(want)]...)
			if !bytes.Equal(got, want) {
				return fail(fmt.Errorf("invocation %d @%s: global mismatch: %s got %v want %v", ci, call.Function, g.Name, got, want))
			}
			snapshot.Globals[g.Name] = got
		}
		for _, name := range names {
			addr := out.ResourceAddresses[name]
			want := expected.Snapshots[ci].Resources[name]
			got := append([]byte{}, z.Memory()[addr:addr+len(want)]...)
			if !bytes.Equal(got, want) {
				return fail(fmt.Errorf("invocation %d @%s: resource mismatch: %s got %v want %v", ci, call.Function, name, got, want))
			}
			snapshot.Resources[name] = got
		}
		var access *hir.ProfileAccessSnapshot
		if workload.ExactDataAccess {
			observed := hir.ProfileAccessSnapshot{Globals: map[string]hir.ProfileByteAccess{}, Resources: map[string]hir.ProfileByteAccess{}}
			measure := func(address, size int) hir.ProfileByteAccess {
				a := hir.ProfileByteAccess{Reads: make([]uint64, size), Writes: make([]uint64, size)}
				for i := 0; i < size; i++ {
					a.Reads[i] = uint64(prof.ReadCount[address+i] - beforeReads[address+i])
					a.Writes[i] = uint64(prof.WriteCount[address+i] - beforeWrites[address+i])
				}
				return a
			}
			for _, g := range source.Globals {
				observed.Globals[g.Name] = measure(out.GlobalAddresses[g.Name], max(mir2.ByteWidth(g.Ty), len(g.Init)))
			}
			for _, name := range names {
				observed.Resources[name] = measure(out.ResourceAddresses[name], len(workload.Resources[name]))
			}
			if !jointEqual(observed, expected.Accesses[ci]) {
				return fail(fmt.Errorf("invocation %d @%s: exact data access mismatch", ci, call.Function))
			}
			access = &observed
		}
		out.Invocations = append(out.Invocations, Z80ProfileInvocationEvidence{Access: access, Function: call.Function, TStates: out.TotalTStates - start, Results: results, Snapshot: snapshot})
		for addr := 0; addr < 65536; addr++ {
			if prof.ReadCount[addr] > 0 && !readable[addr] {
				return fail(fmt.Errorf("read outside declared bounds at %04x", addr))
			}
			if prof.WriteCount[addr] > 0 && !writable[addr] {
				return fail(fmt.Errorf("write outside declared bounds/SMC at %04x", addr))
			}
		}
		if len(prof.IORead) != 0 || len(prof.IOWrite) != 0 {
			return fail(fmt.Errorf("port I/O unsupported"))
		}
	}
	for _, g := range source.Globals {
		addr := out.GlobalAddresses[g.Name]
		size := len(expected.Globals[g.Name])
		out.Globals[g.Name] = append([]byte{}, z.Memory()[addr:addr+size]...)
		if !bytes.Equal(out.Globals[g.Name], expected.Globals[g.Name]) {
			return fail(fmt.Errorf("final global mismatch: %s got %v want %v", g.Name, out.Globals[g.Name], expected.Globals[g.Name]))
		}
	}
	for _, name := range names {
		addr := out.ResourceAddresses[name]
		size := len(workload.Resources[name])
		out.Resources[name] = append([]byte{}, z.Memory()[addr:addr+size]...)
		if !bytes.Equal(out.Resources[name], expected.Resources[name]) {
			return fail(fmt.Errorf("final resource mismatch: %s", name))
		}
	}
	// Bootstrap instructions consume the execution budget but are not part of
	// caller entry-through-RET costs used to select production variants.
	allCycles := out.TotalTStates
	out.TotalTStates = 0
	for _, invocation := range out.Invocations {
		out.TotalTStates += invocation.TStates
	}
	out.BootstrapTStates = allCycles - out.TotalTStates
	return out, nil
}

func workloadPointerStorage(ty mir2.Ty) bool {
	if ty == nil || mir2.IsPointer(ty) {
		return true
	}
	switch t := ty.(type) {
	case *mir2.ArrayTy:
		return workloadPointerStorage(t.Elem)
	case *mir2.StructTy:
		for _, f := range t.Fields {
			if workloadPointerStorage(f.Ty) {
				return true
			}
		}
	}
	return false
}

func workloadSymbol(symbols map[string]int, name string) (int, bool) {
	var b strings.Builder
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	for _, candidate := range []string{name, strings.ToUpper(name), s, strings.ToUpper(s), "v_" + s, strings.ToUpper("v_" + s)} {
		if addr, ok := symbols[candidate]; ok {
			return addr, true
		}
	}
	return 0, false
}
