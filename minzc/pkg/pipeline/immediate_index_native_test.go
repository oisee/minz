package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Expected addresses follow the 16-bit target's modulo-65536 memory model,
// including a word load whose second byte wraps. A displacement of 65535 is
// ordinary -1 modulo 65536, never the legacy SoA256 low-byte-only encoding.
func TestImmediateIndexNativeWrap(t *testing.T) {
	for _, tc := range []struct {
		index, stride int
		elem          mir2.Ty
	}{
		{0, 1, mir2.TyU8}, {1, 1, mir2.TyU8}, {3, 1, mir2.TyU8},
		{255, 1, mir2.TyU8}, {256, 1, mir2.TyU8}, {65535, 1, mir2.TyU8},
		{257, 3, mir2.TyU8}, {65535, 3, mir2.TyU8},
		{1, 256, mir2.TyU8}, {256, 256, mir2.TyU8},
		{0, 2, mir2.TyU16}, {3, 2, mir2.TyU16}, {32768, 2, mir2.TyU16},
	} {
		for _, base := range []int64{0x60f9, 0xfff9, 0xffff} {
			for mode, opt := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, OptSize: true}, {AssertMode: "none", UseGrace: true}} {
				t.Run(fmt.Sprintf("i%d-stride%d-width%d-base%x-mode%d", tc.index, tc.stride, tc.elem.Width(), base, mode), func(t *testing.T) {
					makeModule := func() *hir.Module {
						return &hir.Module{Name: "literal-index", Target: hir.TargetCPM, Funcs: []*hir.Func{{Name: "read", Params: []hir.Param{{Name: "base", Ty: mir2.TyPtr}}, RetTy: tc.elem,
							Body: hir.Blk(hir.Ret(&hir.IndexExpr{Base: hir.Var("base", mir2.TyPtr), Idx: &hir.IntLitExpr{Val: int64(tc.index), Ty: mir2.TyU16}, ElemTy: tc.elem, ElemStride: tc.stride}))}}}
					}
					address := uint16(base + int64(tc.index)*int64(tc.stride))
					memory := map[uint16]byte{address: 0x5a}
					want := int64(0x5a)
					if tc.elem == mir2.TyU16 {
						memory[address+1] = 0xc7
						want |= 0xc700
					}
					vm := mir2.NewVM(hir.LowerModule(makeModule()))
					vm.EnsureHeap(65536)
					for a, v := range memory {
						vm.WriteHeap(int64(a), v)
					}
					got, err := vm.Call("read", []mir2.Value{{I: base}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("raw VM: got=%v err=%v want=%d", got, err, want)
					}
					measureGraceZ80HIR(t, makeModule(), "read", opt, []int64{base}, memory, want)
				})
			}
		}
	}
}

// A host global can be larger than 64K. Constant indexing must retain its
// 32-bit displacement; it cannot inherit native 16-bit constant folding.
func TestImmediateIndexHostGlobalWide(t *testing.T) {
	h, err := nanz.Parse("global story:[u8;131072]\nfun read()->u8{return story[65536]}", "host-wide-index.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h.Target = hir.TargetMZV
	// resolve the global base through an address-returning function rather than
	// assuming a VM heap layout.
	h.Funcs = append(h.Funcs, &hir.Func{Name: "base", RetTy: mir2.TyPtr, Body: hir.Blk(hir.Ret(&hir.AddrOfExpr{Sym: "story"}))})
	m := hir.LowerModule(h)
	vm := mir2.NewVM(m)
	vm.EnsureHeap(200000)
	base, err := vm.Call("base", nil)
	if err != nil || len(base) != 1 {
		t.Fatal(base, err)
	}
	vm.WriteHeap(base[0].I, 0x12)
	vm.WriteHeap(base[0].I+65536, 0x67)
	got, err := vm.Call("read", nil)
	if err != nil || len(got) != 1 || got[0].I != 0x67 {
		t.Fatal("wide host index", got, err)
	}
}

func TestImmediateIndexGlobalReadWriteNative(t *testing.T) {
	const source = `global data:[u8;512]
fun update(x:u8)->u16 {
 data[255]=x
 data[256]=x+1
 return u16(data[255]) | (u16(data[256])<<8)
}`
	for _, options := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, OptSize: true}, {AssertMode: "none", UseGrace: true}} {
		for _, x := range []int64{0, 1, 127, 128, 255} {
			h, err := nanz.Parse(source, "global-index.nanz")
			if err != nil {
				t.Fatal(err)
			}
			h.Funcs = append(h.Funcs, &hir.Func{Name: "base", RetTy: mir2.TyPtr, Body: hir.Blk(hir.Ret(&hir.AddrOfExpr{Sym: "data"}))})
			vm := mir2.NewVM(hir.LowerModule(h))
			base, err := vm.Call("base", nil)
			if err != nil || len(base) != 1 {
				t.Fatal(base, err)
			}
			want := x | int64(byte(x+1))<<8
			got, err := vm.Call("update", []mir2.Value{{I: x}})
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatal("global VM read/write", got, err, want)
			}
			for i, value := range vm.ReadHeap(base[0].I, 512) {
				expected := byte(0)
				switch i {
				case 255:
					expected = byte(x)
				case 256:
					expected = byte(x + 1)
				}
				if value != expected {
					t.Fatal("global buffer changed", i, value, expected)
				}
			}
			h, err = nanz.Parse(source, "global-index.nanz")
			if err != nil {
				t.Fatal(err)
			}
			measureGraceZ80HIR(t, h, "update", options, []int64{x}, nil, want)
		}
	}
}
