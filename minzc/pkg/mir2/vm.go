package mir2

import (
	"fmt"
	"math"
	"os"
)

// VM is a simple interpreter for MIR2 functions.
//
// Primary use: compile-time execution of [comptime] functions (@-metafunctions).
// Secondary use: unit testing IR without a real backend.
//
// Execution model:
//   - Infinite virtual registers per frame (map[Reg]Value)
//   - Block arguments passed as a []Value slice — no phi node tracking needed
//   - Call stack of frames (no shared mutable state between frames)
//   - Host function table for @mir.* intrinsics and @host.* compiler callbacks
//   - Gas counter: aborts runaway compile-time loops
//
// Limitations (intentional for a compile-time VM):
//   - No OpAsm execution (returns zero value + warning)
//   - No OpPatchSlot physical encoding (treated as a mutable slot variable)
//   - Pointer arithmetic uses a flat []byte heap; no OS-level memory
//   - No concurrency
//
// PortIO is the interface for I/O port access, shared across all runtimes
// (MZV VM, MZE Z80 emulator, MZX ZX Spectrum). One harness, all platforms.
type PortIO interface {
	ReadPort(address uint16) byte
	WritePort(address uint16, b byte)
}

type VM struct {
	initErr error
	Module  *Module

	// Host function table.
	// Key: full intrinsic name (e.g. "@mir.io.print.u8", "@host.emit.code")
	// Value: Go function that receives args and returns results.
	Hosts map[string]HostFunc

	// I/O port handler — same interface as emulator.Ports.
	// When set, OpIn8/OpOut8/OpIn16/OpOut16 dispatch here.
	// Console ($23), network ($30), control ($31) — all through one interface.
	Ports PortIO

	// ObserveDataAccess optionally receives each byte actually read by OpLoad or
	// written by OpStore, in execution order (low byte first). Nil disables
	// observation. The callback cannot fail and should not mutate or reenter VM.
	// This is raw MIR2 data traffic, not a universal target memory trace: heap
	// setup/snapshots, host functions (including memcpy/memset), OpPatchSlot,
	// OpPatch, OpLoadPatched, ASM, ports, and instruction fetches are excluded.
	ObserveDataAccess func(VMDataAccess)

	// CheckDataAccess runs before each actual OpLoad/OpStore byte access.
	// A non-nil error stops the instruction before that byte is accessed; earlier
	// successful bytes and their observations remain committed. Nil preserves
	// the default VM behavior. The hook must not mutate or reenter the VM.
	CheckDataAccess func(inst *Inst, addr int64) error

	// Execution limits (protect against infinite loops in comptime code).
	MaxSteps  int64 // 0 = unlimited (not recommended for comptime)
	MaxMemory int   // heap byte limit; 0 = 64 KB default

	// Heap: flat byte array for ptr-typed values.
	heap []byte

	// Internal step counter.
	steps int64

	// globalSyms maps module global names to their heap offsets.
	// Populated by NewVM from Module.Globals.
	globalSyms map[string]int64
	namedSMC   map[string]Value // persistent named immediate values, separate from generic slot addresses
}

// VMDataAccess is one executed OpLoad/OpStore byte access. Addr is the actual
// VM heap offset after pointer-width wrapping, not a relocated Z80 address.
// Value is the byte read or written. Repeated/overlapping accesses produce
// separate events; a rejected heap-range instruction produces none. A per-byte
// CheckDataAccess failure retains observations for earlier successful bytes.
type VMDataAccess struct {
	Op    Op // OpLoad (read) or OpStore (write)
	Addr  int64
	Value byte
}

// StepsExecuted returns the cumulative instruction steps charged by
// this VM, including steps across successive calls. This is VM work, not target
// CPU cycles. Terminators are not charged by the current VM; a step rejected by
// the gas limit is also included.
func (vm *VM) StepsExecuted() int64 { return vm.steps }

// GlobalAddr returns the heap offset for a named global, or -1 if not found.
func (vm *VM) GlobalAddr(name string) int64 {
	if off, ok := vm.globalSyms[name]; ok {
		return off
	}
	return -1
}

// HostFunc is the signature of a host-provided function.
// args correspond to the call's argument list; the returned []Value must
// match the callee's return type count.
type HostFunc func(args []Value) ([]Value, error)

// ── Value ─────────────────────────────────────────────────────────────────────

// Value is a runtime value in the VM.
// All integer types (u8/u16/bool/i8/i16/…) are stored as int64 with width
// enforced by the instruction type.  Pointers are offsets into VM.heap.
type Value struct {
	I int64 // integer / bool / pointer-as-heap-offset
}

var zeroValue = Value{}

// truncate masks v to the bit width of ty.
func truncate(v Value, ty Ty) Value {
	if ty == nil || ty == TyVoid {
		return zeroValue
	}
	bits := ty.Width()
	if bits <= 0 || bits >= 64 {
		return v
	}
	mask := int64((1 << bits) - 1)
	return Value{I: v.I & mask}
}

// signExtend sign-extends from srcBits to 64 bits.
func signExtend(v Value, srcBits int) Value {
	if srcBits <= 0 || srcBits >= 64 {
		return v
	}
	shift := 64 - srcBits
	return Value{I: (v.I << shift) >> shift}
}

// ── Frame ─────────────────────────────────────────────────────────────────────

// frame is one activation record on the call stack.
type frame struct {
	fn            *Func
	vals          map[Reg]Value // virtual register file for this call
	pointerWidths map[Reg]int   // only non-default pointer widths
}

