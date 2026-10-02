package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

const callJudgeSource = `
fun mn(a: u8, b: u8, c: u8) -> u8 { if a < b { return c - a } return c - b }
fun call_sub(a: u8, b: u8) -> u8 { if a == b { return 0 } return a - b }
fun fc(a: u8, b: u8) -> u8 { return call_sub(a,b) + call_sub(b,a) + a }
fun h_c1(x: u8, y: u8) -> u8 { return mn(3, x, y) }
fun h_c2(x: u8, y: u8) -> u8 { return mn(x, y, 100) + x }
fun w(a: u16, b: u16) -> u16 { if a < b { return b - a } return a - b }
fun h_c4(x: u16, y: u16) -> u16 { return w(x, 1000) + y }
`

func TestProductionCallJudge(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model func(a, b uint8) int64
	}{
		{"fc", func(a, b uint8) int64 { return int64(a) }},
		{"h_c1", func(a, b uint8) int64 {
			m := a
			if m > 3 {
				m = 3
			}
			return int64(b - m)
		}},
		{"h_c2", func(a, b uint8) int64 {
			m := a
			if b < m {
				m = b
			}
			return int64(uint8(100 - m + a))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := nanz.Parse(callJudgeSource, "calls")
			if err != nil {
				t.Fatal(err)
			}
			fix := compileProductionHIRFixture(t, hm)
			checked, bad := newU8Judge(t, fix, tc.name, false).sweep(func(a, b uint8) bool { return true }, tc.model)
			t.Logf("%s: %d/%d mismatches", tc.name, len(bad), checked)
			if len(bad) > 0 {
				t.Fatalf("%s\n%s", describeMismatches(bad), fix.asm)
			}
		})
	}
	t.Run("h_c4", func(t *testing.T) {
		hm, err := nanz.Parse(callJudgeSource, "calls")
		if err != nil {
			t.Fatal(err)
		}
		var cases [][]int64
		for x := int64(0); x < 65536; x++ {
			cases = append(cases, []int64{x, 7})
		}
		f := hm.Funcs[len(hm.Funcs)-1]
		judgeMultiplyCases(t, f, cases, func(a []int64) int64 {
			d := a[0] - 1000
			if d < 0 {
				d = -d
			}
			return (d + a[1]) & 65535
		}, hm.Funcs[:len(hm.Funcs)-1]...)
	})
}

