package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestOptSizeByteFillVMZ80(t *testing.T) {
	source := byteLoopNanzGlobals + `fun fill(p:ptr,n:u8,v:u8) { var i:u8=0
 while i<n { p[i]=v
 i=i+1 } }
 fun execute() { fill(&buffer[0],u8(n),value)
 result=n }`
	cases := []byteLoopCase{}
	for _, n := range []int{0, 1, 2, 127, 128, 255} {
		initial := make([]byte, 4096)
		want := make([]byte, 4096)
		for i := range initial {
			initial[i] = byte(i*37 + 11)
			want[i] = initial[i]
		}
		for i := 0; i < n; i++ {
			want[i] = 0xa7
		}
		cases = append(cases, byteLoopCase{initial: initial, want: want, n: n, value: 0xa7, result: n})
	}
	byteLoopNative(t, "nanz", "default", source, "LDIR", cases)
	byteLoopNative(t, "nanz", "size", source, "BYTE_FILL_SCALAR", cases)
}

func TestOptSizePointerScanVMZ80(t *testing.T) {
	source := byteLoopCGlobals + `unsigned char *walk(unsigned char *p){while(*p!=0)p++;return p;}
 void execute(void){result=walk(buffer+src_off)-buffer;}`
	cases := []byteLoopCase{}
	for _, n := range []int{0, 1, 2, 127, 255, 256, 1023} {
		initial := make([]byte, 4096)
		for i := range initial {
			initial[i] = byte(i%255 + 1)
		}
		initial[n] = 0
		cases = append(cases, byteLoopCase{initial: initial, want: append([]byte{}, initial...), result: n})
	}
	byteLoopNative(t, "c", "default", source, "CPIR", cases)
	byteLoopNative(t, "c", "size", source, "BYTE_SCAN_SCALAR", cases)
}

// Compare both modes with raw MIR and native execution for each search area.
func TestOptSizeCandidateProductsVMZ80(t *testing.T) {
	fixtures := []struct{ name, source string }{
		{"inline", `global a:u8 global b:u8
 fun helper()->u8 {return (a+1)&(b+1)}
 fun probe(x:u8,y:u8)->u8 {a=x b=y return helper()+1}`},
		{"affine", `fun probe(p:ptr,n:u8)->u8 {var i:u8=0 var sum:u8=0
 while i<n {sum=sum+p[i+1] i=i+1} return sum}`},
		{"carry", `fun probe(x:u8)->u8 {let shifted:u8=x>>1
 if (x&1)!=0 {return shifted^7} else {return shifted}}`},
		{"copies", variantWinningNest},
		{"allocation", `fun probe(x:u8,y:u8)->u8 {return ((x+1)&(y+3))^((x+7)|(y+9))}`},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			h, e := nanz.Parse(tc.source, "size.nanz")
			if e != nil {
				t.Fatal(e)
			}
			for _, size := range []bool{false, true} {
				o := DefaultOptions()
				o.OptSize = size
				o.AssertMode = "none"
				s, e := CompileHIRSteps(hir.CloneModule(h), o)
				if e != nil {
					t.Fatal(e)
				}
				im, e := assembleVariant(s)
				if e != nil {
					t.Fatal(e)
				}
				t.Logf("size=%v bytes=%d search=%+v decisions=%+v", size, len(im.binary), s.VariantSearch, s.VariantDecisions)
				if size && !strings.Contains(s.Assembly, "z80-encoded-bytes-v1") {
					t.Error("size objective missing")
				}
				raw := hir.LowerModule(hir.CloneModule(h))
				for _, args := range [][]int64{{0, 0}, {1, 2}, {127, 128}, {255, 255}} {
					f := raw.FuncByName("probe")
					n := len(f.Contract.Params)
					av := []mir2.Value{}
					memory := map[uint16]byte{}
					for i := 0; i < n; i++ {
						av = append(av, mir2.Value{I: args[i]})
					}
					if tc.name == "affine" {
						av[0] = mir2.Value{I: 0xc100}
						av[1] = mir2.Value{I: args[1]}
						for i := 0; i < 300; i++ {
							memory[0xc100+uint16(i)] = byte(i*13 + 7)
						}
					}
					vm := mir2.NewVM(raw)
					vm.EnsureHeap(65536)
					for p, v := range memory {
						vm.WriteHeapSlice(int64(p), []byte{v})
					}
					want, e := vm.Call("probe", av)
					if e != nil || len(want) != 1 {
						t.Fatal(want, e)
					}
					vm = mir2.NewVM(s.MIR2Module)
					vm.EnsureHeap(65536)
					for p, v := range memory {
						vm.WriteHeapSlice(int64(p), []byte{v})
					}
					got, e := vm.Call("probe", av)
					if e != nil || len(got) != 1 || got[0].I != want[0].I {
						t.Fatal(got, e, want)
					}
					native := []int64{}
					for _, v := range av {
						native = append(native, v.I)
					}
					measureGraceZ80Compiled(t, h, s, "probe", native, memory, want[0].I, 2000000)
				}
			}
		})
	}
}