func newFrame(fn *Func) *frame {
	fr := &frame{fn: fn, vals: make(map[Reg]Value, 32)}
	record := func(r Reg, ty Ty) {
		if p, ok := ty.(*ptrTy); ok && p.width != 16 {
			if fr.pointerWidths == nil {
				fr.pointerWidths = make(map[Reg]int)
			}
			fr.pointerWidths[r] = p.width
		}
	}
	for _, p := range fn.Contract.Params {
		record(p.Reg, p.Ty)
	}
	for _, b := range fn.Blocks {
		for _, p := range b.Params {
			record(p.Dst, p.Ty)
		}
		for _, i := range b.Insts {
			record(i.Dst, i.Ty)
			if i.Op == OpCall && fn.mod != nil {
				if callee := fn.mod.FuncByName(i.Sym); callee != nil {
					for n, r := range i.ExtraRets {
						if n+1 < len(callee.Contract.Returns) {
							record(r, callee.Contract.Returns[n+1].Ty)
						}
					}
				}
			}
		}
	}
	return fr
}

func (fr *frame) get(r Reg) Value {
	if r == NoReg {
		return zeroValue
	}
	return fr.vals[r]
}

func (fr *frame) set(r Reg, v Value) {
	if r != NoReg {
		fr.vals[r] = v
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

// initNamedSMC registers owned immediates lazily, before a first read or patch.
func (vm *VM) initNamedSMC() {
	if vm.namedSMC != nil {
		return
	}
	vm.namedSMC = make(map[string]Value)
	for _, f := range vm.Module.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpPatchSlot && i.Sym != "" {
					vm.namedSMC[i.Sym] = truncate(Value{I: i.Imm}, i.Ty)
				}
			}
		}
	}
}

// NewVM creates a VM for the given module with sensible defaults.
func NewVM(m *Module) *VM {
	vm := &VM{
		Module:     m,
		Hosts:      make(map[string]HostFunc),
		MaxSteps:   10_000_000,
		MaxMemory:  65536,
		globalSyms: make(map[string]int64),
	}
	vm.heap = make([]byte, 0, 4096)
	if m.NonNullObjects {
		// C object addresses must differ from NULL. Explicit address-zero
		// accesses remain available; raw-address MIR keeps its default layout.
		vm.heap = append(vm.heap, 0)
	}
	if err := VerifyGlobalRelocations(m); err != nil {
		vm.initErr = err
		return vm
	}
	// Pre-allocate all module globals into the heap.
	for _, g := range m.Globals {
		offset := int64(len(vm.heap))
		vm.globalSyms[g.Name] = offset
		w := ByteWidth(g.Ty)
		data := make([]byte, w)
		copy(data, g.Init)
		vm.heap = append(vm.heap, data...)
	}
	// Register function names as symbols so OpAddrOf("funcName") can resolve them.
	// Each function gets a unique heap address (store its name as a tag).
	for _, fn := range m.Funcs {
		if _, exists := vm.globalSyms[fn.Name]; !exists {
			offset := int64(len(vm.heap))
			vm.globalSyms[fn.Name] = offset
			// Write a small tag so the address is unique and non-zero.
			tag := []byte(fn.Name)
			vm.heap = append(vm.heap, tag...)
			vm.heap = append(vm.heap, 0) // NUL terminator
		}
	}
	// Resolve vtable globals: patch function addresses into vtable slots.
	for _, g := range m.Globals {
		if len(g.VtableSyms) == 0 {
			continue
		}
		base, ok := vm.globalSyms[g.Name]
		if !ok {
			continue
		}
		for i, sym := range g.VtableSyms {
			if sym == "" {
				continue
			}
			addr, ok := vm.globalSyms[sym]
			if !ok {
				continue
			}
			off := base + int64(i*2)
			if off+2 <= int64(len(vm.heap)) {
				vm.heap[off] = byte(addr)
				vm.heap[off+1] = byte(addr >> 8)
			}
		}
	}
	for _, g := range m.Globals {
		for _, r := range g.Relocs {
			addr, err := vm.resolveSymbol(r.Symbol)
			if err != nil {
				vm.initErr = err
				return vm
			}
			off := vm.globalSyms[g.Name] + int64(r.Offset)
			value := uint16(addr + r.Addend)
			vm.heap[off], vm.heap[off+1] = byte(value), byte(value>>8)
		}
	}
	vm.registerBuiltinHosts()
	return vm
}

// CallOutcome is a language result; Go error remains a VM trap, not CY failure.
type CallOutcome struct {
	Values    []Value
	Failed    bool
	ErrorCode byte
}

func (vm *VM) CallWithOutcome(name string, args []Value) (CallOutcome, error) {
	fn := vm.Module.FuncByName(name)
	if fn == nil {
		return CallOutcome{}, fmt.Errorf("mir2.VM: function @%s not found", name)
	}
	bound, err := vm.BindStructReturnArguments(fn, args)
	if err != nil {
		return CallOutcome{}, err
	}
	return vm.callFuncOutcome(fn, bound)
}

// Call executes a named function with the given argument values.
// Returns ordinary values; failure returns the u8 code for legacy callers.
// Use CallWithOutcome to distinguish a language error from a success value.
func (vm *VM) Call(funcName string, args []Value) ([]Value, error) {
	fn := vm.Module.FuncByName(funcName)
	if fn == nil {
		return nil, fmt.Errorf("mir2.VM: function @%s not found", funcName)
	}
	bound, err := vm.BindStructReturnArguments(fn, args)
	if err != nil {
		return nil, err
	}
	return vm.callFunc(fn, bound)
}

// CallFunc executes a Func directly.
func (vm *VM) CallFunc(fn *Func, args []Value) ([]Value, error) {
	bound, err := vm.BindStructReturnArguments(fn, args)
	if err != nil {
		return nil, err
	}
	return vm.callFunc(fn, bound)
}

// AllocHeap appends data to the VM heap and returns a pointer value (heap offset).
// Used in tests to set up input arrays for pointer-taking functions.
func (vm *VM) AllocHeap(data []byte) Value {
	offset := int64(len(vm.heap))
	vm.heap = append(vm.heap, data...)
	return Value{I: offset}
}

