package hir

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

const MemoryHostReadU8 = "read-u8-at-u16"
const MemoryHostWriteU8 = "write-u8-at-u16"
const MemoryHostSemantics = "minz.memory-host/bounded-byte-effects/v1"

type MemoryHostRegion struct {
	Address  uint16 `json:"address"`
	Bytes    []byte `json:"bytes"`
	Writable bool   `json:"writable"`
}
type MemoryHostPatch struct {
	Region string `json:"region"`
	Offset int    `json:"offset"`
	Bytes  []byte `json:"bytes"`
	Clear  bool   `json:"clear"`
}
type MemoryHostInvocation struct {
	Function  string            `json:"function"`
	Arguments []int64           `json:"arguments"`
	Globals   map[string][]byte `json:"globals"`
	Patches   []MemoryHostPatch `json:"patches"`
}
type MemoryHostLayout struct {
	CodeOrigin uint16 `json:"code_origin"`
	StackLow   uint16 `json:"stack_low"`
	StackHigh  uint16 `json:"stack_high"`
	ReturnPC   uint16 `json:"return_pc"`
}
type MemoryHostWorkload struct {
	Schema      string                      `json:"schema"`
	TargetModel string                      `json:"target_model"`
	MaxSteps    int64                       `json:"max_steps"`
	MaxMemory   int                         `json:"max_memory"`
	Layout      MemoryHostLayout            `json:"layout"`
	Hosts       map[string]string           `json:"hosts"`
	Regions     map[string]MemoryHostRegion `json:"regions"`
	Globals     map[string][]byte           `json:"globals"`
	Calls       []MemoryHostInvocation      `json:"calls"`
}
type MemoryHostAccess struct {
	Kind    string `json:"kind"`
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}

// MemoryHostAccessError identifies an actual denied semantic byte access.
// Other runtime/infrastructure failures must not be replayed as memory denials.
type MemoryHostAccessError struct {
	Kind    string `json:"kind"`
	Address uint16 `json:"address"`
	Write   bool   `json:"write"`
}

func (e *MemoryHostAccessError) Error() string {
	op := "read"
	if e.Write {
		op = "write"
	}
	return fmt.Sprintf("memory host: %s %s at %d", e.Kind, op, e.Address)
}

type MemoryHostObservation struct {
	Failure *MemoryHostAccessError `json:"failure,omitempty"`
	Results []mir2.Value           `json:"results"`
	Globals map[string][]byte      `json:"globals"`
	Regions map[string][]byte      `json:"regions"`
	Events  []MemoryHostAccess     `json:"events"`
	Error   string                 `json:"error"`
	Steps   int64                  `json:"steps"`
}
type MemoryHostRun struct {
	Compatibility *ProfileCompatibility   `json:"compatibility"`
	Context       ProfileContext          `json:"context"`
	Invocations   []MemoryHostObservation `json:"invocations"`
}

// MemoryHostRegionAt returns the unique declared byte region, never code/global/stack storage.
func MemoryHostRegionAt(w MemoryHostWorkload, addr uint16, write bool) (string, error) {
	for name, r := range w.Regions {
		if int(addr) >= int(r.Address) && int(addr) < int(r.Address)+len(r.Bytes) {
			if write && !r.Writable {
				return "", &MemoryHostAccessError{Kind: "read-only", Address: addr, Write: true}
			}
			return name, nil
		}
	}
	return "", &MemoryHostAccessError{Kind: "undeclared", Address: addr, Write: write}
}

func memoryHostGlobalSizes(m *Module) map[string]int {
	out := map[string]int{}
	for _, g := range m.Globals {
		n := mir2.ByteWidth(g.Ty)
		if len(g.Init) > n {
			n = len(g.Init)
		}
		out[g.Name] = n
	}
	return out
}

