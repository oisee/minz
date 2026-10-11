package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"reflect"
	"sort"
	"strings"
)

type MemoryHostTargetEvidence struct {
	Context           hir.ProfileContext            `json:"context"`
	Binary            []byte                        `json:"binary"`
	BinarySHA256      string                        `json:"binary_sha256"`
	Assembly          string                        `json:"assembly"`
	AdapterSHA256     string                        `json:"adapter_sha256"`
	GlobalAddresses   map[string]int                `json:"global_addresses"`
	Invocations       []hir.MemoryHostObservation   `json:"invocations"`
	InvocationTStates []int64                       `json:"invocation_t_states"`
	TotalTStates      int64                         `json:"total_t_states"`
	ExecutedSteps     int64                         `json:"executed_steps"`
	Accesses          []MemoryHostResolvedAccess    `json:"accesses"`
	SidecarSHA256     string                        `json:"sidecar_sha256"`
	MIRSHA256         string                        `json:"mir_dump_sha256"`
	AllocationSHA256  string                        `json:"allocation_sha256"`
	FinalMIRDump      string                        `json:"final_mir_dump_diagnostic"`
	CallFrames        []MemoryHostResolvedCallFrame `json:"call_frames"`
	CallFramesSHA256  string                        `json:"call_frames_sha256"`
	CallSpills        []MemoryHostResolvedCallSpill `json:"call_spills"`
	CallSpillsSHA256  string                        `json:"call_spills_sha256"`
	AllocationJSON    json.RawMessage               `json:"allocation_diagnostic"`
}