// EnsureHeap grows the heap if needed to hold at least minSize bytes.
func (vm *VM) EnsureHeap(minSize int64) {
	if int(minSize) > len(vm.heap) {
		vm.heap = append(vm.heap, make([]byte, int(minSize)-len(vm.heap))...)
	}
}

// WriteHeap writes a single byte to the VM heap at addr.
func (vm *VM) WriteHeap(addr int64, b byte) {
	if addr >= 0 && int(addr) < len(vm.heap) {
		vm.heap[int(addr)] = b
	}
}

// WriteHeapSlice writes a byte slice to the VM heap starting at addr.
func (vm *VM) WriteHeapSlice(addr int64, data []byte) {
	for i, b := range data {
		off := int(addr) + i
		if off >= 0 && off < len(vm.heap) {
			vm.heap[off] = b
		}
	}
}

// ReadHeap reads n bytes from the VM heap at pointer address addr.
func (vm *VM) ReadHeap(addr int64, n int) []byte {
	end := int(addr) + n
	if int(addr) < 0 || end > len(vm.heap) {
		return nil
	}
	out := make([]byte, n)
	copy(out, vm.heap[addr:end])
	return out
}

// WriteHeapBytes writes data to the VM heap at pointer address addr.
// The destination must already be allocated (within heap bounds).
func (vm *VM) WriteHeapBytes(addr int64, data []byte) {
	end := int(addr) + len(data)
	if int(addr) < 0 || end > len(vm.heap) {
		return
	}
	copy(vm.heap[addr:end], data)
}

// ── Execution engine ──────────────────────────────────────────────────────────

func (vm *VM) callFunc(fn *Func, args []Value) ([]Value, error) {
	out, err := vm.callFuncOutcome(fn, args)
	if out.Failed {
		return []Value{{I: int64(out.ErrorCode)}}, err
	}
	return out.Values, err
}
func (vm *VM) callFuncOutcome(fn *Func, args []Value) (CallOutcome, error) {
	if vm.initErr != nil {
		return CallOutcome{}, vm.initErr
	}
	if fn.Attrs.IsExtern {
		// Try host table by function name.
		if hf, ok := vm.Hosts["@"+fn.Name]; ok {
			values, err := hf(args)
			return CallOutcome{Values: values}, err
		}
		return CallOutcome{}, fmt.Errorf("mir2.VM: extern @%s has no host implementation", fn.Name)
	}

	entry := fn.Entry()
	if entry == nil {
		return CallOutcome{}, fmt.Errorf("mir2.VM: @%s has no entry block", fn.Name)
	}

	fr := newFrame(fn)

	// Bind function parameters.
	params := fn.Contract.Params
	if len(args) != len(params) {
		return CallOutcome{}, fmt.Errorf("mir2.VM: @%s expects %d args, got %d",
			fn.Name, len(params), len(args))
	}
	for i, p := range params {
		fr.set(p.Reg, truncate(args[i], p.Ty))
	}

	// ── Iterative block execution loop ──
	// Instead of recursive execBlock→execTerm→execBlock calls (which overflow
	// the Go stack on deep loops), we iterate: execute block instructions,
	// then resolve the terminator to get the next block + args.
	blk := entry
	var blockArgs []Value // nil for entry block

	for {
		// Bind block arguments to block params.
		if len(blockArgs) != len(blk.Params) {
			return CallOutcome{}, fmt.Errorf("mir2.VM: block @%s expects %d params, got %d args",
				blk.Label, len(blk.Params), len(blockArgs))
		}
		for i, p := range blk.Params {
			fr.set(p.Dst, truncate(blockArgs[i], p.Ty))
		}

		// Execute instructions.
		for _, inst := range blk.Insts {
			if err := vm.step(); err != nil {
				return CallOutcome{}, err
			}
			if err := vm.execInst(fr, inst); err != nil {
				return CallOutcome{}, fmt.Errorf("block @%s: %w", blk.Label, err)
			}
		}

		if term, ok := blk.Term.(*TermErrorRet); ok {
			return CallOutcome{Failed: true, ErrorCode: byte(fr.get(term.Code).I)}, nil
		}
		// Resolve terminator → either return or jump to next block.
		rets, next, nextArgs, err := vm.execTerm(fr, blk)
		if err != nil {
			return CallOutcome{}, err
		}
		if next == nil {
			// TermRet or TermCondRet(return path) — done.
			return CallOutcome{Values: rets}, nil
		}
		// Jump to next block — iterate, don't recurse.
		blk = next
		blockArgs = nextArgs
	}
}

