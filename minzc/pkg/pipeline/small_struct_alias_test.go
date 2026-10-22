package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Block-local pointer names must not make another block's pointer an alias of
// a scalarized field. C's lvalue temporaries have their own lowering contract;
// ordinary typed HIR retains storage whenever a field address is taken.
func checkSmallStructFieldAliasesVMZ80(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		t.Run(language, func(t *testing.T) {
			var h *hir.Module
			var err error
			if language == "c" {
				h, err = c89.CompileWithOpts(`typedef struct {unsigned char hi,lo;} Pair;
unsigned char scratch;
unsigned short probe(unsigned char x){Pair p={x,x+1};scratch=0;
if(x&1){unsigned char *q=&p.lo;*q=9;}else{unsigned char *q=&scratch;*q=10;}
return (unsigned short)p.lo*256+scratch;}`, "aliases.c", c89.CompileOpts{NoLibc: true})
			} else {
				h, err = nanz.Parse(`struct Pair {hi:u8,lo:u8}
global scratch:u8=0
fun probe(x:u8)->u16 {
var p:Pair=Pair{hi:x,lo:x+1}
scratch=0
if (x&1)!=0 {let q:^u8=&p.lo
q^=9} else {let q:^u8=&scratch
q^=10}
return u16(p.lo)*256+u16(scratch)
}`, "aliases.nanz")
			}
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: true, AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			vm := mir2.NewVM(s.MIR2Module)
			for x := 0; x < 256; x++ {
				want := int64(((x+1)&255)*256 + 10)
				if x&1 != 0 {
					want = 9 * 256
				}
				got, err := vm.Call("probe", []mir2.Value{{I: int64(x)}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatal(x, got, err, want)
				}
				measureGraceZ80Compiled(t, h, s, "probe", []int64{int64(x)}, nil, want, 10000)
			}
		})
	}
}
