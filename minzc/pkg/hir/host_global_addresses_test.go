package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestHostGlobalAddressesDoNotNarrowLargeArrays(t *testing.T) {
	const source = `global story:[u8;131072]
global words:[u16;40000]
global result:u8
fun read_byte(index:u32)->u8{return story[index]}
fun read_word(index:u32)->u16{return words[index]}
fun write_byte(index:u32,value:u8)->void{story[index]=value
result=value}
fun narrow_global()->u16{return u16(&result)}
fun literal_wrap()->u8{return u8(ptr(65535)[1])}`
	h, err := nanz.Parse(source, "host-globals.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h.Target = hir.TargetMZV
	m := hir.LowerModule(h)
	if _, err := mir2.Z80Codegen(m, &mir2.AllocResult{}); err == nil {
		t.Fatal("host-sized allocation accepted by Z80")
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, optimized := range []bool{false, true} {
		if optimized {
			for _, f := range m.Funcs {
				mir2.PropagateCopies(f)
				mir2.DeadStoreElim(f)
			}
		}
		vm := mir2.NewVM(m)
		vm.EnsureHeap(300000)
		story := vm.GlobalAddr("story")
		words := vm.GlobalAddr("words")
		result := vm.GlobalAddr("result")
		vm.WriteHeap(story+0x1fe00, 72)
		vm.WriteHeap(words+70000, 0x34)
		vm.WriteHeap(words+70001, 0x12)
		got, err := vm.Call("read_byte", []mir2.Value{{I: 0x1fe00}})
		if err != nil || len(got) != 1 || got[0].I != 72 {
			t.Fatalf("optimized=%v high byte %v %v", optimized, got, err)
		}
		got, err = vm.Call("read_word", []mir2.Value{{I: 35000}})
		if err != nil || len(got) != 1 || got[0].I != 0x1234 {
			t.Fatalf("high stride %v %v", got, err)
		}
		if _, err := vm.Call("write_byte", []mir2.Value{{I: 0x1ffff}, {I: 91}}); err != nil {
			t.Fatal(err)
		}
		if vm.ReadHeap(story+0x1ffff, 1)[0] != 91 || vm.ReadHeap(result, 1)[0] != 91 {
			t.Fatalf("high array/scalar store narrowed: story=%d result=%d address=%d", vm.ReadHeap(story+0x1ffff, 1)[0], vm.ReadHeap(result, 1)[0], result)
		}
		got, err = vm.Call("narrow_global", nil)
		if err != nil || len(got) != 1 || got[0].I != int64(result&65535) {
			t.Fatalf("explicit global pointer did not narrow %v %v", got, err)
		}
		vm.WriteHeap(0, 57)
		got, err = vm.Call("literal_wrap", nil)
		if err != nil || len(got) != 1 || got[0].I != 57 {
			t.Fatalf("explicit16 pointer stopped wrapping %v %v", got, err)
		}
	}
}