// execTerm resolves a terminator.
// Returns (retVals, nextBlock, nextBlockArgs, error).
// If nextBlock is nil, the function returns retVals.
func (vm *VM) execTerm(fr *frame, blk *Block) ([]Value, *Block, []Value, error) {
	switch t := blk.Term.(type) {

	case *TermRet:
		rets := make([]Value, len(t.Vals))
		for i, r := range t.Vals {
			rets[i] = fr.get(r)
		}
		return rets, nil, nil, nil

	case *TermJmp:
		args := collectArgs(fr, t.Args)
		next := fr.fn.BlockByLabel(t.Target)
		if next == nil {
			return nil, nil, nil, fmt.Errorf("jmp to unknown block @%s", t.Target)
		}
		return nil, next, args, nil

	case *TermBrIf:
		cond := fr.get(t.Cond)
		if cond.I != 0 {
			args := collectArgs(fr, t.ThenArgs)
			next := fr.fn.BlockByLabel(t.Then)
			if next == nil {
				return nil, nil, nil, fmt.Errorf("br_if then: unknown block @%s", t.Then)
			}
			return nil, next, args, nil
		}
		args := collectArgs(fr, t.ElseArgs)
		next := fr.fn.BlockByLabel(t.Else)
		if next == nil {
			return nil, nil, nil, fmt.Errorf("br_if else: unknown block @%s", t.Else)
		}
		return nil, next, args, nil

	case *TermBrIf2:
		lhs := fr.get(t.Lhs).I
		rhs := fr.get(t.Rhs).I
		var target string
		var tArgs []Reg
		switch {
		case lhs == rhs:
			target, tArgs = t.Eq, t.EqArgs
		case uint64(lhs) < uint64(rhs):
			target, tArgs = t.Lt, t.LtArgs
		default:
			target, tArgs = t.Gt, t.GtArgs
		}
		next := fr.fn.BlockByLabel(target)
		if next == nil {
			return nil, nil, nil, fmt.Errorf("br_if2: unknown block @%s", target)
		}
		return nil, next, collectArgs(fr, tArgs), nil

	case *TermDJNZ:
		cval := fr.get(t.Counter).I
		cval--
		if cval != 0 {
			bodyBlock := fr.fn.BlockByLabel(t.Body)
			if bodyBlock == nil {
				return nil, nil, nil, fmt.Errorf("djnz body: unknown block @%s", t.Body)
			}
			bodyArgVals := make([]Value, 1+len(t.BodyArgs))
			bodyArgVals[0] = Value{I: cval}
			for i, r := range t.BodyArgs {
				bodyArgVals[i+1] = fr.get(r)
			}
			return nil, bodyBlock, bodyArgVals, nil
		}
		exitBlock := fr.fn.BlockByLabel(t.Exit)
		if exitBlock == nil {
			return nil, nil, nil, fmt.Errorf("djnz exit: unknown block @%s", t.Exit)
		}
		return nil, exitBlock, collectArgs(fr, t.ExitArgs), nil

	case *TermCondRet:
		cond := fr.get(t.Cond)
		if cond.I == 0 {
			rets := make([]Value, len(t.Vals))
			for i, r := range t.Vals {
				rets[i] = fr.get(r)
			}
			return rets, nil, nil, nil
		}
		args := collectArgs(fr, t.ThenArgs)
		next := fr.fn.BlockByLabel(t.Then)
		if next == nil {
			return nil, nil, nil, fmt.Errorf("cond_ret then: unknown block @%s", t.Then)
		}
		return nil, next, args, nil

	case *TermUnreachable:
		return nil, nil, nil, fmt.Errorf("mir2.VM: reached unreachable in @%s block @%s",
			fr.fn.Name, blk.Label)
	}

	return nil, nil, nil, fmt.Errorf("mir2.VM: unknown terminator type in @%s", fr.fn.Name)
}