func TestOptSizeConstantShiftVMZ80(t *testing.T) {
	for _, tc := range []struct {
		ty, op string
		signed bool
	}{{"u16", ">>", false}, {"u16", "<<", false}, {"i16", ">>", true}, {"u8", ">>", false}} {
		t.Run(tc.ty+tc.op, func(t *testing.T) {
			h, e := nanz.Parse("fun probe(x:"+tc.ty+")->"+tc.ty+" {return x "+tc.op+" 4}", "shift_size.nanz")
			if e != nil {
				t.Fatal(e)
			}
			var sizes [2]int
			for i, size := range []bool{false, true} {
				o := DefaultOptions()
				o.OptSize = size
				o.AssertMode = "none"
				s, e := CompileHIRSteps(hir.CloneModule(h), o)
				if e != nil {
					t.Fatal(e)
				}
				im, e := assembleVariant(s)
				if e != nil {
					t.Fatal(e)
				}
				sizes[i] = len(im.binary)
				t.Logf("size%v bytes%d decisions%+v search%+v\n%s", size, len(im.binary), s.VariantDecisions, s.VariantSearch, s.Assembly)
				if size && tc.op != "<<" && !strings.Contains(s.Assembly, ".shift_size_") {
					t.Error("size mode did not reroll constant shift")
				}
				if !size && strings.Contains(s.Assembly, ".shift_size_") {
					t.Error("speed mode rerolled constant shift")
				}
				for _, x := range []int64{0, 1, 15, 16, 127, 128, 255, 256, 0x7fff, 0x8000, 0xffff} {
					width := 16
					if tc.ty == "u8" {
						width = 8
					}
					if width == 8 && x > 255 {
						continue
					}
					want := (x << 4) & ((1 << width) - 1)
					if tc.op == ">>" {
						want = x >> 4
						if tc.signed {
							want = int64(int16(x) >> 4)
						}
					}
					got, e := mir2.NewVM(s.MIR2Module).Call("probe", []mir2.Value{{I: x}})
					mask := int64((1 << width) - 1)
					if e != nil || len(got) != 1 || got[0].I&mask != want&mask {
						t.Fatal(got, e, want)
					}
					measureGraceZ80Compiled(t, h, s, "probe", []int64{x}, nil, want, 200000)
				}
			}
			if tc.op != "<<" && sizes[1] >= sizes[0] || tc.op == "<<" && sizes[1] != sizes[0] {
				t.Errorf("compact shift bytes %v", sizes)
			}
		})
	}
}

func TestOptSizeAllocationVMZ80(t *testing.T) {
	h := graceMeasureParse(t, `global lhs:u8 global rhs:u8
 fun accumulate()->u16 {var weight:u16=u16(lhs) var bits:u8=rhs var total:u16=0
 while bits!=0 {if (bits&1)!=0 {total=total+weight}
 weight=weight<<1 bits=bits>>1} return total}
 fun probe(x:u8,y:u8)->u16 {lhs=x rhs=y return accumulate()}`)
	var sizes [2]int
	for i, size := range []bool{false, true} {
		o := DefaultOptions()
		o.OptSize = size
		o.AssertMode = "none"
		s, e := CompileHIRSteps(hir.CloneModule(h), o)
		if e != nil {
			t.Fatal(e)
		}
		im, e := assembleVariant(s)
		if e != nil {
			t.Fatal(e)
		}
		sizes[i] = len(im.binary)
		t.Logf("size%v bytes%d candidates%+v", size, len(im.binary), s.VariantDecisions)
		if size && !strings.Contains(s.Assembly, "size-alloc=true") {
			t.Error("size allocation was not selected")
		}
		for _, x := range []int64{0, 1, 2, 127, 128, 255} {
			for _, y := range []int64{0, 1, 2, 127, 128, 255} {
				vm := mir2.NewVM(s.MIR2Module)
				got, e := vm.Call("probe", []mir2.Value{{I: x}, {I: y}})
				if e != nil || len(got) != 1 || got[0].I != x*y {
					t.Fatal(got, e, x*y)
				}
				measureGraceZ80Compiled(t, h, s, "probe", []int64{x, y}, nil, x*y, 200000)
			}
		}
	}
	if sizes[1] >= sizes[0] {
		t.Errorf("size allocation grew/tied: %v", sizes)
	}
}