// PrepareMemoryHostWorkload owns the exact bytes hashed and validates the closed builtin model.
func PrepareMemoryHostWorkload(m *Module, w MemoryHostWorkload) (MemoryHostWorkload, ProfileContext, error) {
	var owned MemoryHostWorkload
	b, e := json.Marshal(w)
	if e != nil {
		return owned, ProfileContext{}, e
	}
	if e = json.Unmarshal(b, &owned); e != nil {
		return owned, ProfileContext{}, e
	}
	fail := func(s string) (MemoryHostWorkload, ProfileContext, error) {
		return owned, ProfileContext{}, fmt.Errorf("memory host: %s", s)
	}
	if m == nil || owned.Schema != "minz.memory-host-workload/v1" || owned.TargetModel != "cpm-z80-v1" || owned.MaxSteps <= 0 || owned.MaxSteps > 100000000 || owned.MaxMemory != 65536 || len(owned.Hosts) == 0 || len(owned.Regions) == 0 || len(owned.Calls) == 0 {
		return fail("invalid schema/model/limits/hosts/regions/calls")
	}
	if m.OrderedByteRWPolicy != nil || m.OrderedBytePolicy != nil || m.RegionComparePolicy != nil || m.ForwardCopyPolicy != nil || m.PointerWalkPolicy != nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
		return fail("requires unmodified source")
	}
	if e = Verify(m); e != nil {
		return fail(e.Error())
	}
	l := owned.Layout
	if l.CodeOrigin == 0 || l.StackLow <= l.CodeOrigin || l.StackHigh <= l.StackLow || l.StackHigh >= 65535 || l.ReturnPC >= l.CodeOrigin {
		return fail("invalid code/stack/return layout")
	}
	names := []string{}
	for name := range owned.Regions {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		r := owned.Regions[name]
		lo, hi := int(r.Address), int(r.Address)+len(r.Bytes)
		if name == "" || len(r.Bytes) == 0 || hi > 65536 || lo <= int(l.ReturnPC) && hi > int(l.ReturnPC) || lo <= int(l.StackHigh)+1 && hi > int(l.StackLow) {
			return fail("invalid region or stack/return overlap")
		}
		for _, other := range names[:i] {
			q := owned.Regions[other]
			if lo < int(q.Address)+len(q.Bytes) && int(q.Address) < hi {
				return fail("overlapping regions")
			}
		}
	}
	seen := map[string]bool{}
	for _, f := range m.Funcs {
		kind, ok := owned.Hosts[f.Name]
		if _, collision := memoryHostGlobalSizes(m)[f.Name]; collision {
			return fail("function/global name collision")
		}
		if !f.IsExtern {
			if ok {
				return fail("host names non-extern function")
			}
			continue
		}
		if !ok || f.Body != nil || len(f.Captures) != 0 || len(f.RetTys) != 0 || f.ExternAddr != 0 || f.ExternClobbers != nil {
			return fail("unregistered or unsupported extern")
		}
		if kind != MemoryHostReadU8 && kind != MemoryHostWriteU8 {
			return fail("unknown builtin host kind")
		}
		n := 1
		var ret mir2.Ty = mir2.TyU8
		if kind == MemoryHostWriteU8 {
			n = 2
			ret = mir2.TyVoid
		}
		if len(f.Params) != n || f.RetTy != ret || f.Params[0].Ty != mir2.TyU16 {
			return fail("host signature mismatch")
		}
		if n == 2 && f.Params[1].Ty != mir2.TyU8 {
			return fail("host value type mismatch")
		}
		for _, p := range f.Params {
			if p.SMC || p.RegClass != 0 {
				return fail("host custom parameter ABI unsupported")
			}
		}
		seen[f.Name] = true
	}
	if len(seen) != len(owned.Hosts) {
		return fail("surplus host declaration")
	}
	sizes := memoryHostGlobalSizes(m)
	checkGlobals := func(gs map[string][]byte) error {
		for name, b := range gs {
			n, ok := sizes[name]
			if !ok || len(b) != n {
				return fmt.Errorf("global override %q width mismatch", name)
			}
		}
		return nil
	}
	if e = checkGlobals(owned.Globals); e != nil {
		return fail(e.Error())
	}
	for _, c := range owned.Calls {
		f := m.FuncByName(c.Function)
		if f == nil || f.IsExtern || len(f.Captures) != 0 || len(c.Arguments) != len(f.Params) {
			return fail("invalid invocation")
		}
		for i, p := range f.Params {
			if p.SMC || !memoryHostInteger(p.Ty) {
				return fail("nonliteral integer invocation unsupported")
			}
			if !memoryHostLiteralFits(p.Ty, c.Arguments[i]) {
				return fail("argument outside declared integer width")
			}
		}
		if len(f.RetTys) != 0 || f.RetTy != mir2.TyVoid && !memoryHostInteger(f.RetTy) {
			return fail("unsupported invocation return type")
		}
		if e = checkGlobals(c.Globals); e != nil {
			return fail(e.Error())
		}
		for _, p := range c.Patches {
			r, ok := owned.Regions[p.Region]
			if !ok || p.Offset < 0 || p.Offset > len(r.Bytes) || len(p.Bytes) > len(r.Bytes)-p.Offset || p.Clear && p.Offset != 0 {
				return fail("invalid region patch")
			}
		}
	}
	raw := LowerModule(m)
	if e = ValidateMemoryHostMIR(m, raw, owned); e != nil {
		return fail(e.Error())
	}
	descriptor, _ := json.Marshal(struct {
		Semantics string
		Hosts     map[string]string
		Layout    MemoryHostLayout
	}{MemoryHostSemantics, owned.Hosts, owned.Layout})
	resources := map[string][]byte{}
	for name, r := range owned.Regions {
		resources["memory-region:"+name] = r.Bytes
	}
	ctx, e := BuildAutomaticProfileContext(m, ProfileInputs{Workload: b, Hosts: descriptor, Resources: resources, TargetModel: owned.TargetModel})
	if e != nil {
		return fail(e.Error())
	}
	ctx.SemanticsID += "/" + MemoryHostSemantics
	return owned, ctx, nil
}

