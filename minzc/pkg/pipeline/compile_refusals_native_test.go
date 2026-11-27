package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
)

// runCAssertsNative compiles C source carrying `// assert ... via z80` lines in
// the default and size profiles and requires every assert to execute and pass
// on the Z80 emulator.
func runCAssertsNative(t *testing.T, name, source string, want int) {
	t.Helper()
	for _, size := range []bool{false, true} {
		h, err := c89.CompileWithOpts(source, name, c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatalf("size=%v: %v", size, err)
		}
		h.AssertStats = &hir.AssertStats{}
		opts := DefaultOptions()
		opts.OptSize = size
		if _, err := CompileHIRSteps(h, opts); err != nil {
			t.Fatalf("size=%v: %v", size, err)
		}
		if h.AssertStats.Executed != want || h.AssertStats.Passed != want {
			t.Fatalf("size=%v: assert stats = %+v, want %d executed and passed", size, *h.AssertStats, want)
		}
	}
}

// A constant-condition ternary after an if/else join leaves the join's
// parameter both forwarded by an otherwise empty block and read directly by
// the block after it. Grace's param-forward elimination deleted the forwarding
// block and left the direct read undefined (MIR2 verify: use of undefined).
// Expected values from a host C compiler.
func TestGraceParamForwardKeepsDominatedUsesNative(t *testing.T) {
	const source = `unsigned short f1(unsigned char a, unsigned char b) { unsigned char x = a, y = b, z = a; unsigned short h = 1;
 if (x & 128) x = (x << 1) ^ y; else x = x << 1;
 { unsigned char t = (64 ? x : y); y ^= t; } h = h + x; return h + y; }
unsigned char f2(unsigned char a, unsigned char b) { unsigned char x = a, y=b; if (a) x = b; else x = 1; y = (64 ? x : y); return y+x; }
// assert f1(0,0)==1 via z80
// assert f2(0,0)==2 via z80
// assert f1(1,2)==3 via z80
// assert f2(1,2)==4 via z80
// assert f1(127,3)==508 via z80
// assert f2(127,3)==6 via z80
// assert f1(128,5)==6 via z80
// assert f2(128,5)==10 via z80
// assert f1(165,2)==147 via z80
// assert f2(165,2)==4 via z80
// assert f1(200,77)==366 via z80
// assert f2(200,77)==154 via z80
// assert f1(255,255)==256 via z80
// assert f2(255,255)==254 via z80
// assert f1(64,128)==129 via z80
// assert f2(64,128)==0 via z80
`
	runCAssertsNative(t, "param_forward.c", source, 16)
}