// execInst executes a single instruction, updating fr.vals.
func (vm *VM) execInst(fr *frame, inst *Inst) error {
	a := fr.get(inst.Src[0])
	b := fr.get(inst.Src[1])
	ty := inst.Ty

	var result Value

	switch inst.Op {

	// ── Arithmetic ──────────────────────────────────────────────────────────
	case OpAdd:
		result = truncate(Value{I: a.I + b.I}, ty)
	case OpSub:
		result = truncate(Value{I: a.I - b.I}, ty)
	case OpMul:
		result = truncate(Value{I: a.I * b.I}, ty)
	case OpDiv:
		if b.I == 0 {
			return fmt.Errorf("division by zero")
		}
		result = truncate(Value{I: int64(uint64(a.I) / uint64(b.I))}, ty)
	case OpSDiv:
		lhs := signExtend(a, ty.Width()).I
		rhs := signExtend(b, ty.Width()).I
		if rhs == 0 {
			return fmt.Errorf("signed division by zero")
		}
		result = truncate(Value{I: lhs / rhs}, ty)
	case OpSMod:
		lhs := signExtend(a, ty.Width()).I
		rhs := signExtend(b, ty.Width()).I
		if rhs == 0 {
			return fmt.Errorf("signed remainder by zero")
		}
		result = truncate(Value{I: lhs % rhs}, ty)
	case OpMod:
		if b.I == 0 {
			return fmt.Errorf("mod by zero")
		}
		result = truncate(Value{I: int64(uint64(a.I) % uint64(b.I))}, ty)

	// ── Bitwise ─────────────────────────────────────────────────────────────
	case OpAnd:
		result = Value{I: a.I & b.I}
	case OpOr:
		result = Value{I: a.I | b.I}
	case OpXor:
		result = Value{I: a.I ^ b.I}
	case OpShl:
		shift := uint(b.I)
		result = truncate(Value{I: a.I << shift}, ty)
	case OpShr:
		shift := uint(b.I)
		result = truncate(Value{I: int64(uint64(a.I) >> shift)}, ty)
	case OpSar:
		shift := uint(b.I)
		result = truncate(Value{I: signExtend(a, ty.Width()).I >> shift}, ty)

	// ── Unary ───────────────────────────────────────────────────────────────
	case OpNeg:
		result = truncate(Value{I: -a.I}, ty)
	case OpNot:
		result = truncate(Value{I: ^a.I}, ty)
	case OpBitGet:
		baseTy := inst.SrcTy
		if baseTy == nil {
			baseTy = ty
		}
		v := truncate(a, baseTy)
		result = Value{I: (v.I >> uint(inst.Imm)) & 1}
	case OpBitSet:
		result = truncate(Value{I: a.I | (int64(1) << uint(inst.Imm))}, ty)
	case OpBitReset:
		result = truncate(Value{I: a.I & ^(int64(1) << uint(inst.Imm))}, ty)

	// ── Conversions ─────────────────────────────────────────────────────────
	case OpExt:
		result = truncate(a, ty) // zero-extend: just mask to dst width
	case OpSext:
		srcBits := 8
		if inst.SrcTy != nil {
			srcBits = inst.SrcTy.Width()
		}
		result = truncate(signExtend(a, srcBits), ty)
	case OpTrunc:
		result = truncate(a, ty) // truncate: mask to narrower width

	// ── Comparison ──────────────────────────────────────────────────────────
	case OpCmp:
		var ok bool
		ua, ub := uint64(a.I), uint64(b.I)
		// For signed comparisons, sign-extend operands from their source width.
		// Values are stored truncated (e.g. i8 -5 → 251), so we must recover
		// the sign bit before comparing with Go's int64 operators.
		sa, sb := a, b
		if inst.SrcTy != nil {
			w := inst.SrcTy.Width()
			sa = signExtend(a, w)
			sb = signExtend(b, w)
		}
		switch inst.Cond {
		case CmpEq:
			ok = a.I == b.I
		case CmpNe:
			ok = a.I != b.I
		case CmpLt:
			ok = sa.I < sb.I
		case CmpLe:
			ok = sa.I <= sb.I
		case CmpGt:
			ok = sa.I > sb.I
		case CmpGe:
			ok = sa.I >= sb.I
		case CmpUlt:
			ok = ua < ub
		case CmpUle:
			ok = ua <= ub
		case CmpUgt:
			ok = ua > ub
		case CmpUge:
			ok = ua >= ub
		case CmpSubCarry:
			// Src[0]=r where r = orig_a - orig_b (mod 2^width).
			// Carry from sub ≡ (r + b) >= 2^width ≡ orig_a < orig_b unsigned.
			// Use SrcTy (operand width), not Ty (result=TyBool with Width()=1).
			width := uint(inst.SrcTy.Width())
			ok = (ua + ub) >= (1 << width)
		case CmpSubCarryNot:
			// Complement of CmpSubCarry: no borrow ≡ orig_a >= orig_b unsigned.
			width := uint(inst.SrcTy.Width())
			ok = (ua + ub) < (1 << width)
		}
		if ok {
			result = Value{I: 1}
		}

	// ── Data movement ────────────────────────────────────────────────────────
	case OpConst:
		result = truncate(Value{I: inst.Imm}, ty)
	case OpMove:
		result = truncate(a, ty)

	// ── Memory ───────────────────────────────────────────────────────────────
	case OpAlloca:
		offset := int64(len(vm.heap))
		needed := int(inst.Imm)
		if len(vm.heap)+needed > vm.maxMem() {
			return fmt.Errorf("alloca: heap limit reached (%d bytes)", vm.maxMem())
		}
		vm.heap = append(vm.heap, make([]byte, needed)...)
		result = Value{I: offset}

	case OpAddrOf:
		// Look up global / string symbol.
		offset, err := vm.resolveSymbol(inst.Sym)
		if err != nil {
			return err
		}
		result = Value{I: offset}

	case OpField:
		// Keep the legacy SoA256 encoding consistent with native Z80.
		if inst.Imm == -1 {
			result = truncate(Value{I: (a.I &^ 255) | ((a.I + 1) & 255)}, ty)
		} else {
			result = truncate(Value{I: a.I + inst.Imm}, ty)
		}

	case OpPtrAdd:
		// Advance pointer by runtime offset: ptr + offset.
		result = truncate(Value{I: a.I + b.I}, ty)

	case OpPtrBump:
		// The legacy -1 encoding means SoA256 low-byte advance.
		if inst.Imm == -1 {
			result = truncate(Value{I: (a.I &^ 255) | ((a.I + 1) & 255)}, ty)
		} else {
			result = truncate(Value{I: a.I + inst.Imm}, ty)
		}

	case OpLoad:
		if inst.SequentialBytes && (ty != TyU16 || len(inst.SequentialByteOrigins) != 2 || inst.SequentialByteOrigins[0] == nil || inst.SequentialByteOrigins[1] == nil) {
			return fmt.Errorf("invalid sequential byte load")
		}
		offset := int(a.I)
		width := ByteWidth(ty)
		mask := fr.pointerMask(inst.Src[0])
		if !inst.SequentialBytes && !vm.wrappedHeapRangeValid(a.I, width, mask) {
			return fmt.Errorf("load: out-of-bounds address %d (heap size %d)", offset, len(vm.heap))
		}
		var v int64
		if observe := vm.ObserveDataAccess; observe != nil || vm.CheckDataAccess != nil || inst.SequentialBytes {
			for i := 0; i < width; i++ {
				addr := int64((uint64(a.I) + uint64(i)) & mask)
				if inst.SequentialBytes && !vm.wrappedHeapRangeValid(addr, 1, mask) {
					return fmt.Errorf("sequential byte access: out-of-bounds address %d (heap size %d)", addr, len(vm.heap))
				}
				if vm.CheckDataAccess != nil {
					if err := vm.CheckDataAccess(sequentialByteInstruction(inst, i), addr); err != nil {
						return err
					}
				}
				value := vm.heap[int(addr)]
				v |= int64(value) << (8 * i)
				if observe != nil {
					observe(VMDataAccess{Op: OpLoad, Addr: addr, Value: value})
				}
			}
		} else {
			for i := 0; i < width; i++ {
				v |= int64(vm.heap[int((uint64(a.I)+uint64(i))&mask)]) << (8 * i)
			}
		}
		result = truncate(Value{I: v}, ty)

	case OpStore:
		if inst.SequentialBytes && (ty != TyU16 || len(inst.SequentialByteOrigins) != 2 || inst.SequentialByteOrigins[0] == nil || inst.SequentialByteOrigins[1] == nil) {
			return fmt.Errorf("invalid sequential byte store")
		}
		// Src[0]=ptr, Src[1]=val; ty is value type.
		offset := int(a.I)
		val := b.I
		width := ByteWidth(ty)
		mask := fr.pointerMask(inst.Src[0])
		if !inst.SequentialBytes && !vm.wrappedHeapRangeValid(a.I, width, mask) {
			return fmt.Errorf("store: out-of-bounds address %d (heap size %d)", offset, len(vm.heap))
		}
		if observe := vm.ObserveDataAccess; observe != nil || vm.CheckDataAccess != nil || inst.SequentialBytes {
			for i := 0; i < width; i++ {
				addr := int64((uint64(a.I) + uint64(i)) & mask)
				if inst.SequentialBytes && !vm.wrappedHeapRangeValid(addr, 1, mask) {
					return fmt.Errorf("sequential byte access: out-of-bounds address %d (heap size %d)", addr, len(vm.heap))
				}
				if vm.CheckDataAccess != nil {
					if err := vm.CheckDataAccess(sequentialByteInstruction(inst, i), addr); err != nil {
						return err
					}
				}
				value := byte(val >> (8 * i))
				vm.heap[int(addr)] = value
				if observe != nil {
					observe(VMDataAccess{Op: OpStore, Addr: addr, Value: value})
				}
			}
		} else {
			for i := 0; i < width; i++ {
				vm.heap[int((uint64(a.I)+uint64(i))&mask)] = byte(val >> (8 * i))
			}
		}
		return nil // no Dst

	// ── SMC — mutable immediates ──────────────────────────────────────────────
	// Named slots retain immediate values; anonymous slots use heap addresses.
	case OpPatchSlot:
		if inst.Sym != "" {
			vm.initNamedSMC()
			result = vm.namedSMC[inst.Sym]
			break
		}
		offset := int64(len(vm.heap))
		width := ByteWidth(ty)
		if width == 0 {
			width = 2
		}
		vm.heap = append(vm.heap, make([]byte, width)...)
		// Write initial value.
		init := inst.Imm
		for i := 0; i < width; i++ {
			vm.heap[int(offset)+i] = byte(init >> (8 * i))
		}
		result = Value{I: offset} // slot = pointer to the mutable byte(s)

	case OpLoadPatched:
		// a = slot (heap offset), ty = value type
		offset := int(a.I)
		width := ByteWidth(ty)
		mask := fr.pointerMask(inst.Src[0])
		if !vm.wrappedHeapRangeValid(a.I, width, mask) {
			return fmt.Errorf("load_patched: out-of-bounds slot %d", offset)
		}
		var v int64
		for i := 0; i < width; i++ {
			v |= int64(vm.heap[int((uint64(a.I)+uint64(i))&mask)]) << (8 * i)
		}
		result = truncate(Value{I: v}, ty)

	case OpPatch:
		if inst.Sym != "" {
			vm.initNamedSMC()
			if _, ok := vm.namedSMC[inst.Sym]; !ok {
				return fmt.Errorf("unknown named SMC slot %s", inst.Sym)
			}
			vm.namedSMC[inst.Sym] = truncate(b, ty)
			return nil
		}
		// Src[0]=slot, Src[1]=new value; Ty = value type of the slot.
		// (We infer width from the slot content; use 2 as safe default.)
		offset := int(a.I)
		val := b.I
		width := 2
		if ty != nil {
			width = ByteWidth(ty)
		}
		mask := fr.pointerMask(inst.Src[0])
		if !vm.wrappedHeapRangeValid(a.I, width, mask) {
			return fmt.Errorf("patch: out-of-bounds slot %d", offset)
		}
		for i := 0; i < width; i++ {
			vm.heap[int((uint64(a.I)+uint64(i))&mask)] = byte(val >> (8 * i))
		}
		return nil // no Dst

	// ── Function calls ────────────────────────────────────────────────────────
	case OpCall, OpCallCond:
		if inst.ByteLoop != nil {
			return vm.execByteLoop(fr, inst)
		}
		if inst.Op == OpCallCond {
			take, err := conditionalCallTaken(fr, inst)
			if err != nil {
				return err
			}
			if !take {
				return nil
			}
		}
		if inst.FallibleCY {
			fn := vm.Module.FuncByName(inst.Sym)
			if fn == nil || !fn.Attrs.FallibleCY || len(inst.ExtraRets) != 2 {
				return fmt.Errorf("invalid fallible call %s", inst.Sym)
			}
			out, err := vm.callFuncOutcome(fn, collectArgs(fr, inst.Args))
			if err != nil {
				return err
			}
			value := Value{}
			if len(out.Values) > 0 {
				value = out.Values[0]
			}
			if out.Failed {
				value.I = int64(out.ErrorCode)
			}
			if inst.Dst != NoReg {
				fr.set(inst.Dst, value)
			}
			status := int64(0)
			if out.Failed {
				status = 1
			}
			fr.set(inst.ExtraRets[0], Value{I: status})
			fr.set(inst.ExtraRets[1], Value{I: int64(out.ErrorCode)})
			return nil
		}
		rets, err := vm.execCall(inst.Sym, inst.Args, fr)
		if err != nil {
			return err
		}
		if inst.Dst != NoReg && len(rets) > 0 {
			if _, pointer := inst.Ty.(*ptrTy); pointer {
				fr.set(inst.Dst, truncate(rets[0], inst.Ty))
			} else {
				fr.set(inst.Dst, rets[0])
			}
		}
		// Capture extra return values (positions 1, 2, …).
		for i, r := range inst.ExtraRets {
			if r != NoReg && i+1 < len(rets) {
				fr.set(r, rets[i+1])
			}
		}
		return nil

	case OpCallIndirect:
		// Src[0] is a function pointer. In the VM, we resolve it by scanning
		// module functions — the pointer value is the address assigned by
		// OpAddrOf("funcName"), which is the heap offset of the function's
		// name string. We look up the name and dispatch.
		ptrVal := fr.get(inst.Src[0]).I
		args := collectArgs(fr, inst.Args)
		// Try to find a function whose global symbol address matches ptrVal.
		var targetFn *Func
		for _, fn := range vm.Module.Funcs {
			if addr, ok := vm.globalSyms[fn.Name]; ok && addr == ptrVal {
				targetFn = fn
				break
			}
		}
		if targetFn == nil {
			return fmt.Errorf("mir2.VM: OpCallIndirect: no function at address %d", ptrVal)
		}
		if err := ValidateIndirectTargetABI(inst, targetFn); err != nil {
			return err
		}
		rets, err := vm.callFunc(targetFn, args)
		if err != nil {
			return err
		}
		if inst.Dst != NoReg && len(rets) > 0 {
			if _, pointer := inst.Ty.(*ptrTy); pointer {
				fr.set(inst.Dst, truncate(rets[0], inst.Ty))
			} else {
				fr.set(inst.Dst, rets[0])
			}
		}
		for i, r := range inst.ExtraRets {
			if r != NoReg && i+1 < len(rets) {
				fr.set(r, rets[i+1])
			}
		}
		return nil

	// ── Inline asm ────────────────────────────────────────────────────────────
	case OpAsm:
		// Cannot execute target-specific asm in the VM.
		// Return zero and continue (lenient) — comptime code shouldn't use asm.
		result = zeroValue

	// ── Port I/O ─────────────────────────────────────────────────────────────
	case OpIn8:
		port := uint16(inst.Imm & 0xFF)
		if vm.Ports != nil {
			result = Value{I: int64(vm.Ports.ReadPort(port))}
		} else {
			result = zeroValue
		}
	case OpOut8:
		port := uint16(inst.Imm & 0xFF)
		val := byte(fr.get(inst.Src[0]).I & 0xFF)
		if vm.Ports != nil {
			vm.Ports.WritePort(port, val)
		}
		result = zeroValue
	case OpIn16:
		// 16-bit port address, 8-bit data (Z80: IN r,(C) with BC=port)
		port := uint16(inst.Imm & 0xFFFF)
		if vm.Ports != nil {
			result = Value{I: int64(vm.Ports.ReadPort(port))}
		} else {
			result = zeroValue
		}
	case OpOut16:
		// 16-bit port address, 8-bit data (Z80: OUT (C),r with BC=port)
		port := uint16(inst.Imm & 0xFFFF)
		val := byte(fr.get(inst.Src[0]).I & 0xFF)
		if vm.Ports != nil {
			vm.Ports.WritePort(port, val)
		}
		result = zeroValue

	default:
		return fmt.Errorf("mir2.VM: unhandled opcode %s", inst.Op)
	}

	if _, pointer := inst.Ty.(*ptrTy); pointer {
		result = truncate(result, inst.Ty)
	}
	fr.set(inst.Dst, result)
	return nil
}