// ValidateMemoryHostMIR rejects undeclared effects and checks actual host signatures.
func ValidateMemoryHostMIR(source *Module, m *mir2.Module, w MemoryHostWorkload) error {
	if m == nil || source == nil {
		return fmt.Errorf("memory host: missing source/MIR")
	}
	if m.Target != source.Target {
		return fmt.Errorf("memory host: compiled target differs from source")
	}
	names := map[string]bool{}
	for _, f := range m.Funcs {
		if f == nil || f.Name == "" || names[f.Name] {
			return fmt.Errorf("memory host: invalid/duplicate MIR function")
		}
		names[f.Name] = true
	}
	globalNames := map[string]bool{}
	for _, g := range m.Globals {
		if g.Name == "" || globalNames[g.Name] || names[g.Name] {
			return fmt.Errorf("memory host: invalid/duplicate MIR global")
		}
		globalNames[g.Name] = true
	}
	if !reflect.DeepEqual(source.Globals, m.Globals) {
		return fmt.Errorf("memory host: compiled global metadata differs from source")
	}
	reachable, err := MemoryHostReachableFunctions(m, w)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, g := range m.Globals {
		if g.Storage != mir2.StorageNormal || g.At != nil || len(g.VtableSyms) != 0 || memoryHostPointerStorage(g.Ty) {
			return fmt.Errorf("memory host: unsupported global storage %q", g.Name)
		}
	}
	for _, f := range m.Funcs {
		if f.Attrs.HasAsm {
			return fmt.Errorf("memory host: ASM function unsupported")
		}
		if f.Attrs.IsExtern {
			kind, ok := w.Hosts[f.Name]
			sf := source.FuncByName(f.Name)
			if !ok || sf == nil || !sf.IsExtern || len(f.Blocks) != 0 || len(f.Contract.Params) != len(sf.Params) {
				return fmt.Errorf("memory host: compiled extern mismatch %q", f.Name)
			}
			for i, p := range f.Contract.Params {
				if p.Ty != sf.Params[i].Ty {
					return fmt.Errorf("memory host: compiled parameter mismatch")
				}
			}
			if kind != MemoryHostReadU8 && kind != MemoryHostWriteU8 {
				return fmt.Errorf("memory host: unknown host")
			}
			nr := 0
			if sf.RetTy != mir2.TyVoid {
				nr = 1
			}
			if len(f.Contract.Returns) != nr || nr == 1 && f.Contract.Returns[0].Ty != sf.RetTy || f.Attrs.ExternAddr != 0 || f.Contract.ExternClobbers != nil {
				return fmt.Errorf("memory host: compiled return/ABI mismatch")
			}
			seen[f.Name] = true
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case mir2.OpAsm, mir2.OpIn8, mir2.OpIn16, mir2.OpOut8, mir2.OpOut16, mir2.OpPatchSlot, mir2.OpLoadPatched, mir2.OpPatch, mir2.OpCallIndirect:
					return fmt.Errorf("memory host: ASM/ports/SMC/indirect unsupported")
				case mir2.OpCall:
					if m.FuncByName(i.Sym) == nil && (reachable[f.Name] || !strings.HasPrefix(strings.TrimPrefix(i.Sym, "@"), "mir.io.print.")) {
						return fmt.Errorf("memory host: undeclared call %q", i.Sym)
					}
				}
			}
		}
	}
	if len(seen) != len(w.Hosts) {
		return fmt.Errorf("memory host: compiled host set mismatch")
	}
	return nil
}

