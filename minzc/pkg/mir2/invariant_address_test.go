package mir2

import (
	"reflect"
	"testing"
)

func invariantFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("test")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Reg: 2, Ty: TyU8, Class: ClassCounter}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 3, Imm: 0, Ty: TyU8}}, Term: &TermJmp{Target: "head", Args: []Reg{1, 3}}},
		{Label: "head", Params: []BlockParam{{Dst: 4, Ty: TyPtr}, {Dst: 5, Ty: TyU8}}, Insts: []*Inst{{Op: OpCmp, Dst: 6, Src: [2]Reg{5, 2}, Cond: CmpUlt, Ty: TyBool}}, Term: &TermBrIf{Cond: 6, Then: "body", Else: "exit"}},
		{Label: "body", Insts: []*Inst{{Op: OpConst, Dst: 7, Imm: 1, Ty: TyU16}, {Op: OpPtrAdd, Dst: 8, Src: [2]Reg{4, 7}, Ty: TyPtr}, {Op: OpLoad, Dst: 9, Src: [2]Reg{8}, Ty: TyU8}, {Op: OpConst, Dst: 10, Imm: 1, Ty: TyU8}, {Op: OpAdd, Dst: 11, Src: [2]Reg{5, 10}, Ty: TyU8}}, Term: &TermJmp{Target: "head", Args: []Reg{4, 11}}},
		{Label: "exit", Term: &TermRet{Vals: []Reg{5}}}}
	return m, f
}
func TestInvariantAddressEffects(t *testing.T) {
	for _, mode := range []string{"ordinary", "wrap", "zero", "denied", "mutating-call", "alias-store", "asm", "variant-base", "short-heap"} {
		t.Run(mode, func(t *testing.T) {
			m, f := invariantFixture()
			body := f.BlockByLabel("body")
			if mode == "mutating-call" {
				body.Insts = append(body.Insts[:2], append([]*Inst{{Op: OpCall, Sym: "@mutate", Ty: TyVoid}}, body.Insts[2:]...)...)
			}
			if mode == "alias-store" {
				body.Insts = append(body.Insts[:2], append([]*Inst{{Op: OpStore, Src: [2]Reg{8, 5}, Ty: TyU8}}, body.Insts[2:]...)...)
			}
			if mode == "asm" {
				body.Insts = append(body.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
			}
			if mode == "short-heap" {
				body.Insts = append(body.Insts[:3], append([]*Inst{{Op: OpConst, Dst: 12, Imm: 1, Ty: TyU16}, {Op: OpPtrAdd, Dst: 13, Src: [2]Reg{8, 12}, Ty: TyPtr}, {Op: OpLoad, Dst: 14, Src: [2]Reg{13}, Ty: TyU8}}, body.Insts[3:]...)...)
			}
			if mode == "variant-base" {
				body.Term.(*TermJmp).Args[0] = 8
			}
			n := int64(3)
			base := int64(0x4000)
			if mode == "zero" {
				n = 0
			}
			if mode == "wrap" {
				base = 65535
			}
			if mode == "short-heap" {
				base = 3
			}
			type observation struct {
				Values []Value
				Events []VMDataAccess
				Denied []int64
				RAM    []byte
				Error  bool
			}
			run := func() observation {
				vm := NewVM(m)
				if mode == "short-heap" {
					vm.EnsureHeap(5)
				} else {
					vm.EnsureHeap(65536)
				}
				vm.WriteHeap(base+1&65535, 7)
				vm.Hosts["@mutate"] = func([]Value) ([]Value, error) { vm.WriteHeap(base+1&65535, 42); return nil, nil }
				o := observation{}
				vm.ObserveDataAccess = func(a VMDataAccess) { o.Events = append(o.Events, a) }
				vm.CheckDataAccess = func(i *Inst, a int64) error {
					if i.Op != OpLoad && i.Op != OpStore {
						t.Fatal("changed callback opcode")
					}
					if i.Ty != TyU8 {
						t.Fatal("changed callback byte width")
					}
					if mode == "denied" {
						o.Denied = append(o.Denied, a)
						return &MemoryFaultForTest{}
					}
					return nil
				}
				v, e := vm.Call("test", []Value{{I: base}, {I: n}})
				o.Values = v
				o.Error = e != nil
				if mode == "short-heap" {
					o.RAM = vm.ReadHeap(0, 5)
				} else {
					o.RAM = vm.ReadHeap(0, 65536)
				}
				return o
			}
			before := run()
			did := HoistInvariantAddresses(f)
			if (mode == "asm" || mode == "variant-base" || mode == "mutating-call") && did {
				t.Fatal("unsafe loop admitted")
			}
			if mode != "asm" && mode != "variant-base" && mode != "mutating-call" && !did {
				t.Fatal("invariant address not hoisted")
			}
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			after := run()
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("observable effect changed mode %s", mode)
			}
			if mode == "short-heap" && (!after.Error || len(after.Events) != 1 || after.Events[0].Addr != 4) {
				t.Fatal("low-success/high-trap prefix changed")
			}
			if n == 0 && len(after.Events) > 0 {
				t.Fatal("zero trip touched memory")
			}
		})
	}
}

type MemoryFaultForTest struct{}

func (*MemoryFaultForTest) Error() string { return "denied byte" }
