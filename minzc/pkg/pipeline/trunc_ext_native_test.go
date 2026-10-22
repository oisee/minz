package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestTruncExtensionCRCBitNative(t *testing.T) {
	const src = `unsigned char step(unsigned char crc) {
 if(crc&128) return (crc<<1)^7; return crc<<1; }`
	for _, mode := range []string{"default", "size", "grace"} {
		t.Run(mode, func(t *testing.T) {
			h, err := c89.CompileWithOpts(src, "bit.c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseHIRGrace: mode == "grace", UseGrace: mode == "grace", OptSize: mode == "size", AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(s.Assembly, "LD H, 0") {
				t.Fatal("unnecessary word extension", s.Assembly)
			}
			vm := mir2.NewVM(s.MIR2Module)
			for x := int64(0); x < 256; x++ {
				want := byte(x) << 1
				if x&128 != 0 {
					want ^= 7
				}
				got, err := vm.Call("step", []mir2.Value{{I: x}})
				if err != nil || got[0].I != int64(want) {
					t.Fatal(x, got, err, want)
				}
				measureGraceZ80Compiled(t, h, s, "step", []int64{x}, nil, int64(want), 1000)
			}
		})
	}
}
