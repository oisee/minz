package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestTypedStructBoolSlotsVMZ80(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, tc := range []struct {
			name, source, probe string
			fields              int
		}{
			{"two", `struct B2 { a: bool, b: bool }
fun mk(x:u8)->B2 {return B2{a:(x&1)==1,b:(x&2)==2}}
fun probe(x:u8)->u8 {let b:B2=mk(x)
var n:u8=0
if b.a {n=n+1}
if b.b {n=n+2}
return n}`, "probe", 2},
			{"sb4", `struct B3 { a: bool, b: bool, c: bool }
fun mk(x: u8) -> B3 { return B3{a: (x & 1) == 1, b: (x & 2) == 2, c: x > 127} }
fun probe(x: u8) -> u8 { let b: B3 = mk(x)
  var n: u8 = 0
  if b.a { n = n + 1 }
  if b.b { n = n + 2 }
  if b.c { n = n + 4 }
  return n }
`, "probe", 3},
			{"bits_struct", `struct Bits { a: bool, b: bool, c: bool, d: bool }
fun mk_bits(x: u8) -> Bits { return Bits{a: (x & 1) == 1, b: (x & 2) == 2, c: (x & 4) != 0, d: x > 127} }
fun bits_probe(x: u8) -> u8 { let b: Bits = mk_bits(x)
  var n: u8 = 0
  if b.a { n = n + 1 }
  if b.b { n = n + 2 }
  if b.c { n = n + 4 }
  if b.d { n = n + 8 }
  return n }
`, "bits_probe", 4},
		} {
			t.Run(fmt.Sprintf("%s/grace%v", tc.name, grace), func(t *testing.T) {
				h, err := nanz.Parse(tc.source, "bool_slots.nanz")
				if err != nil {
					t.Fatal(err)
				}
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
				if err != nil {
					t.Fatal(err)
				}
				vm := mir2.NewVM(s.MIR2Module)
				for x := 0; x < 256; x++ {
					want := x & 3
					if tc.fields == 4 {
						want = x & 7
					}
					if tc.fields > 2 && x > 127 {
						want |= 1 << (tc.fields - 1)
					}
					got, err := vm.Call(tc.probe, []mir2.Value{{I: int64(x)}})
					if err != nil || len(got) != 1 || got[0].I != int64(want) {
						t.Fatalf("x=%d VM=%v err=%v want=%d", x, got, err, want)
					}
					measureGraceZ80Compiled(t, h, s, tc.probe, []int64{int64(x)}, nil, int64(want), 10000)
				}
				name := "mk"
				if tc.fields == 4 {
					name = "mk_bits"
				}
				wantSlots := 1 // guarded aggregates return an address
				if tc.fields == 2 {
					wantSlots = 2
				}
				if got := len(s.MIR2Module.FuncByName(name).Contract.Returns); got != wantSlots {
					t.Fatalf("return slots=%d want=%d:\n%s", got, wantSlots, s.MIR2Opt)
				}
			})
		}
	}
}

func TestTypedStructExplicitParamABIVMZ80(t *testing.T) {
	const source = `struct Pair {hi:u8,lo:u8}
fun consume_hl(@z80_hl p:Pair)->u16 {return u16(p.hi)*256+u16(p.lo)}
fun probe(x:u8)->u16 {let p:Pair=Pair{hi:x,lo:x+1}
return consume_hl(p)}`
	for _, grace := range []bool{false, true} {
		h, err := nanz.Parse(source, "abi.nanz")
		if err != nil {
			t.Fatal(err)
		}
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s.Assembly, "p: Pair = HL") {
			t.Fatalf("explicit aggregate ABI lost:\n%s", s.Assembly)
		}
		vm := mir2.NewVM(s.MIR2Module)
		for x := 0; x < 256; x++ {
			want := int64(x*256 + ((x + 1) & 255))
			got, err := vm.Call("probe", []mir2.Value{{I: int64(x)}})
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("x=%d VM=%v err=%v want=%d", x, got, err, want)
			}
			measureGraceZ80Compiled(t, h, s, "probe", []int64{int64(x)}, nil, want, 10000)
		}
	}
}

func TestSingleBitSignedMasksVMZ80(t *testing.T) {
	const source = `unsigned char i8b7_eq(signed char x){return (x & 0x80)==0x80;}
unsigned char i8b7_neg(signed char x){return (x & -128)==-128;}
unsigned char i16b15_neg(short x){return (x & -32768)==-32768;}
unsigned char i16b15_ne(short x){return (x & -32768)!=-32768;}`
	for _, grace := range []bool{false, true} {
		h, err := c89.CompileWithOpts(source, "cbits.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		vm := mir2.NewVM(s.MIR2Module)
		for _, name := range []string{"i8b7_eq", "i8b7_neg", "i16b15_neg", "i16b15_ne"} {
			inputs := []int64{-32768, -32767, -256, -129, -128, -1, 0, 1, 127, 128, 255, 256, 32767}
			if strings.HasPrefix(name, "i8") {
				inputs = nil
				for x := -128; x < 128; x++ {
					inputs = append(inputs, int64(x))
				}
			}
			for _, x := range inputs {
				want := int64(0)
				if x < 0 {
					want = 1
				}
				if name == "i16b15_ne" {
					want = 1 - want
				}
				got, err := vm.Call(name, []mir2.Value{{I: x}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("%s(%d) VM=%v err=%v want=%d", name, x, got, err, want)
				}
				measureGraceZ80Compiled(t, h, s, name, []int64{x}, nil, want, 10000)
			}
		}
	}
}
