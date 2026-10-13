package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Language Book v7 Appendix A lists both infix XOR and postfix dereference.
func TestCaretXORAndPointerDereferenceVMZ80(t *testing.T) {
	source := `fun xor_byte(a:u8,b:u8)->u8 { return a ^ b }
 fun mixed(p:ptr, b:u8)->u8 { p^=37
 return p^ ^ b }
 fun deref_keyword(p:ptr,b:u8)->u8 { p^=37
 return p^ xor b }
 fun deref_add(p:ptr)->u8 { p^=37
 return p^+1 }
 fun keyword(a:u8,b:u8)->u8 { return a xor b }
 fun negative(a:u8)->u8 { return a ^ -1 }`
	for _, tc := range []struct {
		fn   string
		args []int64
		want int64
	}{
		{"xor_byte", []int64{0, 255}, 255}, {"xor_byte", []int64{37, 5}, 32}, {"xor_byte", []int64{255, 255}, 0},
		{"mixed", []int64{0x9000, 5}, 32},
		{"deref_keyword", []int64{0x9000, 5}, 32}, {"deref_add", []int64{0x9000}, 38}, {"keyword", []int64{37, 5}, 32}, {"negative", []int64{37}, 218},
	} {
		t.Run(tc.fn, func(t *testing.T) {
			h := graceMeasureParse(t, source)
			s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			args := make([]mir2.Value, len(tc.args))
			for i, a := range tc.args {
				args[i] = mir2.Value{I: a}
			}
			vm := mir2.NewVM(s.MIR2Module)
			vm.EnsureHeap(65536)
			got, err := vm.Call(tc.fn, args)
			if err != nil || len(got) != 1 || got[0].I != tc.want {
				t.Fatalf("got%v err%v want%d", got, err, tc.want)
			}
			measureGraceZ80HIR(t, h, tc.fn, Options{AssertMode: "none"}, tc.args, nil, tc.want)
		})
	}
}