func RunMemoryHostBaseline(source *Module, w MemoryHostWorkload) (*MemoryHostRun, error) {
	if source == nil {
		return nil, fmt.Errorf("memory host: missing source")
	}
	if _, _, err := PrepareMemoryHostWorkload(source, w); err != nil {
		return nil, err
	}
	return RunMemoryHostMIRBaseline(source, LowerModule(source), w)
}

// RunMemoryHostMIRBaseline is controlled baseline evidence, not a rewrite certificate.
func RunMemoryHostMIRBaseline(source *Module, m *mir2.Module, w MemoryHostWorkload) (*MemoryHostRun, error) {
	owned, ctx, e := PrepareMemoryHostWorkload(source, w)
	if e != nil {
		return nil, e
	}
	if e = ValidateMemoryHostMIR(source, m, owned); e != nil {
		return nil, e
	}
	if e = mir2.Verify(m); e != nil {
		return nil, e
	}
	vm := mir2.NewVM(m)
	used := vm.AllocHeap(nil).I
	for _, r := range owned.Regions {
		if int64(r.Address) < used {
			return nil, fmt.Errorf("memory host: region overlaps VM module storage")
		}
	}
	vm.EnsureHeap(65536)
	vm.MaxSteps = owned.MaxSteps
	vm.MaxMemory = owned.MaxMemory
	vm.Hosts = map[string]mir2.HostFunc{}
	events := []MemoryHostAccess{}
	checkRegion := func(op mir2.Op, addr int64) error {
		if addr < 0 || addr > 65535 {
			return fmt.Errorf("memory host: invalid address")
		}
		_, err := MemoryHostRegionAt(owned, uint16(addr), op == mir2.OpStore)
		return err
	}
	for name, kind := range owned.Hosts {
		vm.Hosts["@"+name] = func(args []mir2.Value) ([]mir2.Value, error) {
			n := 1
			if kind == MemoryHostWriteU8 {
				n = 2
			}
			if len(args) != n {
				return nil, fmt.Errorf("memory host: argument mismatch")
			}
			addr := int64(uint16(args[0].I))
			op := mir2.OpLoad
			if kind == MemoryHostWriteU8 {
				op = mir2.OpStore
			}
			if err := checkRegion(op, addr); err != nil {
				return nil, err
			}
			if op == mir2.OpLoad {
				v := vm.ReadHeap(addr, 1)[0]
				events = append(events, MemoryHostAccess{"read", uint16(addr), v})
				return []mir2.Value{{I: int64(v)}}, nil
			}
			v := byte(args[1].I)
			vm.WriteHeap(addr, v)
			events = append(events, MemoryHostAccess{"write", uint16(addr), v})
			return nil, nil
		}
	}
	authorized := MemoryHostGlobalAccesses(m)
	lastGlobal := false
	vm.CheckDataAccess = func(inst *mir2.Inst, addr int64) error {
		lastGlobal = false
		if name, ok := authorized[inst]; ok {
			start := vm.GlobalAddr(name)
			size := memoryHostMIRGlobalSize(m, name)
			if addr < start || addr >= start+int64(size) {
				return fmt.Errorf("memory host: global access outside proved storage")
			}
			lastGlobal = true
			return nil
		}
		return checkRegion(inst.Op, addr)
	}
	vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
		if lastGlobal {
			return
		}
		kind := "read"
		if a.Op == mir2.OpStore {
			kind = "write"
		}
		events = append(events, MemoryHostAccess{kind, uint16(a.Addr), a.Value})
	}
	for _, r := range owned.Regions {
		vm.WriteHeapBytes(int64(r.Address), r.Bytes)
	}
	put := func(gs map[string][]byte) {
		for name, b := range gs {
			vm.WriteHeapBytes(vm.GlobalAddr(name), b)
		}
	}
	put(owned.Globals)
	compat, e := BuildProfileCompatibility(source, ctx)
	if e != nil {
		return nil, e
	}
	out := &MemoryHostRun{Compatibility: compat, Context: ctx, Invocations: []MemoryHostObservation{}}
	snapshot := func() MemoryHostObservation {
		o := MemoryHostObservation{Results: []mir2.Value{}, Globals: map[string][]byte{}, Regions: map[string][]byte{}, Events: append([]MemoryHostAccess{}, events...)}
		for _, g := range m.Globals {
			n := mir2.ByteWidth(g.Ty)
			if len(g.Init) > n {
				n = len(g.Init)
			}
			o.Globals[g.Name] = vm.ReadHeap(vm.GlobalAddr(g.Name), n)
		}
		for name, r := range owned.Regions {
			o.Regions[name] = vm.ReadHeap(int64(r.Address), len(r.Bytes))
		}
		return o
	}
	for j, c := range owned.Calls {
		put(c.Globals)
		for _, p := range c.Patches {
			r := owned.Regions[p.Region]
			if p.Clear {
				vm.WriteHeapBytes(int64(r.Address), make([]byte, len(r.Bytes)))
			}
			vm.WriteHeapBytes(int64(r.Address)+int64(p.Offset), p.Bytes)
		}
		events = []MemoryHostAccess{}
		args := make([]mir2.Value, len(c.Arguments))
		for i, v := range c.Arguments {
			args[i].I = v
		}
		before := vm.StepsExecuted()
		result, err := vm.Call(c.Function, args)
		o := snapshot()
		o.Steps = vm.StepsExecuted() - before
		if err != nil {
			o.Error = err.Error()
			var denied *MemoryHostAccessError
			if errors.As(err, &denied) {
				copyFailure := *denied
				o.Failure = &copyFailure
			}
			out.Invocations = append(out.Invocations, o)
			return out, fmt.Errorf("memory host: invocation %d: %w", j, err)
		}
		o.Results = append([]mir2.Value{}, result...)
		out.Invocations = append(out.Invocations, o)
	}
	return out, nil
}

