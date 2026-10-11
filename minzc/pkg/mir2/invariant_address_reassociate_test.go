package mir2

import (
	"reflect"
	"testing"
)

// Addition follows the modulo-65536 pointer contract in docs/adr/0045-raw-pointer-semantics.md.
// A changed DAG must retain every byte access, callback and fault prefix.
func TestInvariantAddressReassociationEffects(t *testing.T) {
	for _, mode := range []string{"ordinary", "global", "wrap", "zero", "alias-store", "denied", "short-heap", "extra-use", "unknown-global", "variant-offset", "hard-sum", "wide-sum", "mutating-call"} {
		t.Run(mode, func(t *testing.T) {
			m, f := invariantFixture()
			m.Globals = []Global{{Name: "buffer", Ty: TyU8}}
			f.Contract.Params = append(f.Contract.Params, Param{Reg: 15, Ty: TyU16, Class: ClassGeneral})
			body := f.BlockByLabel("body")
			base := &Inst{Op: OpMove, Dst: 13, Src: [2]Reg{4}, Ty: TyPtr}
			if mode == "global" || mode == "unknown-global" {
				base.Op = OpAddrOf
				base.Src = [2]Reg{}
				base.Sym = "buffer"
				if mode == "unknown-global" {
					base.Sym = "missing"
				}
			}
			// Base comes after the offset sum, as in FAT12's lowered directory copy.
			prefix := []*Inst{{Op: OpExt, Dst: 7, Src: [2]Reg{5}, Ty: TyU16}, {Op: OpAdd, Dst: 12, Src: [2]Reg{15, 7}, Ty: TyU16}, base, {Op: OpPtrAdd, Dst: 8, Src: [2]Reg{13, 12}, Ty: TyPtr}}
			body.Insts = append(prefix, body.Insts[2:]...)
			if mode == "hard-sum" {
				body.Insts[1].ClsHard = true
			}
			if mode == "wide-sum" {
				body.Insts[1].Ty = TyU32
			}
			if mode == "mutating-call" {
				body.Insts = append(body.Insts[:4], append([]*Inst{{Op: OpCall, Sym: "@mutate", Ty: TyVoid}}, body.Insts[4:]...)...)
			}
			if mode == "extra-use" {
				body.Insts = append(body.Insts, &Inst{Op: OpMove, Dst: 16, Src: [2]Reg{12}, Ty: TyU16})
			}
			if mode == "alias-store" {
				body.Insts = append(body.Insts[:4], append([]*Inst{{Op: OpStore, Src: [2]Reg{8, 5}, Ty: TyU8}}, body.Insts[4:]...)...)
			}
			if mode == "variant-offset" {
				head := f.BlockByLabel("head")
				head.Params = append(head.Params, BlockParam{Dst: 17, Ty: TyU16})
				f.BlockByLabel("entry").Term.(*TermJmp).Args = append(f.BlockByLabel("entry").Term.(*TermJmp).Args, 15)
				body.Term.(*TermJmp).Args = append(body.Term.(*TermJmp).Args, 12)
				body.Insts[1].Src[0] = 17
			}
			n, addr, ofs := int64(3), int64(0x4000), int64(11)
			if mode == "wrap" {
				addr = 65535
				ofs = 0
			}
			if mode == "global" {
				addr = 0
			}
			if mode == "zero" || mode == "unknown-global" {
				n = 0
			}
			if mode == "short-heap" {
				addr = 3
				ofs = 1
			}
			type observation struct {
				Values []Value
				Events []VMDataAccess
				Denied []int64
				Memory []byte
				Error  string
			}
			run := func() observation {
				vm := NewVM(m)
				limit := 65536
				if mode == "short-heap" {
					limit = 5
				}
				vm.EnsureHeap(int64(limit))
				if mode == "short-heap" {
					vm.heap = vm.heap[:limit]
				}
				for p := 0; p < limit; p++ {
					vm.WriteHeap(int64(p), byte(p*37+11))
				}
				o := observation{}
				vm.Hosts["@mutate"] = func([]Value) ([]Value, error) { vm.WriteHeap((addr+ofs)&65535, 42); return nil, nil }
				vm.ObserveDataAccess = func(a VMDataAccess) { o.Events = append(o.Events, a) }
				vm.CheckDataAccess = func(i *Inst, a int64) error {
					if i.Ty != TyU8 || (i.Op != OpLoad && i.Op != OpStore) {
						t.Fatal("changed byte callback")
					}
					if mode == "denied" {
						o.Denied = append(o.Denied, a)
						return &MemoryFaultForTest{}
					}
					return nil
				}
				var e error
				o.Values, e = vm.Call("test", []Value{{I: addr}, {I: n}, {I: ofs}})
				if e != nil {
					o.Error = e.Error()
				}
				o.Memory = vm.ReadHeap(0, limit)
				return o
			}
			before := run()
			did := HoistInvariantAddresses(f)
			rejected := mode == "extra-use" || mode == "unknown-global" || mode == "variant-offset" || mode == "hard-sum" || mode == "wide-sum" || mode == "mutating-call"
			if did == rejected {
				t.Fatalf("changed=%v rejected=%v", did, rejected)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			after := run()
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("observable address effect changed: before%+v after%+v", before.Events, after.Events)
			}
			if !rejected {
				for _, i := range body.Insts {
					if i.Dst == 12 {
						t.Fatal("invariant address sum remained inside loop")
					}
				}
			}
			if mode == "ordinary" || mode == "wrap" || mode == "global" {
				if len(after.Events) != int(n) {
					t.Fatal("load count")
				}
				for i, event := range after.Events {
					if event.Op != OpLoad || event.Addr != (addr+ofs+int64(i))&65535 {
						t.Fatal("independent modular oracle", event)
					}
				}
			}
			if n == 0 && len(after.Events) != 0 {
				t.Fatal("zero trip touched memory")
			}
			if mode == "short-heap" && (after.Error == "" || len(after.Events) != 1 || after.Events[0].Addr != 4) {
				t.Fatal("successful byte/fault prefix")
			}
		})
	}
}
