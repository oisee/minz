package pipeline

import (
	"fmt"
	"testing"
)

// Short-circuit joins produce byte-valued boolean block parameters. Their
// branch must test the value, independent of predecessor comparison flags.
func TestMaterializedBooleanBranchZ80(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, x := range []byte{0, 1, 2} {
			for _, y := range []byte{0, 1, 2} {
				src := fmt.Sprintf(`global x:u8=%d
global y:u8=%d
fun test()->u8 { if x==1 && y==0 {return 91} if x==0 || y==2 {return 37} return 12 }`, x, y)
				want := byte(12)
				if x == 1 && y == 0 {
					want = 91
				} else if x == 0 || y == 2 {
					want = 37
				}
				measureGraceZ80(t, src, "test", grace, nil, nil, int64(want))
			}
		}
	}
}
