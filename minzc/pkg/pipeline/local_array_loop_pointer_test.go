package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Book v7 §2.6 array storage/indexing and §3.1 u8 arithmetic: an array binding carries
// its base address; its elements remain byte-sized across loop parameters.
func TestLocalArrayLoopPointerWidthVMZ80(t *testing.T) {
	src := `global padding:[u8;300]
fun test(n:u8)->u8 {
 var data:[u8;3]
 data[0]=5
 data[1]=17
 data[2]=201
 var i:u8=0
 var acc:u8=0
 while i<n {
  acc=acc*3+data[i]
  i=i+1
 }
 return acc
}`
	for n := 0; n <= 3; n++ {
		want := int64(0)
		for _, v := range []int64{5, 17, 201}[:n] {
			want = (want*3 + v) & 255
		}
		for _, grace := range []bool{false, true} {
			t.Run(fmt.Sprintf("n%d/grace%v", n, grace), func(t *testing.T) {
				raw := hir.LowerModule(graceMeasureParse(t, src))
				vm := mir2.NewVM(raw)
				if vm.GlobalAddr("test$data") < 256 {
					t.Fatal("fixture must expose pointer truncation")
				}
				v, e := vm.Call("test", []mir2.Value{{I: int64(n)}})
				if e != nil || len(v) != 1 || v[0].I != want {
					t.Fatal("raw VM", v, e, want)
				}
				measureGraceZ80(t, src, "test", grace, []int64{int64(n)}, nil, want)
			})
		}
	}
}
