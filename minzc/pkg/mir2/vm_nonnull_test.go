package mir2

import "testing"

func TestVMAllocationsDoNotUseNullAddress(t *testing.T) {
	for _, globals := range []bool{false, true} {
		m := &Module{NonNullObjects: true}
		if globals {
			m.AddGlobal(Global{Name: "first", Ty: TyU8})
		}
		m.AddFunc("first_function")
		vm := NewVM(m)
		if globals && vm.GlobalAddr("first") <= 0 {
			t.Fatal("global aliases null")
		}
		if vm.GlobalAddr("first_function") <= 0 {
			t.Fatal("function aliases null")
		}
		if vm.AllocHeap([]byte{7}).I <= 0 {
			t.Fatal("heap allocation aliases null")
		}
		vm.WriteHeap(0, 99)
		if vm.ReadHeap(0, 1)[0] != 99 {
			t.Fatal("explicit address zero access changed")
		}
	}
}

func TestVMRawAddressGlobalLayoutUnchanged(t *testing.T) {
	m := &Module{Globals: []Global{{Name: "raw", Ty: TyU8}}}
	if NewVM(m).GlobalAddr("raw") != 0 {
		t.Fatal("raw-address layout changed")
	}
}
