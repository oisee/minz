package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Every input is checked against an independent oracle, before and after MIR
// optimization and on Z80. Check the tuple itself as well as a caller consuming
// every slot: a truthiness check alone would accept noncanonical bool bytes.
func TestMultiBoolReturnVMZ80(t *testing.T) {
	multiReturnProfiles(t, func(t *testing.T, opt Options) {
		t.Run("expressions", func(t *testing.T) { testMultiBoolReturnVMZ80(t, opt) })
		t.Run("critic_repros", func(t *testing.T) { testMultiBoolReturnReprosVMZ80(t, opt) })
	})
}

func multiReturnProfiles(t *testing.T, check func(*testing.T, Options)) {
	t.Helper()
	for _, profile := range []string{"default", "size", "grace", "nograce"} {
		opt := DefaultOptions()
		opt.AssertMode = "none"
		opt.OptSize = profile == "size"
		opt.UseHIRGrace = profile == "grace"
		opt.UseGrace = profile != "nograce"
		t.Run(profile, func(t *testing.T) { check(t, opt) })
	}
}

func testMultiBoolReturnVMZ80(t *testing.T, opt Options) {
	for _, tc := range []struct {
		name, types, exprs, lets, names, consume string
		count                                    int
	}{
		{"three", "bool,bool,bool", "(x & 1)==1,(x & 2)==2,x>127", "let a:bool=(x & 1)==1\nlet b:bool=(x & 2)==2\nlet c:bool=x>127", "a,b,c", "if a {n=n+1}\nif b {n=n+2}\nif c {n=n+4}", 3},
		{"four", "bool,bool,bool,bool", "(x & 1)==1,(x & 2)==2,(x & 4)!=0,(x & 8)!=0", "let a:bool=(x & 1)==1\nlet b:bool=(x & 2)==2\nlet c:bool=(x & 4)!=0\nlet d:bool=(x & 8)!=0", "a,b,c,d", "if a {n=n+1}\nif b {n=n+2}\nif c {n=n+4}\nif d {n=n+8}", 4},
		{"mixed", "bool,u8,bool,u8", "x!=0,x+1,x>127,x xor 85", "let a:bool=x!=0\nlet b:u8=x+1\nlet c:bool=x>127\nlet d:u8=x xor 85", "a,b,c,d", "n=b+d\nif a {n=n+1}\nif c {n=n+4}", 4},
	} {
		for _, bound := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bound=%v", tc.name, bound), func(t *testing.T) {
				body := "return (" + tc.exprs + ")"
				if bound {
					body = tc.lets + "\nreturn (" + tc.names + ")"
				}
				source := fmt.Sprintf("fun make_values(x:u8)->(%s){%s}\nfun probe(x:u8)->u8{let (%s)=make_values(x)\nvar n:u8=0\n%s\nreturn n}", tc.types, body, tc.names, tc.consume)
				bit := func(b bool) int64 {
					if b {
						return 1
					}
					return 0
				}
				var asserts []hir.Assert
				for x := int64(0); x < 256; x++ {
					vals := []int64{x & 1, (x >> 1) & 1, bit(x > 127)}
					want := vals[0] + 2*vals[1] + 4*vals[2]
					if tc.name == "four" {
						vals = []int64{x & 1, (x >> 1) & 1, (x >> 2) & 1, (x >> 3) & 1}
						want = x & 15
					}
					if tc.name == "mixed" {
						vals = []int64{bit(x != 0), (x + 1) & 255, bit(x > 127), x ^ 85}
						want = (vals[1] + vals[3] + vals[0] + 4*vals[2]) & 255
					}
					for _, via := range []string{"mir2", "z80"} {
						asserts = append(asserts, hir.Assert{FuncName: "make_values", Args: []int64{x}, ExpectedMulti: vals, Via: via, Source: fmt.Sprintf("tuple x=%d", x)}, hir.Assert{FuncName: "probe", Args: []int64{x}, Expected: want, Via: via, Source: fmt.Sprintf("caller x=%d", x)})
					}
				}
				parse := func() *hir.Module {
					h, e := nanz.Parse(source, "multi-bool.nanz")
					if e != nil {
						t.Fatal(e)
					}
					h.Sandboxes = []hir.Sandbox{{Name: "tuple", Asserts: asserts}}
					return h
				}
				h := parse()
				if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
					t.Fatal(e)
				}
				h = parse()
				steps, e := CompileHIRSteps(h, opt)
				if e != nil {
					t.Fatal(e)
				}
				f := steps.MIR2Module.FuncByName("make_values")
				if len(f.Contract.Returns) != tc.count {
					t.Fatalf("tuple contract lost: %v", f.Contract)
				}
				for _, b := range f.Blocks {
					for _, in := range b.Insts {
						if in.Op == mir2.OpCmp && steps.Allocation.Loc(in.Dst).Kind != mir2.LocFlag && !strings.Contains(steps.Assembly, ".bool_true_") {
							t.Fatal("allocated comparison has no byte materialization")
						}
					}
				}
				if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
					t.Fatal(e)
				}
				defer func() {
					if t.Failed() {
						t.Log(steps.Assembly)
					}
				}()
				p28RunNative(t, h, steps)
			})
		}
	}
}

