package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestFlat24PointerMemory(t *testing.T) {
	const source = `struct View { data:^u8 len:u16 marker:u8 }
global view:View
global words:[u16;40000]
fun cross(base:^u8,index:u16)->u8{return base[index]}
fun word(index:u16)->u16{return words[index]}
fun save(base:^u8)->u8{view.data=base
view.len=4660
view.marker=91
return view.data[1]}
fun address()->^u8{return &view.marker}
fun stride(base:^u16,index:u16)->u16{return base[index]}`
	for _, bits := range []int{16, 24} {
		for _, optimized := range []bool{false, true} {
			h, err := nanz.ParseWithOpts(source, "flat24.nanz", nanz.ParseOpts{PointerBits: bits})
			if err != nil {
				t.Fatal(err)
			}
			h.Target = hir.TargetAgon
			h.NonNullObjects = true
			st := h.Structs[0]
			if got := mir2.ByteWidth(st); got != bits/8+3 {
				t.Fatalf("bits=%d struct size=%d", bits, got)
			}
			m := hir.LowerModule(h)
			if err := mir2.Verify(m); err != nil {
				t.Fatal(err)
			}
			if optimized {
				for _, f := range m.Funcs {
					mir2.PropagateCopies(f)
					mir2.DeadStoreElim(f)
				}
			}
			vm := mir2.NewVM(m)
			vm.MaxMemory = 2 << 20
			vm.EnsureHeap(1 << 20)
			vm.WriteHeap(0, 17)
			vm.WriteHeap(65536, 29)
			call := func(name string, args ...int64) int64 {
				t.Helper()
				vs := make([]mir2.Value, len(args))
				for i, v := range args {
					vs[i].I = v
				}
				out, err := vm.Call(name, vs)
				if err != nil || len(out) != 1 {
					t.Fatalf("bits=%d opt=%v %s: %v %v", bits, optimized, name, out, err)
				}
				return out[0].I
			}
			want := int64(17)
			if bits == 24 {
				want = 29
			}
			if got := call("cross", 65535, 1); got != want {
				t.Fatalf("bits=%d opt=%v cross=%d want=%d", bits, optimized, got, want)
			}
			saveWant := want
			if got := call("save", 65535); got != saveWant {
				t.Fatalf("bits=%d save=%d want=%d", bits, got, saveWant)
			}
			addr := vm.GlobalAddr("view")
			raw := vm.ReadHeap(addr, bits/8+3)
			if raw[bits/8] != 0x34 || raw[bits/8+1] != 0x12 || raw[bits/8+2] != 91 {
				t.Fatalf("neighbor fields corrupted: %x", raw)
			}
			if got := call("address"); got != int64(addr)+int64(bits/8+2) {
				t.Fatalf("field address=%d", got)
			}
			if bits == 24 {
				if got := call("save", 0x4ffff); got != 0 {
					t.Fatalf("high pointer read=%d", got)
				}
				raw = vm.ReadHeap(addr, 3)
				if raw[0] != 255 || raw[1] != 255 || raw[2] != 4 {
					t.Fatalf("pointer truncated: %x", raw)
				}
				vm.WriteHeap(0x50000, 0x78)
				vm.WriteHeap(0x50001, 0x56)
				if got := call("stride", 0x4fffe, 1); got != 0x5678 {
					t.Fatalf("typed stride=%x", got)
				}
				words := vm.GlobalAddr("words")
				vm.WriteHeap(words+70000, 0xcd)
				vm.WriteHeap(words+70001, 0xab)
				if got := call("word", 35000); got != 0xabcd {
					t.Fatalf("array stride narrowed=%x", got)
				}
			}
		}
	}
}

func TestFlat24HighStructAndLocalCell(t *testing.T) {
	const source = `struct View{data:^u8 len:u16 marker:u8}
global padding:[u8;65536]
global view:View
fun save(base:^u8)->u8{view.data=base
view.len=4660
view.marker=91
return view.data[1]}
fun address()->^u8{return &view.marker}
fun local(base:^u8)->u8{var temp:View
temp.data=base
temp.len=7
temp.marker=92
return temp.data[1]}`
	for _, optimized := range []bool{false, true} {
		h, err := nanz.ParseWithOpts(source, "high-cell.nanz", nanz.ParseOpts{PointerBits: 24})
		if err != nil {
			t.Fatal(err)
		}
		h.Target = hir.TargetAgon
		m := hir.LowerModule(h)
		if err := mir2.Verify(m); err != nil {
			t.Fatal(err)
		}
		if _, err := mir2.Z80Codegen(m, &mir2.AllocResult{}); err == nil {
			t.Fatal("flat24 admitted to ordinary Z80")
		}
		if optimized {
			for _, f := range m.Funcs {
				mir2.PropagateCopies(f)
				mir2.DeadStoreElim(f)
			}
		}
		vm := mir2.NewVM(m)
		vm.MaxMemory = 2 << 20
		vm.EnsureHeap(1 << 20)
		vm.WriteHeap(0x50000, 83)
		for _, fn := range []string{"save", "local"} {
			got, err := vm.Call(fn, []mir2.Value{{I: 0x4ffff}})
			if err != nil || len(got) != 1 || got[0].I != 83 {
				t.Fatalf("opt=%v %s: %v %v", optimized, fn, got, err)
			}
		}
		if got := vm.GlobalAddr("view"); got != 65536 {
			t.Fatalf("high struct address=%d", got)
		}
		raw := vm.ReadHeap(65536, 6)
		want := []byte{255, 255, 4, 0x34, 0x12, 91}
		for i, v := range want {
			if raw[i] != v {
				t.Fatalf("opt=%v high struct: %x", optimized, raw)
			}
		}
		got, err := vm.Call("address", nil)
		if err != nil || len(got) != 1 || got[0].I != 65541 {
			t.Fatalf("high field address: %v %v", got, err)
		}
	}
}
