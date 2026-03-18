package lir

import "fmt"

// VM executes a LIR program on a simulated register file.
// Used for convergence testing: MIR2-VM result must equal LIR-VM result
// on every machine descriptor.
type VM struct {
	Desc   *MachineDesc
	Regs   []uint64 // register file indexed by Loc position
	Memory []byte   // flat memory (64K)
	Flags  uint64   // CPU flags
}

// NewVM creates a VM for the given machine.
func NewVM(desc *MachineDesc) *VM {
	return &VM{
		Desc:   desc,
		Regs:   make([]uint64, len(desc.Locs)),
		Memory: make([]byte, 65536),
	}
}

// Reset clears all registers and memory.
func (vm *VM) Reset() {
	for i := range vm.Regs {
		vm.Regs[i] = 0
	}
	for i := range vm.Memory {
		vm.Memory[i] = 0
	}
	vm.Flags = 0
}

// Get returns the value in physical location i.
func (vm *VM) Get(i int) uint64 {
	if i < 0 || i >= len(vm.Regs) {
		return 0
	}
	mask := uint64((1 << vm.Desc.Locs[i].Width) - 1)
	return vm.Regs[i] & mask
}

// Set stores a value in physical location i, masked to location width.
func (vm *VM) Set(i int, val uint64) {
	if i < 0 || i >= len(vm.Regs) {
		return
	}
	mask := uint64((1 << vm.Desc.Locs[i].Width) - 1)
	vm.Regs[i] = val & mask
}

// ExecInst executes one LIR instruction.
func (vm *VM) ExecInst(inst *Inst) error {
	dst := inst.Dst.Phys
	src0 := inst.Srcs[0].Phys
	src1 := inst.Srcs[1].Phys

	switch inst.Pat.MIROp {
	case OpConst:
		vm.Set(dst, uint64(inst.Imm))

	case OpMove:
		vm.Set(dst, vm.Get(src0))

	case OpAdd:
		// If src1 is unused (-1), use Imm as the second operand (field offset, etc.)
		var b uint64
		if src1 >= 0 {
			b = vm.Get(src1)
		} else {
			b = uint64(inst.Imm)
		}
		vm.Set(dst, vm.Get(src0)+b)

	case OpSub:
		a, b := vm.Get(src0), vm.Get(src1)
		vm.Set(dst, a-b)
		// Set carry flag: borrow if a < b
		if a < b {
			vm.Flags |= 1
		} else {
			vm.Flags &^= 1
		}

	case OpMul:
		vm.Set(dst, vm.Get(src0)*vm.Get(src1))

	case OpAnd:
		vm.Set(dst, vm.Get(src0)&vm.Get(src1))

	case OpOr:
		vm.Set(dst, vm.Get(src0)|vm.Get(src1))

	case OpXor:
		vm.Set(dst, vm.Get(src0)^vm.Get(src1))

	case OpCmp:
		a, b := vm.Get(src0), vm.Get(src1)
		if a == b {
			vm.Flags = 0x40 // Z flag
		} else if a < b {
			vm.Flags = 0x01 // C flag
		} else {
			vm.Flags = 0
		}
		vm.Set(dst, vm.Flags)

	case OpLoad:
		addr := vm.Get(src0)
		// Use pattern width if specified, else min(register width, 8).
		// Pattern width=0 means "any width" — default to 8 for byte loads.
		w := 8
		if inst.Pat != nil && inst.Pat.Width > 0 {
			w = inst.Pat.Width
		}
		var val uint64
		for i := 0; i < w/8; i++ {
			val |= uint64(vm.Memory[(addr+uint64(i))&0xFFFF]) << (i * 8)
		}
		vm.Set(dst, val)

	case OpStore:
		addr := vm.Get(src0)
		val := vm.Get(src1)
		w := 8
		if inst.Pat != nil && inst.Pat.Width > 0 {
			w = inst.Pat.Width
		}
		for i := 0; i < w/8; i++ {
			vm.Memory[(addr+uint64(i))&0xFFFF] = byte(val >> (i * 8))
		}

	case OpShl:
		vm.Set(dst, vm.Get(src0)<<vm.Get(src1))

	case OpShr:
		vm.Set(dst, vm.Get(src0)>>vm.Get(src1))

	case OpLoad16LE:
		// Load 16-bit little-endian: lo byte at [addr], hi byte at [addr+1]
		addr := vm.Get(src0)
		lo := uint64(vm.Memory[addr&0xFFFF])
		hi := uint64(vm.Memory[(addr+1)&0xFFFF])
		vm.Set(dst, lo|(hi<<8))

	default:
		return fmt.Errorf("lir.VM: unhandled op %d", inst.Pat.MIROp)
	}

	return nil
}