func TestCBoolStructReturnVMZ80(t *testing.T) {
	multiReturnProfiles(t, testCBoolStructReturnVMZ80)
}

func testCBoolStructReturnVMZ80(t *testing.T, opt Options) {
	for _, count := range []int{3, 4} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			fields, assign, consume := "", "", ""
			exprs := []string{"(x&1)==1", "(x&2)==2", "x>127", "(x&8)!=0"}
			for i := 0; i < count; i++ {
				fields += fmt.Sprintf("_Bool b%d;", i)
				assign += fmt.Sprintf("r.b%d=%s;", i, exprs[i])
				consume += fmt.Sprintf("if(r.b%d)n+=%d;", i, 1<<i)
			}
			source := fmt.Sprintf("struct Bits{%s}; struct Bits make_values(unsigned char x){struct Bits r;%s return r;} unsigned char probe(unsigned char x){struct Bits r=make_values(x);unsigned char n=0;%s return n;}", fields, assign, consume)
			h, e := c89.Compile(source, "bool-struct.c")
			if e != nil {
				t.Fatal(e)
			}
			var asserts []hir.Assert
			for x := int64(0); x < 256; x++ {
				want := x & 3
				if x > 127 {
					want += 4
				}
				if count == 4 {
					want += x & 8
				}
				for _, via := range []string{"mir2", "z80"} {
					asserts = append(asserts, hir.Assert{FuncName: "probe", Args: []int64{x}, Expected: want, Via: via, Source: fmt.Sprintf("C struct%d x=%d", count, x)})
				}
			}
			h.Sandboxes = []hir.Sandbox{{Name: "C structs", Asserts: asserts}}
			if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
				t.Fatal(e)
			}
			h, e = c89.Compile(source, "bool-struct.c")
			if e != nil {
				t.Fatal(e)
			}
			h.Sandboxes = []hir.Sandbox{{Name: "C structs", Asserts: asserts}}
			steps, e := CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
				t.Fatal(e)
			}
			p28RunNative(t, h, steps)
		})
	}
}

// Preserve the supplied programs verbatim, including the addressable Nanz
// structs, so later struct-return lowering also exercises this boundary.
func testMultiBoolReturnReprosVMZ80(t *testing.T, opt Options) {
	for _, tc := range []struct {
		file, fn string
		bits     []uint
	}{
		{"tup3d", "probe", []uint{0, 1, 7}},
		{"tup4b", "probe", []uint{0, 1, 2, 3}},
		{"sb4", "probe", []uint{0, 1, 7}},
		{"sb1", "probe", []uint{0, 1, 2, 3}},
		{"bits_struct", "bits_probe", []uint{0, 1, 2, 7}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			src, e := os.ReadFile("testdata/multi_bool_return/" + tc.file + ".nanz")
			if e != nil {
				t.Fatal(e)
			}
			var asserts []hir.Assert
			for x := int64(0); x < 256; x++ {
				var want int64
				for slot, bit := range tc.bits {
					want |= ((x >> bit) & 1) << slot
				}
				for _, via := range []string{"mir2", "z80"} {
					asserts = append(asserts, hir.Assert{FuncName: tc.fn, Args: []int64{x}, Expected: want, Via: via, Source: fmt.Sprintf("%s x=%d", tc.file, x)})
				}
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(src), tc.file+".nanz")
				if e != nil {
					t.Fatal(e)
				}
				h.Sandboxes = []hir.Sandbox{{Name: "repro", Asserts: asserts}}
				return h
			}
			h := parse()
			if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
				t.Fatal(e)
			}
			h = parse()
			steps, e := CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
				t.Fatal(e)
			}
			p28RunNative(t, h, steps)
		})
	}
}
