package mir2

import "fmt"

func (vm *VM) execByteLoop(fr *frame, x *Inst) error {
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