func memoryHostInteger(t mir2.Ty) bool {
	if r, ok := t.(*mir2.RangedTy); ok {
		t = r.Base
	}
	return t == mir2.TyU8 || t == mir2.TyU16 || t == mir2.TyI8 || t == mir2.TyI16 || t == mir2.TyBool
}
func memoryHostLiteralFits(t mir2.Ty, v int64) bool {
	if r, ok := t.(*mir2.RangedTy); ok {
		if v < r.Lo || v >= r.Hi {
			return false
		}
		t = r.Base
	}
	switch t {
	case mir2.TyU8:
		return v >= 0 && v <= 255
	case mir2.TyU16:
		return v >= 0 && v <= 65535
	case mir2.TyI8:
		return v >= -128 && v <= 127
	case mir2.TyI16:
		return v >= -32768 && v <= 32767
	case mir2.TyBool:
		return v == 0 || v == 1
	}
	return false
}

// MemoryHostReachableFunctions closes the direct-call graph from exact workload entries.
// Missing compiler print intrinsics are tolerated only outside this graph.
func MemoryHostReachableFunctions(m *mir2.Module, w MemoryHostWorkload) (map[string]bool, error) {
	out := map[string]bool{}
	queue := []string{}
	for _, c := range w.Calls {
		queue = append(queue, c.Function)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if out[name] {
			continue
		}
		f := m.FuncByName(name)
		if f == nil {
			return nil, fmt.Errorf("memory host: reachable undefined function %q", name)
		}
		out[name] = true
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpCall {
					queue = append(queue, i.Sym)
				}
			}
		}
	}
	return out, nil
}

// MemoryHostGlobalAccesses proves only a global symbol plus bounded constant
// offset, without phi/block parameters or unknown integer pointer provenance.
// Map keys are actual instruction identity, not numerical runtime addresses.
func MemoryHostGlobalAccesses(m *mir2.Module) map[*mir2.Inst]string {
	out := map[*mir2.Inst]string{}
	for inst, root := range MemoryHostGlobalOrigins(m) {
		out[inst] = root.Symbol
	}
	return out
}