// execCall dispatches a named call: host table first, then module functions.
func (vm *VM) execCall(sym string, argRegs []Reg, fr *frame) ([]Value, error) {
	args := collectArgs(fr, argRegs)

	// Host / intrinsic?
	if hf, ok := vm.Hosts[sym]; ok {
		return hf(args)
	}

	// Module function (strip leading @ if present)
	name := sym
	if len(name) > 0 && name[0] == '@' {
		name = name[1:]
	}
	fn := vm.Module.FuncByName(name)
	if fn == nil {
		return nil, fmt.Errorf("mir2.VM: unknown function %s", sym)
	}
	return vm.callFunc(fn, args)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func collectArgs(fr *frame, regs []Reg) []Value {
	out := make([]Value, len(regs))
	for i, r := range regs {
		out[i] = fr.get(r)
	}
	return out
}

func (vm *VM) step() error {
	vm.steps++
	if vm.MaxSteps > 0 && vm.steps > vm.MaxSteps {
		return fmt.Errorf("mir2.VM: gas limit reached (%d steps)", vm.MaxSteps)
	}
	return nil
}

func (vm *VM) maxMem() int {
	if vm.MaxMemory > 0 {
		return vm.MaxMemory
	}
	return 65536
}

// ResolveSymbol returns the heap offset of a named symbol,
// allocating string literals into heap on first reference.
func (vm *VM) ResolveSymbol(sym string) (int64, error) {
	return vm.resolveSymbol(sym)
}

func (vm *VM) resolveSymbol(sym string) (int64, error) {
	// Module globals (pre-allocated in NewVM).
	if off, ok := vm.globalSyms[sym]; ok {
		return off, nil
	}
	// String literals: "@mir2.str.N"
	var idx int
	if n, _ := fmt.Sscanf(sym, "@mir2.str.%d", &idx); n == 1 {
		if idx < 0 || idx >= vm.Module.Strings.Len() {
			return 0, fmt.Errorf("mir2.VM: string index %d out of range", idx)
		}
		s := vm.Module.Strings.At(idx)
		offset := int64(len(vm.heap))
		kind := vm.Module.Strings.Kind(idx)
		overhead := 1
		if kind == StrLString {
			overhead = 2
		}
		limit := vm.MaxMemory
		if limit <= 0 {
			limit = 65536
		}
		if len(vm.heap) > limit || len(s)+overhead > limit-len(vm.heap) {
			return 0, fmt.Errorf("mir2.VM: string storage exceeds %d byte memory limit", limit)
		}
		switch kind {
		case StrSString:
			if len(s) > 255 {
				return 0, fmt.Errorf("mir2.VM: SString exceeds 255 bytes")
			}
			vm.heap = append(vm.heap, byte(len(s)))
		case StrLString:
			if len(s) > 65535 {
				return 0, fmt.Errorf("mir2.VM: LString exceeds 65535 bytes")
			}
			vm.heap = append(vm.heap, byte(len(s)), byte(len(s)>>8))
		case StrCString:
		default:
			return 0, fmt.Errorf("mir2.VM: invalid string kind %d", kind)
		}
		vm.heap = append(vm.heap, []byte(s)...)
		if kind == StrCString {
			vm.heap = append(vm.heap, 0)
		}
		// Cache so repeated references get the same address.
		vm.globalSyms[sym] = offset
		return offset, nil
	}
	return 0, fmt.Errorf("mir2.VM: cannot resolve symbol %q", sym)
}

// ── Built-in host functions (@mir.*) ─────────────────────────────────────────

func (vm *VM) registerBuiltinHosts() {
	// I/O
	vm.Hosts["@mir.io.print.char"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		_, err := os.Stdout.Write([]byte{byte(args[0].I)})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	vm.Hosts["@mir.io.print.u8"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		fmt.Printf("%d", uint8(args[0].I))
		return nil, nil
	}
	vm.Hosts["@mir.io.print.u16"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		fmt.Printf("%d", uint16(args[0].I))
		return nil, nil
	}
	vm.Hosts["@mir.io.print.i16"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		fmt.Printf("%d", int16(args[0].I))
		return nil, nil
	}
	vm.Hosts["@mir.io.print.bool"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		if args[0].I != 0 {
			fmt.Print("true")
		} else {
			fmt.Print("false")
		}
		return nil, nil
	}
	vm.Hosts["@mir.io.print.nl"] = func(_ []Value) ([]Value, error) {
		fmt.Println()
		return nil, nil
	}
	vm.Hosts["@mir.io.print.str"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		// arg is a pointer into the VM heap — read null-terminated string
		ptr := args[0].I
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		fmt.Print(string(buf))
		return nil, nil
	}
	vm.Hosts["@mir.io.print.dec"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		fmt.Printf("%d", uint8(args[0].I))
		return nil, nil
	}
	vm.Hosts["@mir.io.console.log"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		ptr := args[0].I
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		fmt.Fprintf(os.Stderr, "%s\n", string(buf))
		return nil, nil
	}
	vm.Hosts["@mir.io.console.err"] = func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, nil
		}
		ptr := args[0].I
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		fmt.Fprintf(os.Stderr, "%s\n", string(buf))
		return nil, nil
	}
	vm.Hosts["@mir.io.halt"] = func(_ []Value) ([]Value, error) {
		return nil, fmt.Errorf("mir2.VM: halt")
	}

	// Math
	vm.Hosts["@mir.math.abs"] = func(args []Value) ([]Value, error) {
		v := args[0].I
		if v < 0 {
			v = -v
		}
		return []Value{{I: v}}, nil
	}
	vm.Hosts["@mir.math.min"] = func(args []Value) ([]Value, error) {
		if args[0].I < args[1].I {
			return []Value{args[0]}, nil
		}
		return []Value{args[1]}, nil
	}
	vm.Hosts["@mir.math.max"] = func(args []Value) ([]Value, error) {
		if args[0].I > args[1].I {
			return []Value{args[0]}, nil
		}
		return []Value{args[1]}, nil
	}
	vm.Hosts["@mir.math.clz"] = func(args []Value) ([]Value, error) {
		v := uint64(args[0].I)
		if v == 0 {
			return []Value{{I: 64}}, nil
		}
		return []Value{{I: int64(math.Log2(float64(v)))}}, nil // approx for now
	}

	// Memory
	vm.Hosts["@mir.mem.memset"] = func(args []Value) ([]Value, error) {
		// args: dst_ptr, byte_val, count
		if len(args) < 3 {
			return nil, fmt.Errorf("memset: need 3 args")
		}
		dst := int(args[0].I)
		val := byte(args[1].I)
		n := int(args[2].I)
		if dst+n > len(vm.heap) {
			return nil, fmt.Errorf("memset: out of bounds")
		}
		for i := 0; i < n; i++ {
			vm.heap[dst+i] = val
		}
		return nil, nil
	}
	vm.Hosts["@mir.mem.memcpy"] = func(args []Value) ([]Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("memcpy: need 3 args")
		}
		dst := int(args[0].I)
		src := int(args[1].I)
		n := int(args[2].I)
		if dst+n > len(vm.heap) || src+n > len(vm.heap) {
			return nil, fmt.Errorf("memcpy: out of bounds")
		}
		copy(vm.heap[dst:], vm.heap[src:src+n])
		return nil, nil
	}
}

