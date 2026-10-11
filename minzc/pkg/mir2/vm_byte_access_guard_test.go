package mir2

import (
	"fmt"
	"reflect"
	"testing"
)

func TestVMByteAccessGuardPartialPrefix(t *testing.T) {
	for _, op := range []Op{OpLoad, OpStore} {
		for _, reject := range []int64{0x8000, 0x8001} {
			t.Run(fmt.Sprintf("%s/%d", op, reject), func(t *testing.T) {
				f := &Func{Name: "access", Contract: Contract{Returns: []Return{{Ty: TyU16}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Imm: 0x8000, Ty: TyPtr}, {Op: OpConst, Dst: 2, Imm: 0x1234, Ty: TyU16}}, Term: &TermRet{Vals: []Reg{2}}}}}
				if op == OpLoad {
					f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: 3, Src: [2]Reg{1}, Ty: TyU16})
					f.Blocks[0].Term = &TermRet{Vals: []Reg{3}}
				} else {
					f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Src: [2]Reg{1, 2}, Ty: TyU16})
				}
				vm := NewVM(&Module{Funcs: []*Func{f}})
				vm.EnsureHeap(65536)
				vm.WriteHeapBytes(0x8000, []byte{0x34, 0xaa})
				got := []VMDataAccess{}
				vm.CheckDataAccess = func(inst *Inst, a int64) error {
					if a == reject {
						return fmt.Errorf("denied")
					}
					return nil
				}
				vm.ObserveDataAccess = func(a VMDataAccess) { got = append(got, a) }
				if _, err := vm.Call("access", nil); err == nil {
					t.Fatal("accepted denied byte")
				}
				want := []VMDataAccess{}
				if reject == 0x8001 {
					want = append(want, VMDataAccess{Op: op, Addr: 0x8000, Value: 0x34})
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
				}
				if !reflect.DeepEqual(vm.ReadHeap(0x8000, 2), []byte{0x34, 0xaa}) {
					t.Fatal("invalid byte mutated")
				}
				vm.CheckDataAccess = nil
				got = nil
				if _, err := vm.Call("access", nil); err != nil {
					t.Fatal(err)
				}
				if len(got) != 2 {
					t.Fatal(got)
				}
			})
		}
	}
}