// Sample every byte boundary through permuted contracts, nested call chains,
// runtime scratch writes, and a call on a loop back edge.
func TestProductionCallShapes(t *testing.T) {
	const src = `
fun diff(a: u8, b: u8) -> u8 { if a < b { return b-a } return a-b }
fun seven(a:u8,b:u8,c:u8,d:u8,e:u8,f:u8,g:u8)->u8 {
 if a < b { return c-d+e-f+g } return c+d-e+f-g
}
fun perm(a:u8,b:u8,c:u8,d:u8,e:u8,f:u8,g:u8)->u8 { return seven(g,f,e,d,c,b,a) }
fun nest(a:u8,b:u8)->u8 { return diff(diff(a,b),a) }
fun chain(a:u8,b:u8)->u8 { return nest(b,a) }
fun loop(a:u8,b:u8)->u8 { var r:u8 = a
 var i:u8 = 0
 while i < 4 { r = diff(r,b)
 i = i + 1 }
 return r }
fun mul(a:u16,b:u16)->u16 { if a == 0 { return b } return a*b }
fun scratch(a:u16,b:u16,c:u16)->u16 { return mul(a,b)+c }
fun cyc(a:u16,b:u16,c:u16)->u16 { return mul(c,a)+b }
fun two(a:u16,b:u16,c:u16)->u16 { return mul(a,b)+mul(b,c)+a }
fun wordloop(a:u16,b:u16)->u16 { var r:u16 = a
 var i:u8 = 0
 while i < 3 { r = mul(r,b)
 i = i + 1 }
 return r }
fun mix0(a:u16,b:u16,c:u16) -> u16 {
if a < b {
return ((c - a) xor 60921)
}
return ((c - b) xor 60921)
}
fun mix1(a:u16,b:u16,c:u16) -> u16 {
if a < b {
return ((c - a) xor 15791)
}
return ((c - b) xor 15791)
}
fun wordnest(a:u16,b:u16,c:u16)->u16 { return mix1(mix0(a,b,c),b,a) }
fun wordrotate(a:u16,b:u16,c:u16)->u16 { return mix0(c,a,b) }
fun mixedhelper(x:u8,w:u16,y:u8)->u16 { if x < y {return w + x} return w + y }
fun mixed(a:u16,b:u8,c:u8)->u16 { return mixedhelper(c,a,b) + (b as u16) }
fun mn(a:u8,b:u8,c:u8)->u8 { if a<b { return c-a } return c-b }
fun perm3(x:u8,y:u8,z:u8)->u8 { return mn(z,x,y) }
`
	diff := func(a, b int64) int64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	for _, tc := range []struct {
		name  string
		n     int
		word  bool
		model func([]int64) int64
	}{
		{"perm", 7, false, func(a []int64) int64 {
			if a[6] < a[5] {
				return (a[4] - a[3] + a[2] - a[1] + a[0]) & 255
			}
			return (a[4] + a[3] - a[2] + a[1] - a[0]) & 255
		}},
		{"perm3", 3, false, func(a []int64) int64 {
			m := a[0]
			if a[2] < m {
				m = a[2]
			}
			return (a[1] - m) & 255
		}},
		{"nest", 2, false, func(a []int64) int64 { return diff(diff(a[0], a[1]), a[0]) }},
		{"chain", 2, false, func(a []int64) int64 { return diff(diff(a[1], a[0]), a[1]) }},
		{"loop", 2, false, func(a []int64) int64 {
			r := a[0]
			for i := 0; i < 4; i++ {
				r = diff(r, a[1])
			}
			return r
		}},
		{"scratch", 3, true, func(a []int64) int64 {
			v := a[0] * a[1]
			if a[0] == 0 {
				v = a[1]
			}
			return (v + a[2]) & 65535
		}},
		{"two", 3, true, func(a []int64) int64 {
			m := a[0] * a[1]
			if a[0] == 0 {
				m = a[1]
			}
			n := a[1] * a[2]
			if a[1] == 0 {
				n = a[2]
			}
			return (m + n + a[0]) & 65535
		}},
		{"wordloop", 2, true, func(a []int64) int64 {
			r := a[0]
			for i := 0; i < 3; i++ {
				if r == 0 {
					r = a[1]
				} else {
					r = (r * a[1]) & 65535
				}
			}
			return r
		}},
		{"wordnest", 3, true, func(a []int64) int64 {
			m := a[0]
			if a[1] < m {
				m = a[1]
			}
			v := ((a[2] - m) & 65535) ^ 60921
			m = v
			if a[1] < m {
				m = a[1]
			}
			return ((a[0] - m) & 65535) ^ 15791
		}},
		{"wordrotate", 3, true, func(a []int64) int64 {
			m := a[2]
			if a[0] < m {
				m = a[0]
			}
			return ((a[1] - m) & 65535) ^ 60921
		}},
		{"mixed", 3, true, func(a []int64) int64 {
			x, y := a[2]&255, a[1]&255
			if y < x {
				x = y
			}
			return (a[0] + x + y) & 65535
		}},
		{"cyc", 3, true, func(a []int64) int64 {
			v := a[2] * a[0]
			if a[2] == 0 {
				v = a[0]
			}
			return (v + a[1]) & 65535
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := nanz.Parse(src, "call_shapes")
			if err != nil {
				t.Fatal(err)
			}
			var f *hir.Func
			var callees []*hir.Func
			for _, fn := range hm.Funcs {
				if fn.Name == tc.name {
					f = fn
				} else {
					callees = append(callees, fn)
				}
			}
			var cases [][]int64
			limit := int64(256)
			if tc.word {
				limit = 65536
			}
			for x := int64(0); x < limit; x++ {
				if tc.word && x > 1023 && x%257 != 0 {
					continue
				}
				for _, y := range []int64{0, 1, 3, 127, 128, 255} {
					args := make([]int64, tc.n)
					args[0] = x
					args[1] = y
					for i := 2; i < tc.n; i++ {
						args[i] = (x*int64(i+1) + y) & (limit - 1)
					}
					cases = append(cases, args)
				}
			}
			if tc.name == "perm3" {
				cases = nil
				for x := int64(0); x < 256; x++ {
					for z := int64(0); z < 256; z++ {
						cases = append(cases, []int64{x, 50, z})
					}
				}
			}
			judgeMultiplyCases(t, f, cases, tc.model, callees...)
			t.Logf("%d cases, zero mismatches", len(cases))
		})
	}
}