// ExecBlock runs all instructions in a block. Returns the terminator.
func (vm *VM) ExecBlock(b *Block) *Term {
	for i := range b.Insts {
		if err := vm.ExecInst(&b.Insts[i]); err != nil {
			panic(err)
		}
	}
	return &b.Term
}

// ExecProg runs a multi-block LIR program to completion.
// Starts at block 0, follows terminators. Returns the return values
// (physical register indices) or error. Safety limit on iterations.
func (vm *VM) ExecProg(prog *Prog) ([]uint64, error) {
	if len(prog.Blocks) == 0 {
		return nil, fmt.Errorf("empty program")
	}

	// Build label → block index map
	blockMap := make(map[string]int, len(prog.Blocks))
	for i, b := range prog.Blocks {
		blockMap[b.Label] = i
	}

	cur := 0
	fuel := 10000 // safety limit
	for fuel > 0 {
		fuel--
		b := &prog.Blocks[cur]

		// Execute all instructions in this block
		for i := range b.Insts {
			if b.Insts[i].Pat == nil {
				continue // skip synthetic param-def cells
			}
			if err := vm.ExecInst(&b.Insts[i]); err != nil {
				return nil, fmt.Errorf("block %s inst %d: %w", b.Label, i, err)
			}
		}

		// Execute terminator
		term := &b.Term
		switch term.Kind {
		case TermNone:
			// Fall through to next block
			cur++
			if cur >= len(prog.Blocks) {
				return nil, nil
			}

		case TermJump:
			if len(term.Targets) == 0 {
				return nil, fmt.Errorf("block %s: jump with no target", b.Label)
			}
			vm.copyBlockArgs(prog, term, 0, blockMap)
			idx, ok := blockMap[term.Targets[0]]
			if !ok {
				return nil, fmt.Errorf("block %s: jump to unknown %s", b.Label, term.Targets[0])
			}
			cur = idx

		case TermBranch:
			if len(term.Targets) < 2 {
				return nil, fmt.Errorf("block %s: branch needs 2 targets", b.Label)
			}
			cond := vm.Get(term.Cond.Phys)
			if cond != 0 {
				vm.copyBlockArgs(prog, term, 0, blockMap)
				cur = blockMap[term.Targets[0]] // then
			} else {
				vm.copyBlockArgs(prog, term, 1, blockMap)
				cur = blockMap[term.Targets[1]] // else
			}

		case TermDJNZ:
			if len(term.Targets) < 2 {
				return nil, fmt.Errorf("block %s: djnz needs 2 targets", b.Label)
			}
			// Decrement counter
			counter := vm.Get(term.Counter.Phys)
			counter--
			vm.Set(term.Counter.Phys, counter)
			if counter != 0 {
				vm.copyBlockArgs(prog, term, 0, blockMap)
				cur = blockMap[term.Targets[0]] // body (loop back)
			} else {
				vm.copyBlockArgs(prog, term, 1, blockMap)
				cur = blockMap[term.Targets[1]] // exit
			}

		case TermReturn:
			var vals []uint64
			for _, op := range term.RetVals {
				vals = append(vals, vm.Get(op.Phys))
			}
			return vals, nil

		default:
			return nil, fmt.Errorf("block %s: unknown terminator kind %d", b.Label, term.Kind)
		}
	}

	return nil, fmt.Errorf("fuel exhausted (infinite loop?)")
}

// copyBlockArgs implements parallel-copy semantics for block argument passing.
// It reads ALL source values first, then writes ALL destinations, so that
// overlapping src/dst registers (e.g. swap) work correctly.
func (vm *VM) copyBlockArgs(prog *Prog, term *Term, edgeIdx int, blockMap map[string]int) {
	if edgeIdx >= len(term.Args) || edgeIdx >= len(term.Targets) {
		return
	}
	args := term.Args[edgeIdx]
	targetLabel := term.Targets[edgeIdx]
	targetIdx, ok := blockMap[targetLabel]
	if !ok {
		return
	}
	targetBlock := &prog.Blocks[targetIdx]
	if len(args) == 0 || len(targetBlock.Params) == 0 {
		return
	}

	// Phase 1: read all source values
	n := len(args)
	if n > len(targetBlock.Params) {
		n = len(targetBlock.Params)
	}
	vals := make([]uint64, n)
	for i := 0; i < n; i++ {
		vals[i] = vm.Get(args[i].Phys)
	}

	// Phase 2: write all destination values
	for i := 0; i < n; i++ {
		vm.Set(targetBlock.Params[i].Phys, vals[i])
	}
}