func (fr *frame) pointerMask(r Reg) uint64 {
	bits := fr.pointerWidths[r]
	if bits == 0 {
		bits = 16
	}
	if bits >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << uint(bits)) - 1
}
func (vm *VM) wrappedHeapRangeValid(address int64, width int, mask uint64) bool {
	if address < 0 || width < 0 {
		return false
	}
	for i := 0; i < width; i++ {
		addr := (uint64(address) + uint64(i)) & mask
		if addr >= uint64(len(vm.heap)) {
			return false
		}
	}
	return true
}

func sequentialByteInstruction(inst *Inst, byteIndex int) *Inst {
	if inst.SequentialBytes && len(inst.SequentialByteOrigins) == 2 {
		return inst.SequentialByteOrigins[byteIndex]
	}
	return inst
}

// OpCallCond consumes an already computed boolean. Cond is its native flag
// predicate and may invert the producer when the else-arm was folded.
func conditionalCallTaken(fr *frame, inst *Inst) (bool, error) {
	predicate := CmpNe // documented fallback for a non-comparison boolean
	// Use the call's own block, exactly as FoldConditionalCalls does. A
	// dominating comparison in another block intentionally uses CmpNe fallback.
	for _, b := range fr.fn.Blocks {
		for _, i := range b.Insts {
			if i == inst {
				predicate = findCondForReg(b, inst.Src[0])
			}
		}
	}
	take := fr.get(inst.Src[0]).I != 0
	if inst.Cond == predicate {
		return take, nil
	}
	if inst.Cond == invertCmpCond(predicate) {
		return !take, nil
	}
	return false, fmt.Errorf("mir2.VM: ambiguous conditional call predicate %v for boolean %v", inst.Cond, predicate)
}
