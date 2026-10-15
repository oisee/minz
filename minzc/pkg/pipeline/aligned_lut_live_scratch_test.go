package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Exercise each LUT addressing form with an unrelated live neighbour, plus
// indices in H/A that the selected address/scratch sequence could overwrite.
func TestAlignedLUTRetainsLiveHighByteNative(t *testing.T) {
	for _, regs := range [][2]string{{"c", "b"}, {"e", "d"}, {"h", "l"}, {"l", "h"}, {"a", "b"}} {
		t.Run(regs[0]+"-"+regs[1], func(t *testing.T) {
			source := fmt.Sprintf(`global colors:[u8;8]=[0,5,6,3,4,2,70,1]
    fun pick(@z80_%s index:u8,@z80_%s keep:u8)->u8 {
     let color:u8=colors[index]
     return color+keep
    }`, regs[0], regs[1])
			for _, size := range []bool{false, true} {
				for index, w := range []byte{0, 5, 6, 3, 4, 2, 70, 1} {
					for _, keep := range []byte{0, 1, 127, 128, 165, 255} {
						h, e := nanz.Parse(source, "aligned-lut-live.nanz")
						if e != nil {
							t.Fatal(e)
						}
						h.Globals[0].Ty.(*mir2.ArrayTy).Align = 256
						measureGraceZ80HIR(t, h, "pick", Options{ContractOpt: true, OptSize: size, AssertMode: "none"}, []int64{int64(index), int64(keep)}, nil, int64(w+keep))
					}
				}
			}
		})
	}
}

// The fix must keep the inexpensive addressing form when no neighbour is live.
func TestAlignedLUTNoLiveNeighbourRetainsFastPath(t *testing.T) {
	h, e := nanz.Parse(`global colors:[u8;8]=[0,5,6,3,4,2,70,1]
 fun pick(@z80_c index:u8)->u8 {return colors[index]}`, "aligned-lut-fast.nanz")
	if e != nil {
		t.Fatal(e)
	}
	h.Globals[0].Ty.(*mir2.ArrayTy).Align = 256
	cycles, _ := measureGraceZ80HIR(t, h, "pick", Options{ContractOpt: true, AssertMode: "none"}, []int64{6}, nil, 70)
	if cycles != 24 {
		t.Fatalf("uncontended page lookup costs %d T, want 24 including RET", cycles)
	}
}
