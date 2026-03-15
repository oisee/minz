package mir2

import (
	"fmt"
	"math"
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
type VM struct {
	Module *Module

	// Host function table.
	// Key: full intrinsic name (e.g. "@mir.io.print.u8", "@host.emit.code")
	// Value: Go function that receives args and returns results.
	Hosts map[string]HostFunc

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
		MaxSteps:   100_000,
		MaxMemory:  65536,
		globalSyms: make(map[string]int64),
	}
	vm.heap = make([]byte, 0, 4096)
	// Pre-allocate all module globals into the heap.
	for _, g := range m.Globals {
		offset := int64(len(vm.heap))
		vm.globalSyms[g.Name] = offset
		if len(g.Init) > 0 {
			vm.heap = append(vm.heap, g.Init...)
		} else {
			vm.heap = append(vm.heap, make([]byte, ByteWidth(g.Ty))...)
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

	return vm.execBlock(fr, entry, nil)
}

// execBlock executes blk, binding blockArgs to the block's params first.
// Returns when a TermRet is reached.
func (vm *VM) execBlock(fr *frame, blk *Block, blockArgs []Value) ([]Value, error) {
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

	// Execute terminator.
	return vm.execTerm(fr, blk)
}

// execTerm follows a terminator, potentially jumping to another block.
func (vm *VM) execTerm(fr *frame, blk *Block) ([]Value, error) {
	switch t := blk.Term.(type) {

	case *TermRet:
		rets := make([]Value, len(t.Vals))
		for i, r := range t.Vals {
			rets[i] = fr.get(r)
		}
		return rets, nil

	case *TermJmp:
		args := collectArgs(fr, t.Args)
		next := fr.fn.BlockByLabel(t.Target)
		if next == nil {
			return nil, fmt.Errorf("jmp to unknown block @%s", t.Target)
		}
		return vm.execBlock(fr, next, args)

	case *TermBrIf:
		cond := fr.get(t.Cond)
		if cond.I != 0 {
			args := collectArgs(fr, t.ThenArgs)
			next := fr.fn.BlockByLabel(t.Then)
			if next == nil {
				return nil, fmt.Errorf("br_if then: unknown block @%s", t.Then)
			}
			return vm.execBlock(fr, next, args)
		}
		args := collectArgs(fr, t.ElseArgs)
		next := fr.fn.BlockByLabel(t.Else)
		if next == nil {
			return nil, fmt.Errorf("br_if else: unknown block @%s", t.Else)
		}
		return vm.execBlock(fr, next, args)

	case *TermBrIf2:
		lhs := fr.get(t.Lhs).I
		rhs := fr.get(t.Rhs).I
		var target string
		var args []Reg
		switch {
		case lhs == rhs:
			target, args = t.Eq, t.EqArgs
		case uint64(lhs) < uint64(rhs):
			target, args = t.Lt, t.LtArgs
		default:
			target, args = t.Gt, t.GtArgs
		}
		next := fr.fn.BlockByLabel(target)
		if next == nil {
			return nil, fmt.Errorf("br_if2: unknown block @%s", target)
		}
		return vm.execBlock(fr, next, collectArgs(fr, args))

	case *TermDJNZ:
		// Decrement counter, branch to Body if non-zero, else Exit.
		cval := fr.get(t.Counter).I
		cval-- // decrement
		if cval != 0 {
			// Body: first block param receives the decremented counter.
			// BodyArgs are args for body.Params[1:].
			bodyBlock := fr.fn.BlockByLabel(t.Body)
			if bodyBlock == nil {
				return nil, fmt.Errorf("djnz body: unknown block @%s", t.Body)
			}
			// Build args: [decremented counter] + BodyArgs.
			bodyArgVals := make([]Value, 1+len(t.BodyArgs))
			bodyArgVals[0] = Value{I: cval}
			for i, r := range t.BodyArgs {
				bodyArgVals[i+1] = fr.get(r)
			}
			return vm.execBlock(fr, bodyBlock, bodyArgVals)
		}
		// Exit path.
		exitBlock := fr.fn.BlockByLabel(t.Exit)
		if exitBlock == nil {
			return nil, fmt.Errorf("djnz exit: unknown block @%s", t.Exit)
		}
		return vm.execBlock(fr, exitBlock, collectArgs(fr, t.ExitArgs))

	case *TermCondRet:
		cond := fr.get(t.Cond)
		if cond.I == 0 {
			// Return path (cond is false/zero).
			rets := make([]Value, len(t.Vals))
			for i, r := range t.Vals {
				rets[i] = fr.get(r)
			}
			return rets, nil
		}
		// Jump path (cond is non-zero).
		args := collectArgs(fr, t.ThenArgs)
		next := fr.fn.BlockByLabel(t.Then)
		if next == nil {
			return nil, fmt.Errorf("cond_ret then: unknown block @%s", t.Then)
		}
		return vm.execBlock(fr, next, args)

	case *TermUnreachable:
		return nil, fmt.Errorf("mir2.VM: reached unreachable in @%s block @%s",
			fr.fn.Name, blk.Label)
	}

	return nil, fmt.Errorf("mir2.VM: unknown terminator type in @%s", fr.fn.Name)
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
			ok = (ua+ub) >= (1 << width)
		case CmpSubCarryNot:
			// Complement of CmpSubCarry: no borrow ≡ orig_a >= orig_b unsigned.
			width := uint(inst.SrcTy.Width())
			ok = (ua+ub) < (1 << width)
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
		// Indirect: Src[0] is a heap offset pointing to the function name
		// (VM convention: function pointers are interned symbol offsets).
		// For now, resolve via a synthetic symbol table.
		return fmt.Errorf("mir2.VM: OpCallIndirect not yet implemented")

	// ── Inline asm ────────────────────────────────────────────────────────────
	case OpAsm:
		// Cannot execute target-specific asm in the VM.
		// Return zero and continue (lenient) — comptime code shouldn't use asm.
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

// resolveSymbol returns the heap offset of a named symbol,
// allocating string literals into heap on first reference.
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
