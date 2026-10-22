package mir2

import "fmt"

func (vm *VM) execByteLoop(fr *frame, x *Inst) error {
	if x.Sym == copyBytesIntrinsic {
		return vm.execCopyBytes(fr, x)
	}
	if x.Sym == fillBytesIntrinsic {
		return vm.execFillBytes(fr, x)
	}
	if x.Sym != scanZeroIntrinsic || x.ByteLoop == nil || x.ByteLoop.Load == nil || len(x.Args) != 1 {
		return fmt.Errorf("invalid byte scan")
	}
	base := fr.get(x.Args[0]).I
	pointer := base
	length := int64(0)
	for {
		vm.steps++
		if vm.MaxSteps > 0 && vm.steps > vm.MaxSteps {
			return fmt.Errorf("byte scan: step limit exceeded")
		}
		if !vm.wrappedHeapRangeValid(pointer, 1, 65535) {
			return fmt.Errorf("load: out-of-bounds address %d", pointer)
		}
		if vm.CheckDataAccess != nil {
			if e := vm.CheckDataAccess(x.ByteLoop.Load, pointer); e != nil {
				return e
			}
		}
		v := vm.heap[int(pointer)]
		if vm.ObserveDataAccess != nil {
			vm.ObserveDataAccess(VMDataAccess{Op: OpLoad, Addr: pointer, Value: v})
		}
		if v == 0 {
			break
		}
		length = (length + 1) & 65535
		pointer = (pointer + 1) & 65535
	}
	if x.Ty == TyPtr {
		fr.set(x.Dst, Value{I: pointer})
	} else {
		fr.set(x.Dst, Value{I: length})
	}
	return nil
}

// Each byte is authorized and committed separately, retaining the successful
// prefix on faults. Symbol reloads preserve the original header/body traffic.
func (vm *VM) byteLoopAccess(x *Inst, addr int64, value byte) (byte, error) {
	if !vm.wrappedHeapRangeValid(addr, 1, 65535) {
		return 0, fmt.Errorf("byte loop: out-of-bounds address %d", addr)
	}
	if vm.CheckDataAccess != nil {
		if e := vm.CheckDataAccess(x, addr); e != nil {
			return 0, e
		}
	}
	if x.Op == OpLoad {
		value = vm.heap[int(addr)]
	} else {
		vm.heap[int(addr)] = value
	}
	if vm.ObserveDataAccess != nil {
		vm.ObserveDataAccess(VMDataAccess{Op: x.Op, Addr: addr, Value: value})
	}
	return value, nil
}
func (vm *VM) execFillBytes(fr *frame, x *Inst) error {
	e := x.ByteLoop
	dst := fr.get(x.Args[0]).I
	count := fr.get(x.Args[1]).I
	value := byte(fr.get(x.Args[2]).I)
	for i := int64(0); ; i++ {
		if e.CountLoad != nil {
			v, err := vm.byteLoopAccess(e.CountLoad, vm.GlobalAddr(e.CountSymbol), 0)
			if err != nil {
				return err
			}
			count = int64(v)
		}
		if i >= count {
			return nil
		}
		if e.ValueLoad != nil {
			v, err := vm.byteLoopAccess(e.ValueLoad, vm.GlobalAddr(e.ValueSymbol), 0)
			if err != nil {
				return err
			}
			value = v
		}
		if _, err := vm.byteLoopAccess(e.Store, (dst+i)&65535, value); err != nil {
			return err
		}
	}
}

// Forward copy deliberately reads and writes one byte at a time: overlapping
// dst=src+1 propagates the first byte. Go copy would incorrectly use memmove.
func (vm *VM) execCopyBytes(fr *frame, x *Inst) error {
	dst, src, n := fr.get(x.Args[0]).I, fr.get(x.Args[2]).I, fr.get(x.Args[1]).I
	for i := int64(0); i < n; i++ {
		v, err := vm.byteLoopAccess(x.ByteLoop.Load, (src+i)&65535, 0)
		if err != nil {
			return err
		}
		if _, err = vm.byteLoopAccess(x.ByteLoop.Store, (dst+i)&65535, v); err != nil {
			return err
		}
	}
	return nil
}
