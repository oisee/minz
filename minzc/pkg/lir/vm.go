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
		vm.Set(dst, vm.Get(src0)+vm.Get(src1))

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
		w := vm.Desc.Locs[dst].Width
		var val uint64
		for i := 0; i < w/8; i++ {
			val |= uint64(vm.Memory[(addr+uint64(i))&0xFFFF]) << (i * 8)
		}
		vm.Set(dst, val)

	case OpStore:
		addr := vm.Get(src0)
		val := vm.Get(src1)
		w := vm.Desc.Locs[src1].Width
		for i := 0; i < w/8; i++ {
			vm.Memory[(addr+uint64(i))&0xFFFF] = byte(val >> (i * 8))
		}

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