// runZ80MemoryHostWorkload consumes only privately compiled steps and an internally
// produced raw oracle. It is not a public assembly or oracle admission path.
func runZ80MemoryHostWorkload(steps Steps, source *hir.Module, w hir.MemoryHostWorkload, oracle hir.MemoryHostRun) (*MemoryHostTargetEvidence, error) {
	fail := func(e error) (*MemoryHostTargetEvidence, error) { return nil, fmt.Errorf("memory host target: %w", e) }
	owned, ctx, e := hir.PrepareMemoryHostWorkload(source, w)
	if e != nil {
		return fail(e)
	}
	w = owned
	compat, e := hir.BuildProfileCompatibility(source, ctx)
	if e != nil {
		return fail(e)
	}
	b, e := json.Marshal(oracle)
	if e != nil {
		return fail(e)
	}
	var expected hir.MemoryHostRun
	if e = json.Unmarshal(b, &expected); e != nil {
		return fail(e)
	}
	if !reflect.DeepEqual(compat, expected.Compatibility) || !reflect.DeepEqual(ctx, expected.Context) || len(expected.Invocations) == 0 || len(expected.Invocations) > len(w.Calls) || steps.MIR2Module == nil || steps.Allocation == nil {
		return fail(fmt.Errorf("context/oracle/compiled module mismatch"))
	}
	if e = hir.ValidateMemoryHostMIR(source, steps.MIR2Module, w); e != nil {
		return fail(e)
	}
	origins := hir.MemoryHostGlobalOrigins(steps.MIR2Module)
	records := []mir2.NativeMemoryAccess{}
	frames := []mir2.NativeCallFrameSnapshot{}
	callSpills := []mir2.NativeCallSpillSnapshot{}
	captured, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: origins, NativeMemoryAccesses: &records, NativeCallFrames: &frames, NativeCallSpills: &callSpills, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if e != nil {
		return fail(e)
	}
	captured = emitExternStubs(captured, steps.MIR2Module)
	// Regenerated optional annotations may alter text, but never baseline bytes.
	plain, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x%04x\n", w.Layout.CodeOrigin) + steps.Assembly)
	if e != nil || len(plain.Errors) != 0 {
		return fail(fmt.Errorf("private baseline assembly failed"))
	}
	capturedProgram, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x%04x\n", w.Layout.CodeOrigin) + captured)
	if e != nil || len(capturedProgram.Errors) != 0 || !reflect.DeepEqual(plain.Binary, capturedProgram.Binary) {
		return fail(fmt.Errorf("native annotation changed private baseline binary"))
	}
	assembly := captured
	adapters := map[string]string{}
	hosts := make([]string, 0, len(w.Hosts))
	for n := range w.Hosts {
		hosts = append(hosts, n)
	}
	sort.Strings(hosts)
	for _, n := range hosts {
		f := steps.MIR2Module.FuncByName(n)
		if f == nil {
			return fail(fmt.Errorf("missing host ABI"))
		}
		kind := w.Hosts[n]
		count := 1
		returns := 1
		body := "    LD A,(HL)\n    RET\n"
		if kind == hir.MemoryHostWriteU8 {
			count = 2
			returns = 0
			body = "    LD (HL),C\n    RET\n"
		}
		if len(f.Contract.Params) != count || len(f.Contract.Returns) != returns || assertParamLocation(f.Contract.Params[0], steps.Allocation).Name != "HL" {
			return fail(fmt.Errorf("unsupported host address ABI %s", n))
		}
		if count == 2 && assertParamLocation(f.Contract.Params[1], steps.Allocation).Name != "C" || returns == 1 && f.Contract.Returns[0].Class != mir2.ClassAcc {
			return fail(fmt.Errorf("unsupported host value ABI %s", n))
		}
		marker := n + ":\n"
		if strings.Count(assembly, marker) != 1 {
			return fail(fmt.Errorf("host label missing/ambiguous"))
		}
		assembly = strings.Replace(assembly, marker, marker+body, 1)
		adapters[n] = body
	}
	a, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x%04x\n", w.Layout.CodeOrigin) + assembly)
	if e != nil {
		return fail(e)
	}
	if len(a.Errors) != 0 || len(a.Binary) == 0 || int(a.Origin)+len(a.Binary) > 65536 {
		return fail(fmt.Errorf("assembly/layout invalid: %v", a.Errors))
	}
	reachable, e := hir.MemoryHostReachableFunctions(steps.MIR2Module, w)
	if e != nil {
		return fail(e)
	}
	ranges := []memoryHostFunctionRange{}
	for _, f := range steps.MIR2Module.Funcs {
		p, ok := workloadSymbol(a.Symbols, f.Name)
		if ok {
			ranges = append(ranges, memoryHostFunctionRange{f.Name, p})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	allowedInstruction := func(p int) bool {
		return memoryHostReachableInstruction(ranges, reachable, p)
	}
	m := &memoryHostZ80Memory{lengths: map[uint16]int{}}
	copy(m.data[int(a.Origin):], a.Binary)
	// Exact assembled instruction extents distinguish code from DB/DW/DS storage.
	for _, l := range a.Listing {
		t := strings.TrimSpace(strings.SplitN(l.SourceLine, ";", 2)[0])
		if i := strings.Index(t, ":"); i >= 0 {
			t = strings.TrimSpace(t[i+1:])
		}
		upper := strings.ToUpper(t)
		if len(l.Bytes) == 0 || t == "" || strings.HasPrefix(upper, "DB ") || strings.HasPrefix(upper, "DW ") || strings.HasPrefix(upper, "DS ") || strings.HasPrefix(upper, "DEFB ") || strings.HasPrefix(upper, "DEFW ") {
			continue
		}
		if l.Address < 0 || l.Address+len(l.Bytes) > 65536 {
			return fail(fmt.Errorf("instruction bounds"))
		}
		if !allowedInstruction(l.Address) {
			continue
		}
		m.lengths[uint16(l.Address)] = len(l.Bytes)
		for p := l.Address; p < l.Address+len(l.Bytes); p++ {
			m.executable[p] = true
		}
	}
	used := [65536]bool{}
	mark := func(start, n int) error {
		if start < 0 || n <= 0 || start+n > 65536 {
			return fmt.Errorf("layout bounds")
		}
		for p := start; p < start+n; p++ {
			if used[p] {
				return fmt.Errorf("layout overlap %04x", p)
			}
		}
		for p := start; p < start+n; p++ {
			used[p] = true
		}
		return nil
	}
	if e = mark(int(a.Origin), len(a.Binary)); e != nil {
		return fail(e)
	}
	if e = mark(int(w.Layout.StackLow), int(w.Layout.StackHigh)-int(w.Layout.StackLow)); e != nil {
		return fail(e)
	}
	for p := int(w.Layout.StackLow); p < int(w.Layout.StackHigh); p++ {
		m.readable[p] = true
		m.writable[p] = true
		m.stack[p] = true
	}
	out := &MemoryHostTargetEvidence{Context: ctx, Binary: a.Binary, Assembly: assembly, BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(a.Binary)), GlobalAddresses: map[string]int{}, Invocations: []hir.MemoryHostObservation{}, InvocationTStates: []int64{}}
	adapterBytes, _ := json.Marshal(adapters)
	out.AdapterSHA256 = fmt.Sprintf("%x", sha256.Sum256(adapterBytes))
	for n, r := range w.Regions {
		if e = mark(int(r.Address), len(r.Bytes)); e != nil {
			return fail(e)
		}
		copy(m.data[int(r.Address):], r.Bytes)
		for p := int(r.Address); p < int(r.Address)+len(r.Bytes); p++ {
			m.region[p] = n
			m.readable[p] = true
			m.writable[p] = r.Writable
		}
	}
	for _, g := range source.Globals {
		address, ok := workloadSymbol(a.Symbols, g.Name)
		n := max(mir2.ByteWidth(g.Ty), len(g.Init))
		if !ok || address < 0 || address+n > 65536 {
			return fail(fmt.Errorf("global storage missing"))
		}
		for p := address; p < address+n; p++ {
			if m.executable[p] || m.region[p] != "" || m.stack[p] {
				return fail(fmt.Errorf("global overlaps protected layout"))
			}
			m.global[p] = true
			m.readable[p] = true
			m.writable[p] = true
		}
		out.GlobalAddresses[g.Name] = address
		copy(m.data[address:address+n], g.Init)
		if initial, ok := w.Globals[g.Name]; ok {
			copy(m.data[address:address+n], initial)
		}
	}
	// Compiler-owned spill declaration size comes from exact assembled DB bytes.
	for _, line := range a.Listing {
		label := line.Label
		if strings.HasPrefix(label, "_spill_") || strings.HasPrefix(label, "_SPILL_") {
			for address := line.Address; address < line.Address+len(line.Bytes); address++ {
				if address >= 0 && address < 65536 && !m.executable[address] && m.region[address] == "" && !m.stack[address] {
					m.readable[address] = true
					m.writable[address] = true
					m.spill[address] = true
				}
			}
		}
	}
	protected, resolved, e := resolveMemoryHostSidecar(a, records)
	if e != nil {
		return fail(e)
	}
	snapshotSteps, resolvedFrames, e := resolveMemoryHostCallFrames(a, assembly, frames)
	if e != nil {
		return fail(e)
	}
	callSpillSteps, resolvedCallSpills, e := resolveMemoryHostCallSpills(a, assembly, callSpills, steps.MIR2Module, steps.Allocation)
	if e != nil {
		return fail(e)
	}
	for pc := range callSpillSteps {
		if _, overlaps := snapshotSteps[pc]; overlaps {
			return fail(fmt.Errorf("native snapshot recipe overlap"))
		}
	}
	m.callSpills = callSpillSteps
	out.CallSpills = resolvedCallSpills
	callSpillPayload, e := json.Marshal(resolvedCallSpills)
	if e != nil {
		return fail(e)
	}
	out.CallSpillsSHA256 = fmt.Sprintf("%x", sha256.Sum256(callSpillPayload))
	m.snapshots = snapshotSteps
	out.CallFrames = resolvedFrames
	framePayload, e := json.Marshal(resolvedFrames)
	if e != nil {
		return fail(e)
	}
	out.CallFramesSHA256 = fmt.Sprintf("%x", sha256.Sum256(framePayload))
	m.protected = protected
	out.Accesses = resolved
	sidecar, _ := json.Marshal(resolved)
	out.SidecarSHA256 = fmt.Sprintf("%x", sha256.Sum256(sidecar))
	out.FinalMIRDump = steps.MIR2Module.Dump()
	out.MIRSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(out.FinalMIRDump)))
	allocation, e := json.Marshal(steps.Allocation)
	if e != nil {
		return fail(e)
	}
	out.AllocationJSON = append(json.RawMessage(nil), allocation...)
	out.AllocationSHA256 = fmt.Sprintf("%x", sha256.Sum256(allocation))
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	for i, call := range w.Calls {
		if m.activeCallSpill != nil || len(m.savedCallSpills) != 0 {
			return fail(fmt.Errorf("pending native call-spill snapshot before invocation"))
		}
		m.active = false
		for _, p := range call.Patches {
			r := w.Regions[p.Region]
			start := int(r.Address)
			if p.Clear {
				clear(m.data[start : start+len(r.Bytes)])
			}
			copy(m.data[start+p.Offset:], p.Bytes)
		}
		for n, v := range call.Globals {
			copy(m.data[out.GlobalAddresses[n]:], v)
		}
		f := steps.MIR2Module.FuncByName(call.Function)
		entry, ok := workloadSymbol(a.Symbols, call.Function)
		if !ok || !m.executable[entry] || len(f.Contract.Params) != len(call.Arguments) {
			return fail(fmt.Errorf("invocation ABI missing"))
		}
		m.cpu.SetPC(uint16(entry))
		m.cpu.SetSP(w.Layout.StackHigh - 2)
		m.callFrames = []memoryHostCallFrame{{w.Layout.StackHigh - 2, w.Layout.ReturnPC}}
		m.savedCallSpills = nil
		m.activeCallSpill = nil
		clear(m.stackInitialized[:])
		m.stackInitialized[w.Layout.StackHigh-2] = true
		m.stackInitialized[w.Layout.StackHigh-1] = true
		m.data[w.Layout.StackHigh-2] = byte(w.Layout.ReturnPC)
		m.data[w.Layout.StackHigh-1] = byte(w.Layout.ReturnPC >> 8)
		for j, v := range call.Arguments {
			loc := assertParamLocation(f.Contract.Params[j], steps.Allocation)
			if e = memoryHostSetArgument(m, loc, v); e != nil {
				return fail(e)
			}
		}
		m.events = []memoryHostNativeAccess{}
		m.active = true
		start := m.cpu.Tstates
		stepsBefore := out.ExecutedSteps
		var runErr error
		for m.cpu.PC() != w.Layout.ReturnPC {
			if out.ExecutedSteps >= w.MaxSteps {
				return fail(fmt.Errorf("native instruction limit exceeded"))
			}
			if m.cpu.SP() < w.Layout.StackLow || m.cpu.SP() > w.Layout.StackHigh {
				return fail(fmt.Errorf("native stack bounds exceeded"))
			}
			for n, kind := range w.Hosts {
				p, _ := workloadSymbol(a.Symbols, n)
				if m.cpu.PC() == uint16(p) {
					_, runErr = hir.MemoryHostRegionAt(w, m.cpu.HL(), kind == hir.MemoryHostWriteU8)
					break
				}
			}
			if runErr != nil {
				break
			}
			_, runErr = memoryHostNativeStep(m)
			var cpuFailure memoryHostCPUFailure
			if errors.As(runErr, &cpuFailure) {
				return fail(runErr)
			}
			out.ExecutedSteps++
			if runErr != nil {
				return fail(runErr)
			}
			if m.cpu.Halted {
				return fail(fmt.Errorf("unexpected native halt"))
			}
		}
		m.active = false
		obs := hir.MemoryHostObservation{Results: []mir2.Value{}, Globals: map[string][]byte{}, Regions: map[string][]byte{}, Events: []hir.MemoryHostAccess{}, Steps: out.ExecutedSteps - stepsBefore}
		for _, event := range m.events {
			obs.Events = append(obs.Events, hir.MemoryHostAccess{Kind: event.Kind, Address: event.Address, Value: event.Value})
		}
		for _, g := range source.Globals {
			p := out.GlobalAddresses[g.Name]
			n := max(mir2.ByteWidth(g.Ty), len(g.Init))
			obs.Globals[g.Name] = append([]byte{}, m.data[p:p+n]...)
		}
		for n, r := range w.Regions {
			obs.Regions[n] = append([]byte{}, m.data[int(r.Address):int(r.Address)+len(r.Bytes)]...)
		}
		if runErr != nil {
			var denied *hir.MemoryHostAccessError
			if !errors.As(runErr, &denied) || !reflect.DeepEqual(denied, expected.Invocations[i].Failure) {
				return fail(fmt.Errorf("memory denial cause mismatch"))
			}
			copied := *denied
			obs.Failure = &copied
			obs.Error = runErr.Error()
		} else if m.activeCallSpill != nil || len(m.savedCallSpills) != 0 {
			return fail(fmt.Errorf("unretired native call-spill snapshot"))
		} else if m.cpu.SP() != w.Layout.StackHigh {
			return fail(fmt.Errorf("return stack mismatch"))
		} else {
			hf := source.FuncByName(call.Function)
			if hf.RetTy != mir2.TyVoid {
				if len(f.Contract.Returns) != 1 {
					return fail(fmt.Errorf("return ABI missing"))
				}
				v := int64(m.cpu.HL())
				if f.Contract.Returns[0].Class == mir2.ClassAcc {
					v = int64(m.cpu.A)
				}
				obs.Results = append(obs.Results, mir2.Value{I: v})
			}
		}
		want := expected.Invocations[i]
		if !reflect.DeepEqual(obs.Results, want.Results) || !reflect.DeepEqual(obs.Globals, want.Globals) || !reflect.DeepEqual(obs.Regions, want.Regions) || !reflect.DeepEqual(obs.Events, want.Events) || (obs.Error == "") != (want.Error == "") {
			return fail(fmt.Errorf("oracle mismatch invocation %d: results=%t globals=%t regions=%t events=%t error=%q wantError=%q", i, reflect.DeepEqual(obs.Results, want.Results), reflect.DeepEqual(obs.Globals, want.Globals), reflect.DeepEqual(obs.Regions, want.Regions), reflect.DeepEqual(obs.Events, want.Events), obs.Error, want.Error))
		}
		out.Invocations = append(out.Invocations, obs)
		t := int64(m.cpu.Tstates - start)
		out.InvocationTStates = append(out.InvocationTStates, t)
		out.TotalTStates += t
		if runErr != nil {
			break
		}
	}
	return out, nil
}
func memoryHostSetArgument(m *memoryHostZ80Memory, loc mir2.PhysLoc, v int64) error {
	switch loc.Name {
	case "A":
		m.cpu.A = byte(v)
	case "B":
		m.cpu.B = byte(v)
	case "C":
		m.cpu.C = byte(v)
	case "D":
		m.cpu.D = byte(v)
	case "E":
		m.cpu.E = byte(v)
	case "H":
		m.cpu.H = byte(v)
	case "L":
		m.cpu.L = byte(v)
	case "HL":
		m.cpu.H = byte(v >> 8)
		m.cpu.L = byte(v)
	case "DE":
		m.cpu.D = byte(v >> 8)
		m.cpu.E = byte(v)
	case "BC":
		m.cpu.B = byte(v >> 8)
		m.cpu.C = byte(v)
	default:
		return fmt.Errorf("unsupported invocation location %s", loc.Name)
	}
	return nil
}

// Equal entry addresses are legitimate assembler aliases (e.g. tail-call EQU).
// Only the latest entry group owns this PC; reachability does not bleed into
// a following distinct unreachable function. Instruction lengths still come
// exclusively from assembled instruction lines, never this range alone.
type memoryHostFunctionRange struct {
	name  string
	start int
}

func memoryHostReachableInstruction(ranges []memoryHostFunctionRange, reachable map[string]bool, pc int) bool {
	start := -1
	allowed := false
	for _, f := range ranges {
		if f.start > pc {
			break
		}
		if f.start != start {
			start = f.start
			allowed = false
		}
		allowed = allowed || reachable[f.name]
	}
	return allowed
}