func TestOptSizeSearchBudgetVMZ80(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&source, "fun work%d(x:u16)->u16 {return x>>4}\n", i)
	}
	h, e := nanz.Parse(source.String(), "size_budget.nanz")
	if e != nil {
		t.Fatal(e)
	}
	var previous string
	for run := 0; run < 2; run++ {
		o := DefaultOptions()
		o.OptSize = true
		o.AssertMode = "none"
		s, e := CompileHIRSteps(hir.CloneModule(h), o)
		if e != nil {
			t.Fatal(e)
		}
		if s.VariantSearch.Limit != 4 || s.VariantSearch.CandidateCompilations != 4 || !s.VariantSearch.Exhausted {
			t.Fatalf("size search exceeded/ignored fixed budget: %+v", s.VariantSearch)
		}
		if run > 0 && s.Assembly != previous {
			t.Fatal("size search output is not deterministic")
		}
		previous = s.Assembly
		for i := 0; i < 8; i++ {
			name := fmt.Sprintf("work%d", i)
			for _, x := range []int64{0, 1, 0x8000, 0xffff} {
				got, e := mir2.NewVM(s.MIR2Module).Call(name, []mir2.Value{{I: x}})
				if e != nil || len(got) != 1 || got[0].I != x>>4 {
					t.Fatal(got, e)
				}
				measureGraceZ80Compiled(t, h, s, name, []int64{x}, nil, x>>4, 200000)
			}
		}
	}
}

func TestOptSizeLengthScanVMZ80(t *testing.T) {
	source := byteLoopCGlobals + `unsigned short length(unsigned char *p){unsigned short i=0;while(p[i]!=0)i++;return i;}
 void execute(void){result=length(buffer+src_off);}`
	cases := []byteLoopCase{}
	for _, n := range []int{0, 1, 2, 127, 255, 256, 1023} {
		initial := make([]byte, 4096)
		for i := range initial {
			initial[i] = byte(i%255 + 1)
		}
		initial[n] = 0
		cases = append(cases, byteLoopCase{initial: initial, want: append([]byte{}, initial...), result: n})
	}
	byteLoopNative(t, "c", "default", source, "CPIR", cases)
	byteLoopNative(t, "c", "size", source, "BYTE_SCAN_SCALAR", cases)
}

func TestOptSizeWordStoreAddressVMZ80(t *testing.T) {
	h := graceMeasureParse(t, `global stored:u16
 fun probe(x:u16)->u16 {stored=x return x}`)
	var sizes [2]int
	for i, size := range []bool{false, true} {
		o := DefaultOptions()
		o.OptSize = size
		o.AssertMode = "none"
		s, e := CompileHIRSteps(hir.CloneModule(h), o)
		if e != nil {
			t.Fatal(e)
		}
		im, e := assembleVariant(s)
		if e != nil {
			t.Fatal(e)
		}
		sizes[i] = len(im.binary)
		for _, x := range []int64{0, 1, 255, 256, 0x8000, 0xffff} {
			vm := mir2.NewVM(s.MIR2Module)
			got, e := vm.Call("probe", []mir2.Value{{I: x}})
			bytes := vm.ReadHeap(vm.GlobalAddr("stored"), 2)
			if e != nil || len(got) != 1 || got[0].I != x || int64(bytes[0])+256*int64(bytes[1]) != x {
				t.Fatal(got, e, bytes)
			}
			measureGraceZ80Compiled(t, h, s, "probe", []int64{x}, nil, x, 200000)
		}
	}
	if sizes[1] >= sizes[0] {
		t.Errorf("unused address retained: %v", sizes)
	}
}
