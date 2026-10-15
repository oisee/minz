package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestZXSpriteByteWaveNative(t *testing.T) {
	source := `import zx.sprite_effect
global offsets:[i8;8]
fun run(phase:u8)->u8 {
 let wave:ByteWave=ByteWave{phase:phase}
 wave.fill(&offsets[0],8)
 var sum:u8=0
 var row:u8=0
 while row<8 {sum=sum*3+u8(offsets[row]) row=row+1}
 return sum
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "bytewave.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		for _, phase := range []int{0, 1, 2, 3, 4, 5, 6, 7, 127, 128, 255} {
			want := byte(0)
			for row := 0; row < 8; row++ {
				want = want*3 + byte(wave[(phase+row)%8])
			}
			measureGraceZ80HIR(t, parse(), "run", opt, []int64{int64(phase)}, nil, int64(want))
		}
	}
}
