package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"testing"
)

func TestTruncMaskKeepsByteStoreIdentityAndFaultPrefix(t *testing.T) {
	const source = `fun write(p:^u8,v:u16)->void {
 let lo:u8=v&255
 let shift:u16=v>>8
 let hi:u8=shift&255
 p[0]=lo
 p[1]=hi
 }`
	for _, mode := range []string{"ordinary", "wrap", "high-denied"} {
		h, e := nanz.Parse(source, "byte_store.nanz")
		if e != nil {
			t.Fatal(e)
		}
		m := hir.LowerModule(h)
		f := m.FuncByName("write")
		var stores []*mir2.Inst
		for _, i := range f.Entry().Insts {
			if i.Op == mir2.OpStore {
				stores = append(stores, i)
			}
		}
		if len(stores) != 2 {
			t.Fatal("two original byte stores required")
		}
		originals := []mir2.Inst{*stores[0], *stores[1]}
		type obs struct {
			Events []mir2.VMDataAccess
			Denied []int64
			RAM    []byte
			Fault  bool
		}
		addr := int64(0x4000)
		if mode == "wrap" {
			addr = 65535
		}

		run := func() obs {
			vm := mir2.NewVM(m)
			limit := int64(65536)
			vm.MaxMemory = int(limit)
			vm.EnsureHeap(limit)
			vm.WriteHeap(addr, 0xaa)
			o := obs{}
			vm.ObserveDataAccess = func(a mir2.VMDataAccess) { o.Events = append(o.Events, a) }
			vm.CheckDataAccess = func(i *mir2.Inst, a int64) error {
				if i.Ty != mir2.TyU8 || i.Op != mir2.OpStore || i != stores[0] && i != stores[1] {
					t.Fatal("original byte Inst changed")
				}
				if mode == "high-denied" && i == stores[1] {
					o.Denied = append(o.Denied, a)
					return &fusionAuditDenied{}
				}
				return nil
			}
			_, e := vm.Call("write", []mir2.Value{{I: addr}, {I: 0x1234}})
			o.Fault = e != nil
			o.RAM = vm.ReadHeap(0, int(limit))
			return o
		}
		before := run()
		if !mir2.SimplifyIdentities(f) {
			t.Fatal("identity not applied")
		}
		mir2.DeadStoreElim(f)
		for i, s := range stores {
			if !reflect.DeepEqual(*s, originals[i]) {
				t.Fatal("original OpStore metadata/arguments changed")
			}
		}
		after := run()
		if !reflect.DeepEqual(before, after) {
			t.Fatal("byte effects/fault prefix changed", mode, before.Events, after.Events)
		}
		if mode == "high-denied" {
			if !after.Fault || len(after.Events) != 1 || after.Events[0].Value != 0x34 || after.Events[0].Addr != addr {
				t.Fatal("low-write/high-fault prefix", mode, after.Fault, after.Events)
			}
		}
	}
}
