package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestSmallStructNeutralVMZ80(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		t.Run(language, func(t *testing.T) {
			parse := func() *hir.Module {
				var h *hir.Module
				var err error
				if language == "c" {
					h, err = c89.CompileWithOpts(`typedef struct {unsigned char hi,lo;} Pair;
Pair branch(unsigned char x){switch(x){case 0:{Pair p={7,8};return p;}default:{Pair p={x,1};return p;}}}
unsigned short switched_probe(unsigned char x){switch(x){case 0:{Pair p=branch(x);return (unsigned short)p.hi*256+p.lo;}default:{Pair p=branch(x);return (unsigned short)p.hi*256+p.lo;}}}
unsigned char trace;
unsigned char mark(unsigned char x){trace=trace*3+x;return x;}
Pair ordered(unsigned char x){unsigned char lo=mark(x+1),hi=mark(x);Pair p={hi,lo};return p;}
unsigned short ordered_probe(unsigned char x){trace=0;Pair p=ordered(x);return (unsigned short)p.hi*256+trace;}
Pair make_pair(unsigned char x){Pair p={x,x+1};return p;}
unsigned short consume(Pair p){return (unsigned short)p.hi*256+p.lo;}
unsigned short probe(unsigned char x,unsigned char y){Pair p=make_pair(x);return (consume(p)+x)^y;}
void mutate_byte(unsigned char *p){*p=9;}
unsigned char field_escaped(unsigned char x){Pair p={x,x+1};mutate_byte(&p.lo);return p.lo;}
void mutate(Pair *p){p->lo=9;}
unsigned char escaped(unsigned char x){Pair p={x,x+1};mutate(&p);return p.lo;}
unsigned char local(unsigned char x){Pair p={x,x+1};p.lo+=3;return p.lo;}`, "small.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse(`struct Pair {hi:u8,lo:u8}
fun branch(x:u8)->Pair {switch x {
case 0: return Pair{hi:7,lo:8}
default: return Pair{hi:x,lo:1}
}}
fun switched_probe(x:u8)->u16 {switch x {
case 0:
let p:Pair=branch(x)
return u16(p.hi)*256+u16(p.lo)
default:
let p:Pair=branch(x)
return u16(p.hi)*256+u16(p.lo)
}}
global trace:u8=0
fun mark(x:u8)->u8 {trace=trace*3+x
return x}
fun ordered(x:u8)->Pair {return Pair{lo:mark(x+1),hi:mark(x)}}
fun ordered_probe(x:u8)->u16 {trace=0
let p:Pair=ordered(x)
return u16(p.hi)*256+u16(trace)}
fun make_pair(x:u8)->Pair {return Pair{hi:x,lo:x+1}}
fun consume(p:Pair)->u16 {return u16(p.hi)*256+u16(p.lo)}
fun probe(x:u8,y:u8)->u16 {let p:Pair=make_pair(x)
return (consume(p)+u16(x)) xor u16(y)}
fun mutate_byte(p:^u8){p^=9}
fun field_escaped(x:u8)->u8 {var p:Pair=Pair{hi:x,lo:x+1}
mutate_byte(&p.lo)
return p.lo}
fun mutate(p:^Pair){p.lo=9}
fun escaped(x:u8)->u8 {var p:Pair=Pair{hi:x,lo:x+1}
mutate(&p)
return p.lo}
fun local(x:u8)->u8 {var p:Pair=Pair{hi:x,lo:x+1}
p.lo=p.lo+3
return p.lo}`, "small.nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				return h
			}
			for _, grace := range []bool{false, true} {
				h := parse()
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
				if err != nil {
					t.Fatal(err)
				}
				if len(s.MIR2Module.FuncByName("make_pair").Contract.Returns) != 2 {
					t.Fatalf("struct return not scalarized:\n%s", s.MIR2Opt)
				}
				for _, f := range s.MIR2Module.Funcs {
					for _, b := range f.Blocks {
						for _, i := range b.Insts {
							if i.Op == mir2.OpAlloca && f.Name != "escaped" && f.Name != "field_escaped" {
								t.Fatalf("aggregate storage remains in %s:\n%s", f.Name, s.MIR2Opt)
							}
						}
					}
				}
				for _, name := range []string{"escaped", "field_escaped"} {
					retained := false
					for _, b := range s.MIR2Module.FuncByName(name).Blocks {
						for _, i := range b.Insts {
							retained = retained || i.Op == mir2.OpAlloca
						}
					}
					if !retained {
						t.Fatal("address-taken object lost its storage", name)
					}
				}
				vm := mir2.NewVM(s.MIR2Module)
				for x := 0; x < 256; x++ {
					for _, y := range []int{0, 1, 127, 255} {
						// The low field wraps before widening; the addition wraps at u16.
						want := int64(((x*256 + ((x + 1) & 255) + x) & 65535) ^ y)
						got, e := vm.Call("probe", []mir2.Value{{I: int64(x)}, {I: int64(y)}})
						if e != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("x%d y%d VM%v err%v want%d", x, y, got, e, want)
						}
						measureGraceZ80Compiled(t, h, s, "probe", []int64{int64(x), int64(y)}, nil, want, 10000)
					}
					gotEscape, errEscape := vm.Call("escaped", []mir2.Value{{I: int64(x)}})
					if errEscape != nil || len(gotEscape) != 1 || gotEscape[0].I != 9 {
						t.Fatal("escaped", x, gotEscape, errEscape)
					}
					measureGraceZ80Compiled(t, h, s, "escaped", []int64{int64(x)}, nil, 9, 10000)
					gotField, errField := vm.Call("field_escaped", []mir2.Value{{I: int64(x)}})
					if errField != nil || len(gotField) != 1 || gotField[0].I != 9 {
						t.Fatal("field address escape", x, gotField, errField)
					}
					measureGraceZ80Compiled(t, h, s, "field_escaped", []int64{int64(x)}, nil, 9, 10000)
					wantSwitch := int64(x*256 + 1)
					if x == 0 {
						wantSwitch = 7*256 + 8
					}
					gotSwitch, errSwitch := vm.Call("switched_probe", []mir2.Value{{I: int64(x)}})
					if errSwitch != nil || len(gotSwitch) != 1 || gotSwitch[0].I != wantSwitch {
						t.Fatal("switch", x, gotSwitch, errSwitch, wantSwitch)
					}
					measureGraceZ80Compiled(t, h, s, "switched_probe", []int64{int64(x)}, nil, wantSwitch, 10000)
					wantOrdered := int64(x*256 + ((((x+1)&255)*3 + x) & 255))
					gotOrdered, errOrdered := vm.Call("ordered_probe", []mir2.Value{{I: int64(x)}})
					if errOrdered != nil || len(gotOrdered) != 1 || gotOrdered[0].I != wantOrdered {
						t.Fatal("initializer order", x, gotOrdered, errOrdered, wantOrdered)
					}
					measureGraceZ80Compiled(t, h, s, "ordered_probe", []int64{int64(x)}, nil, wantOrdered, 10000)
					want := int64((x + 4) & 255)
					got, e := vm.Call("local", []mir2.Value{{I: int64(x)}})
					if e != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("local x%d VM%v err%v", x, got, e)
					}
					measureGraceZ80Compiled(t, h, s, "local", []int64{int64(x)}, nil, want, 10000)
				}
			}
		})
	}
	t.Run("field_aliases", checkSmallStructFieldAliasesVMZ80)
}
