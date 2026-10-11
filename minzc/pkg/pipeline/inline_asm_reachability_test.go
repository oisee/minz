package pipeline

import (
	"fmt"
	"testing"
)

// The helper's mangled name makes it eligible for pruning. Its only reference
// is an opaque ASM CALL: native execution must still reach its body.
func TestInlineAsmOnlyFunctionReachabilityZ80(t *testing.T) {
	source := `global result:u8 = 0
fun imported__module__helper() -> void { result = 73 }
fun test() -> u8 {
 asm z80 { CALL imported__module__helper }
 return result
}`
	for _, grace := range []bool{false, true} {
		t.Run(fmt.Sprintf("grace=%v", grace), func(t *testing.T) {
			cycles, bytes := measureGraceZ80(t, source, "test", grace, nil, nil, 73)
			t.Logf("result=73 T_states=%d code_bytes=%d", cycles, bytes)
		})
	}
}