// MemoryHostGlobalOrigins uses reaching symbolic values at each actual
// instruction. Reused allocated registers are killed by their later definition;
// differing incoming values and block parameters never acquire global authority.
func MemoryHostGlobalOrigins(m *mir2.Module) map[*mir2.Inst]mir2.GlobalMemoryOrigin {
	out := map[*mir2.Inst]mir2.GlobalMemoryOrigin{}
	if m == nil {
		return out
	}
	type value struct {
		symbol   string
		offset   int64
		constant bool
		number   int64
	}
	type state map[mir2.Reg]value
	clone := func(s state) state {
		r := state{}
		for k, v := range s {
			r[k] = v
		}
		return r
	}
	same := func(a, b state) bool { return reflect.DeepEqual(a, b) }
	successors := func(t mir2.Term) []string {
		if t == nil {
			return nil
		}
		return t.Successors()
	}
	for _, f := range m.Funcs {
		pred := map[string][]string{}
		for _, b := range f.Blocks {
			for _, next := range successors(b.Term) {
				pred[next] = append(pred[next], b.Label)
			}
		}
		incoming, outgoing := map[string]state{}, map[string]state{}
		for _, b := range f.Blocks {
			incoming[b.Label] = state{}
			outgoing[b.Label] = state{}
		}
		transfer := func(b *mir2.Block, in state, record bool) state {
			current := clone(in)
			for _, p := range b.Params {
				delete(current, p.Dst)
			}
			for ii, i := range b.Insts {
				if record && (i.Op == mir2.OpLoad || i.Op == mir2.OpStore) {
					p, ok := current[i.Src[0]]
					w := mir2.ByteWidth(i.Ty)
					if ok && p.symbol != "" && p.offset >= 0 && w > 0 && w <= 2 && p.offset <= int64(memoryHostMIRGlobalSize(m, p.symbol)-w) {
						out[i] = mir2.GlobalMemoryOrigin{Symbol: p.symbol, Offset: int(p.offset), Width: w, Write: i.Op == mir2.OpStore, Function: f.Name, Block: b.Label, Instruction: ii}
					}
				}
				if i.Dst == mir2.NoReg {
					continue
				}
				a, aok := current[i.Src[0]]
				c, cok := current[i.Src[1]]
				delete(current, i.Dst)
				switch i.Op {
				case mir2.OpAddrOf:
					if memoryHostMIRGlobalSize(m, i.Sym) > 0 {
						current[i.Dst] = value{symbol: i.Sym}
					}
				case mir2.OpConst:
					current[i.Dst] = value{constant: true, number: i.Imm}
				case mir2.OpMove:
					if aok {
						current[i.Dst] = a
					}
				case mir2.OpPtrBump:
					if aok && a.symbol != "" && i.Imm >= 0 && i.Imm <= 65535 && a.offset <= 65535-i.Imm {
						a.offset += i.Imm
						current[i.Dst] = a
					}
				case mir2.OpPtrAdd:
					if aok && a.symbol != "" && cok && c.constant && c.number >= 0 && c.number <= 65535 && a.offset <= 65535-c.number {
						a.offset += c.number
						current[i.Dst] = a
					}
				}
			}
			return current
		}
		stable := false
		for iteration := 0; iteration < len(f.Blocks)*4+4; iteration++ {
			changed := false
			for bi, b := range f.Blocks {
				in := state{}
				ps := pred[b.Label]
				if bi != 0 && len(ps) > 0 {
					in = clone(outgoing[ps[0]])
					for _, p := range ps[1:] {
						for r, v := range in {
							if other, ok := outgoing[p][r]; !ok || other != v {
								delete(in, r)
							}
						}
					}
				}
				next := transfer(b, in, false)
				if !same(in, incoming[b.Label]) || !same(next, outgoing[b.Label]) {
					changed = true
					incoming[b.Label] = in
					outgoing[b.Label] = next
				}
			}
			if !changed {
				stable = true
				break
			}
		}
		if stable {
			for _, b := range f.Blocks {
				transfer(b, incoming[b.Label], true)
			}
		}
	}
	return out
}

func memoryHostMIRGlobalSize(m *mir2.Module, name string) int {
	for _, g := range m.Globals {
		if g.Name == name {
			n := mir2.ByteWidth(g.Ty)
			if len(g.Init) > n {
				n = len(g.Init)
			}
			return n
		}
	}
	return 0
}

func memoryHostPointerStorage(t mir2.Ty) bool {
	if mir2.IsPointer(t) {
		return true
	}
	switch x := t.(type) {
	case *mir2.ArrayTy:
		return memoryHostPointerStorage(x.Elem)
	case *mir2.StructTy:
		for _, f := range x.Fields {
			if memoryHostPointerStorage(f.Ty) {
				return true
			}
		}
	case *mir2.TupleTy:
		for _, e := range x.Elems {
			if memoryHostPointerStorage(e) {
				return true
			}
		}
	case *mir2.SliceTy:
		return true
	}
	return false
}
