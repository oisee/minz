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
// PortIO is the interface for I/O port access, shared across all runtimes
// (MZV VM, MZE Z80 emulator, MZX ZX Spectrum). One harness, all platforms.
type PortIO interface {
	ReadPort(address uint16) byte
	WritePort(address uint16, b byte)
}

type VM struct {
	Module *Module

	// Host function table.
	// Key: full intrinsic name (e.g. "@mir.io.print.u8", "@host.emit.code")
	// Value: Go function that receives args and returns results.
	Hosts map[string]HostFunc

	// I/O port handler — same interface as emulator.Ports.
	// When set, OpIn8/OpOut8/OpIn16/OpOut16 dispatch here.
	// Console ($23), network ($30), control ($31) — all through one interface.
	Ports PortIO

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
}

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
	fn   *Func
	vals map[Reg]Value // virtual register file for this call
}

func newFrame(fn *Func) *frame {
	return &frame{fn: fn, vals: make(map[Reg]Value, 32)}
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
	// Pre-allocate all module globals into the heap.
	for _, g := range m.Globals {
		offset := int64(len(vm.heap))
		vm.globalSyms[g.Name] = offset
		w := ByteWidth(g.Ty)
		if len(g.Init) > 0 {
			vm.heap = append(vm.heap, g.Init...)
			// Pad if Init is shorter than type width.
			if len(g.Init) < w {
				vm.heap = append(vm.heap, make([]byte, w-len(g.Init))...)
			}
		} else {
			vm.heap = append(vm.heap, make([]byte, w)...)
		}
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
	vm.registerBuiltinHosts()
	return vm
}

// Call executes a named function with the given argument values.
// Returns the return values (empty for void).
func (vm *VM) Call(funcName string, args []Value) ([]Value, error) {
	fn := vm.Module.FuncByName(funcName)
	if fn == nil {
		return nil, fmt.Errorf("mir2.VM: function @%s not found", funcName)
	}
	return vm.callFunc(fn, args)
}

// CallFunc executes a Func directly.
func (vm *VM) CallFunc(fn *Func, args []Value) ([]Value, error) {
	return vm.callFunc(fn, args)
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
	if fn.Attrs.IsExtern {
		// Try host table by function name.
		if hf, ok := vm.Hosts["@"+fn.Name]; ok {
			return hf(args)
		}
		return nil, fmt.Errorf("mir2.VM: extern @%s has no host implementation", fn.Name)
	}

	entry := fn.Entry()
	if entry == nil {
		return nil, fmt.Errorf("mir2.VM: @%s has no entry block", fn.Name)
	}

	fr := newFrame(fn)

	// Bind function parameters.
	params := fn.Contract.Params
	if len(args) != len(params) {
		return nil, fmt.Errorf("mir2.VM: @%s expects %d args, got %d",
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
			return nil, fmt.Errorf("mir2.VM: block @%s expects %d params, got %d args",
				blk.Label, len(blk.Params), len(blockArgs))
		}
		for i, p := range blk.Params {
			fr.set(p.Dst, truncate(blockArgs[i], p.Ty))
		}

		// Execute instructions.
		for _, inst := range blk.Insts {
			if err := vm.step(); err != nil {
				return nil, err
			}
			if err := vm.execInst(fr, inst); err != nil {
				return nil, fmt.Errorf("block @%s: %w", blk.Label, err)
			}
		}

		// Resolve terminator → either return or jump to next block.
		rets, next, nextArgs, err := vm.execTerm(fr, blk)
		if err != nil {
			return nil, err
		}
		if next == nil {
			// TermRet or TermCondRet(return path) — done.
			return rets, nil
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
		if b.I == 0 {
			return fmt.Errorf("signed division by zero")
		}
		result = truncate(Value{I: a.I / b.I}, ty)
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
		shift := uint(b.I) & 63
		result = truncate(Value{I: a.I << shift}, ty)
	case OpShr:
		shift := uint(b.I) & 63
		result = truncate(Value{I: int64(uint64(a.I) >> shift)}, ty)
	case OpSar:
		shift := uint(b.I) & 63
		result = truncate(Value{I: a.I >> shift}, ty)

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
		// Advance pointer by compile-time byte offset (struct field access).
		result = Value{I: a.I + inst.Imm}

	case OpPtrAdd:
		// Advance pointer by runtime offset: ptr + offset.
		result = Value{I: a.I + b.I}

	case OpPtrBump:
		// Advance iterator pointer by compile-time stride.
		result = Value{I: a.I + inst.Imm}

	case OpLoad:
		offset := int(a.I)
		width := ByteWidth(ty)
		if offset < 0 || offset+width > len(vm.heap) {
			return fmt.Errorf("load: out-of-bounds address %d (heap size %d)", offset, len(vm.heap))
		}
		var v int64
		for i := 0; i < width; i++ {
			v |= int64(vm.heap[offset+i]) << (8 * i)
		}
		result = truncate(Value{I: v}, ty)

	case OpStore:
		// Src[0]=ptr, Src[1]=val; ty is value type.
		offset := int(a.I)
		val := b.I
		width := ByteWidth(ty)
		if offset < 0 || offset+width > len(vm.heap) {
			return fmt.Errorf("store: out-of-bounds address %d (heap size %d)", offset, len(vm.heap))
		}
		for i := 0; i < width; i++ {
			vm.heap[offset+i] = byte(val >> (8 * i))
		}
		return nil // no Dst

	// ── SMC — mutable immediates ──────────────────────────────────────────────
	// In the VM, patch_slot is simply a heap slot.
	case OpPatchSlot:
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
		if offset < 0 || offset+width > len(vm.heap) {
			return fmt.Errorf("load_patched: out-of-bounds slot %d", offset)
		}
		var v int64
		for i := 0; i < width; i++ {
			v |= int64(vm.heap[offset+i]) << (8 * i)
		}
		result = truncate(Value{I: v}, ty)

	case OpPatch:
		// Src[0]=slot, Src[1]=new value; Ty = value type of the slot.
		// (We infer width from the slot content; use 2 as safe default.)
		offset := int(a.I)
		val := b.I
		width := 2
		if ty != nil {
			width = ByteWidth(ty)
		}
		if offset < 0 || offset+width > len(vm.heap) {
			return fmt.Errorf("patch: out-of-bounds slot %d", offset)
		}
		for i := 0; i < width; i++ {
			vm.heap[offset+i] = byte(val >> (8 * i))
		}
		return nil // no Dst

	// ── Function calls ────────────────────────────────────────────────────────
	case OpCall:
		rets, err := vm.execCall(inst.Sym, inst.Args, fr)
		if err != nil {
			return err
		}
		if inst.Dst != NoReg && len(rets) > 0 {
			fr.set(inst.Dst, rets[0])
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
		rets, err := vm.callFunc(targetFn, args)
		if err != nil {
			return err
		}
		if inst.Dst != NoReg && len(rets) > 0 {
			fr.set(inst.Dst, rets[0])
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
		vm.heap = append(vm.heap, []byte(s)...)
		vm.heap = append(vm.heap, 0) // NUL terminator
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
		fmt.Printf("%c", rune(args[0].I))
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
