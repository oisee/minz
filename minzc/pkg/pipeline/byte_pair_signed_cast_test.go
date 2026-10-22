package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestBytePairUnsignedReinterpretVMZ80(t *testing.T) {
	h, err := c89.CompileWithOpts(`unsigned short packed(signed char lo,signed char hi){return (unsigned char)lo | ((unsigned short)(unsigned char)hi<<8);}`, "lanes.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.MIR2Module.Dump(), "pack_bytes") {
		t.Fatal(s.MIR2Module.Dump())
	}
	for _, lo := range []int64{-128, -127, -1, 0, 1, 126, 127} {
		for _, hi := range []int64{-128, -127, -1, 0, 1, 126, 127} {
			want := int64(uint16(byte(lo)) | uint16(byte(hi))<<8)
			for _, m := range []*mir2.Module{raw, s.MIR2Module} {
				got, e := mir2.NewVM(m).Call("packed", []mir2.Value{{I: lo}, {I: hi}})
				if e != nil || len(got) != 1 || got[0].I != want {
					t.Fatal(lo, hi, got, e, want)
				}
			}
			measureGraceZ80Compiled(t, h, s, "packed", []int64{lo, hi}, nil, want, 1000)
		}
	}
}
