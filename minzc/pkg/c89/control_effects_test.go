package c89_test

import (
	c89 "github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"testing"
)

func TestControlEffectsRawVM(t *testing.T) {
	src, e := os.ReadFile("../../../reports/c89-control-effects/source.c")
	if e != nil {
		t.Fatal(e)
	}
	h, e := c89.CompileWithOpts(string(src), "control-effects.c", c89.CompileOpts{NoLibc: true})
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	for _, f := range []string{"while_post", "do_post", "for_post", "choose", "choose_else", "for_continue", "do_continue", "while_call"} {
		t.Run(f, func(t *testing.T) {
			for n := 0; n <= 32; n++ {
				vm := mir2.NewVM(m)
				vm.MaxSteps = 10000
				v, e := vm.Call(f, []mir2.Value{{I: int64(n)}})
				want := 0
				switch f {
				case "choose":
					want = 7
				case "choose_else":
					want = 7
					if n == 0 {
						want = 8
					}
				case "while_post", "for_post":
					want = n * (n + 1) / 2
				case "do_post":
					want = n * (n + 1) / 2
				case "for_continue":
					want = n * (n - 1) / 2
					if n > 2 {
						want -= 2
					}
				case "do_continue":
					want = n * (n + 1) / 2
					if n >= 2 {
						want -= 2
					}
				case "while_call":
					if n > 0 {
						want = n * (n - 1) / 2
					}
					if n > 2 {
						want -= 2
					}
				}
				if e != nil || len(v) != 1 || v[0].I != int64(byte(want)) {
					t.Fatalf("%s n%d got%v want%d err%v", f, n, v, byte(want), e)
				}
			}
		})
	}
}

// Nested loop continue targets belong to the nearest loop; a break bypasses
// post/test effects, and a return in an unselected arm has no effects.
func TestControlEffectsNestedAndBreak(t *testing.T) {
	const source = `unsigned int nested(unsigned char limit) {
 unsigned char i=0,j=0,sum=0;
 for(i=0;i<limit;i++) { j=0; while(++j<4) { if(j==2) continue; sum+=j; } if(i==2) continue; sum+=i; }
 return (unsigned int)i*256+sum;
 }
 unsigned int stop(unsigned char limit) { unsigned char i=0,sum=0; for(;i++<limit;) { if(i==3) break; sum+=i; } return (unsigned int)i*256+sum; }
 unsigned int selected(unsigned char yes) { unsigned char n=7; if(yes++) return n++; return (unsigned int)yes*256+n; }`
	model := func(f string, n int64) int64 {
		switch f {
		case "nested":
			sum := n*4 + n*(n-1)/2
			if n > 2 {
				sum -= 2
			}
			return n*256 + (sum & 255)
		case "stop":
			i, sum := int64(0), int64(0)
			for {
				old := i
				i++
				if old >= n {
					break
				}
				if i == 3 {
					break
				}
				sum += i
			}
			return i*256 + sum
		case "selected":
			if n != 0 {
				return 7
			}
			return 263
		}
		panic("unknown")
	}
	parse := func() *hir.Module {
		h, e := c89.CompileWithOpts(source, "nested-controls.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, o := range []pipeline.Options{{}, {UseHIRGrace: true, UseGrace: true}, {UseHIRGrace: true, UseGrace: true, OptSize: true}} {
		h := parse()
		for _, f := range []string{"nested", "stop", "selected"} {
			for n := int64(0); n <= 8; n++ {
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: f, Args: []int64{n}, Expected: model(f, n), Via: "z80"})
			}
		}
		s, e := pipeline.CompileHIRSteps(h, o)
		if e != nil {
			t.Fatal(e)
		}
		for label, m := range map[string]*mir2.Module{"raw": hir.LowerModule(parse()), "final": s.MIR2Module} {
			for _, f := range []string{"nested", "stop", "selected"} {
				for n := int64(0); n <= 8; n++ {
					vm := mir2.NewVM(m)
					vm.MaxSteps = 10000
					v, e := vm.Call(f, []mir2.Value{{I: n}})
					if e != nil || len(v) != 1 || v[0].I != model(f, n) {
						t.Fatal(label, f, n, v, e, model(f, n))
					}
				}
			}
		}
	}
}
