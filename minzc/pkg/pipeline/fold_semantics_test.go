package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestFoldEffectsAndStageOrderVMZ80(t *testing.T) {
	cases := []struct {
		name, src string
		want      int64
	}{
		{"callback_prefix", `global hits:u8
fun callback(acc:u8,x:u8)->u8 { hits = hits + 1
 return acc + x }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..n).fold(0,callback)
 return sum + hits*16 }`, 95},
		{"acc_only_callback", `global hits:u8
fun callback(acc:u8)->u8 { hits=hits+1
 return acc+1 }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..n).fold(0,callback)
 return sum+hits*16 }`, 85},
		{"callback_local", `fun callback(acc:u8,x:u8)->u8 { let shifted=x+1
 return acc+shifted }
fun test(n:u8)->u8 { return range(0..n).fold(0,callback) }`, 20},
		{"counter_order", `fun callback(acc:u8,x:u8)->u8 { return acc*3+x }
fun test(n:u8)->u8 { return range(0..n).fold(0,callback) }`, (5*81 + 4*27 + 3*9 + 2*3 + 1) & 255},
		{"filter_then_map", `global hits:u8
fun keep(x:u8)->bool { return x>2 }
fun mapper(x:u8)->u8 { hits=hits+1
 return x+1 }
fun callback(acc:u8,x:u8)->u8 { return acc+x }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..n).filter(keep).map(mapper).fold(0,callback)
 return sum+hits*16 }`, 63},
		{"map_then_filter", `global hits:u8
fun keep(x:u8)->bool { return x>3 }
fun mapper(x:u8)->u8 { hits=hits+1
 return x+1 }
fun callback(acc:u8,x:u8)->u8 { return acc+x }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..n).map(mapper).filter(keep).fold(0,callback)
 return sum+hits*16 }`, 95},
		{"two_filters", `global hits:u8
fun keep(x:u8)->bool { return x>2 }
fun predicate(x:u8)->bool { hits=hits+1
 return x<5 }
fun callback(acc:u8,x:u8)->u8 { return acc+x }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..n).filter(keep).filter(predicate).fold(0,callback)
 return sum+hits*16 }`, 55},
		{"filter_none", `fun keep(x:u8)->bool { return false }
fun callback(acc:u8,x:u8)->u8 { return acc+x }
fun test(n:u8)->u8 { return range(0..n).filter(keep).fold(7,callback) }`, 7},
		{"empty", `global hits:u8
fun callback(acc:u8,x:u8)->u8 { hits=hits+1
 return acc+x }
fun test(n:u8)->u8 { hits=0
 let sum=range(0..0).fold(7,callback)
 return sum+hits }`, 7},
		{"memory_stages", `global data:[u8;5] = [1,2,3,4,5]
global hits:u8
fun keep(x:u8)->bool { return x>2 }
fun mapper(x:u8)->u8 { hits=hits+1
 return x+1 }
fun callback(acc:u8,x:u8)->u8 { return acc+x }
fun test(n:u8)->u8 { hits=0
 let sum=fold(data.filter(keep).map(mapper),0,callback,n)
 return sum+hits*16 }`, 63},
	}
	for _, c := range cases {
		counts := []int{5}
		if c.name == "callback_prefix" || c.name == "counter_order" {
			counts = []int{0, 1, 5, 255}
		}
		for _, n := range counts {
			want := c.want
			if c.name == "callback_prefix" {
				want = int64((n*(n+1)/2 + n*16) & 255)
			}
			if c.name == "counter_order" {
				want = 0
				for i := n; i > 0; i-- {
					want = (want*3 + int64(i)) & 255
				}
			}
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/n=%d/grace=%v", c.name, n, enabled), func(t *testing.T) {
					h := graceMeasureParse(t, c.src)
					if enabled {
						if _, err := hir.RunGrace(h, 0); err != nil {
							t.Fatal(err)
						}
					}
					m := hir.LowerModule(h)
					if err := mir2.Verify(m); err != nil {
						t.Fatal(err)
					}
					got, err := mir2.NewVM(m).Call("test", []mir2.Value{{I: int64(n)}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("raw VM got=%v want=%d err=%v\n%s", got, want, err, m.Dump())
					}
					cycles, bytes := measureGraceZ80(t, c.src, "test", enabled, []int64{int64(n)}, nil, want)
					t.Logf("code_bytes=%d T_states=%d", bytes, cycles)
				})
			}
		}
	}
}
