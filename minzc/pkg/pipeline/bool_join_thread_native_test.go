package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Short-circuit operators used as conditions are threaded by
// mir2.ThreadBoolJoins. Every case runs over the whole u8 input domain on the
// raw MIR VM, the optimised MIR VM and the Z80, in C and Nanz, in each
// pipeline mode. `count` observes how often a right-hand side with an effect
// ran (C 6.3.13/6.3.14: the right operand is evaluated only when needed).
//
// Each source defines globals input/out/count and execute(), which stores
// f(input) to out. `threaded` cases must end with no boolean join left.
func TestBoolJoinThreadingNativeCAndNanz(t *testing.T) {
	type outcome struct{ out, count byte }
	cases := []struct {
		name     string
		c, nanz  string
		want     func(x byte) outcome
		threaded bool
	}{
		{"and",
			`unsigned char f(unsigned char x){if(x>10&&x<200)return 1;return 2;}`,
			`fun f(x:u8)->u8 {if x>10 && x<200 {return 1} return 2}`,
			func(x byte) outcome { return outcome{pick(x > 10 && x < 200), 0} }, true},
		{"or",
			`unsigned char f(unsigned char x){if(x<10||x>200)return 1;return 2;}`,
			`fun f(x:u8)->u8 {if x<10 || x>200 {return 1} return 2}`,
			func(x byte) outcome { return outcome{pick(x < 10 || x > 200), 0} }, true},
		{"not_and",
			`unsigned char f(unsigned char x){if(!(x>10&&x<200))return 1;return 2;}`,
			`fun f(x:u8)->u8 {if !(x>10 && x<200) {return 1} return 2}`,
			func(x byte) outcome { return outcome{pick(!(x > 10 && x < 200)), 0} }, true},
		{"and3",
			`unsigned char f(unsigned char x){if(x>10&&x<200&&x!=50)return 1;return 2;}`,
			`fun f(x:u8)->u8 {if x>10 && x<200 && x!=50 {return 1} return 2}`,
			func(x byte) outcome { return outcome{pick(x > 10 && x < 200 && x != 50), 0} }, true},
		{"and_or",
			`unsigned char f(unsigned char x){if(x>10&&(x<50||x>150))return 1;return 2;}`,
			`fun f(x:u8)->u8 {if x>10 && (x<50 || x>150) {return 1} return 2}`,
			func(x byte) outcome { return outcome{pick(x > 10 && (x < 50 || x > 150)), 0} }, true},
		{"while_and",
			`unsigned char f(unsigned char x){unsigned char n=0;while(x>10&&x<200){x+=7;n++;}return n;}`,
			`fun f(x:u8)->u8 {var y:u8=x
 var n:u8=0
 while y>10 && y<200 {y=y+7
 n=n+1}
 return n}`,
			func(x byte) outcome {
				n := byte(0)
				for x > 10 && x < 200 {
					x += 7
					n++
				}
				return outcome{n, 0}
			}, true},
		{"call_rhs",
			`unsigned char bump(unsigned char v){count++;return v;}
unsigned char f(unsigned char x){if(x>10&&bump(x)<200)return 1;return 2;}`,
			`fun bump(v:u8)->u8 {count=count+1
 return v}
fun f(x:u8)->u8 {if x>10 && bump(x)<200 {return 1} return 2}`,
			func(x byte) outcome {
				c := byte(0)
				if x > 10 {
					c = 1
				}
				return outcome{pick(x > 10 && x < 200), c}
			}, true},
		{"store_rhs",
			`unsigned char f(unsigned char x){if(x<10||(count=x)>200)return 1;return 2;}`,
			`fun f(x:u8)->u8 {if x<10 || set(x)>200 {return 1} return 2}
fun set(v:u8)->u8 {count=v
 return v}`,
			func(x byte) outcome {
				c := byte(0)
				if !(x < 10) {
					c = x
				}
				return outcome{pick(x < 10 || x > 200), c}
			}, true},
		{"stored_value",
			`unsigned char f(unsigned char x){unsigned char t=x>10&&x<200;if(t)count=9;return t;}`,
			`fun f(x:u8)->u8 {let t:bool = x>10 && x<200
 if t {count=9}
 if t {return 1}
 return 0}`,
			func(x byte) outcome {
				if x > 10 && x < 200 {
					return outcome{1, 9}
				}
				return outcome{0, 0}
			}, false},
		{"return_value",
			`unsigned char f(unsigned char x){return x>=10&&x<=200;}`,
			`fun f(x:u8)->u8 {if x>=10 && x<=200 {return 1} return 0}`,
			func(x byte) outcome { return outcome{zeroOne(x >= 10 && x <= 200), 0} }, false},
		// A bool-returning callee, called twice so it stays a function: its
		// result may be returned in a flag. The callers branch on it.
		{"flag_return",
			`unsigned char in(unsigned char x){return x>=10&&x<=200;}
unsigned char f(unsigned char x){if(in(x))count=count+1;if(in(x+1))return 1;return 2;}`,
			`fun in(x:u8)->bool {return x>=10 && x<=200}
fun f(x:u8)->u8 {if in(x) {count=count+1}
 if in(x+1) {return 1}
 return 2}`,
			func(x byte) outcome {
				c := byte(0)
				if x >= 10 && x <= 200 {
					c = 1
				}
				y := x + 1
				return outcome{pick(y >= 10 && y <= 200), c}
			}, false},
	}
	for _, tc := range cases {
		for _, language := range []string{"c", "nanz"} {
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(tc.name+"/"+language+"/"+mode, func(t *testing.T) {
					parse := func() *hir.Module {
						var h *hir.Module
						var err error
						if language == "c" {
							h, err = c89.CompileWithOpts("unsigned char input,out,count;\n"+tc.c+"\nvoid execute(void){out=f(input);}", "bj.c", c89.CompileOpts{NoLibc: true})
						} else {
							h, err = nanz.Parse("global input:u8=0\nglobal out:u8=0\nglobal count:u8=0\n"+tc.nanz+"\nfun execute(){out=f(input)}", "bj.nanz")
						}
						if err != nil {
							t.Fatal(err)
						}
						return h
					}
					raw := hir.LowerModule(parse())
					s, err := CompileHIRSteps(parse(), Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
					if err != nil {
						t.Fatal(err)
					}
					if tc.threaded {
						for _, f := range s.MIR2Module.Funcs {
							for _, b := range f.Blocks {
								if strings.Contains(b.Label, "cond_join") || strings.Contains(b.Label, "logic_join") {
									t.Fatalf("boolean join @%s left in %s:\n%s", b.Label, f.Name, s.MIR2Module.Dump())
								}
							}
						}
					}
					a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if err != nil {
						t.Fatal(err)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					addr := func(name string) int {
						p := a.Symbols[name]
						if p == 0 {
							p = a.Symbols["v_"+name]
						}
						if p == 0 {
							t.Fatal("missing symbol", name)
						}
						return p
					}
					in, out, count, entry := addr("input"), addr("out"), addr("count"), addr("execute")
					vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], a.Binary)
					for x := 0; x < 256; x++ {
						want := tc.want(byte(x))
						for i, v := range vms {
							v.WriteHeapSlice(v.GlobalAddr("input"), []byte{byte(x)})
							v.WriteHeapSlice(v.GlobalAddr("out"), []byte{0xa5})
							v.WriteHeapSlice(v.GlobalAddr("count"), []byte{0})
							if _, err := v.Call("execute", nil); err != nil {
								t.Fatal(err)
							}
							got := outcome{v.ReadHeap(v.GlobalAddr("out"), 1)[0], v.ReadHeap(v.GlobalAddr("count"), 1)[0]}
							if got != want {
								t.Fatalf("VM %d x=%d got %+v want %+v", i, x, got, want)
							}
						}
						mem[in], mem[out], mem[count] = byte(x), 0xa5, 0
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{F: byte(x), PC: uint16(entry), SP: 0xeffe})
						for step := 0; step < 20000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						regs := z.GetRegisters()
						got := outcome{mem[out], mem[count]}
						if regs.PC != 0xf100 || regs.SP != 0xf000 || got != want || mem[in] != byte(x) {
							t.Fatalf("Z80 x=%d got %+v want %+v PC/SP=%04x/%04x\n%s", x, got, want, regs.PC, regs.SP, s.Assembly)
						}
					}
				})
			}
		}
	}
}

func pick(c bool) byte {
	if c {
		return 1
	}
	return 2
}

func zeroOne(c bool) byte {
	if c {
		return 1
	}
	return 0
}
