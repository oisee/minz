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

// C99 6.2.1p7: an automatic object is in scope in its own initializer. The C
// lowering registered a shadowing object's HIR name before lowering the
// initializer but declared it only afterwards, so an initializer naming the
// object failed with an internal "unknown variable" error. The inner reads in
// f, and the non-shadowing self(...) read, are of an indeterminate value, so
// only results that do not depend on them are asserted; sizeof and & of the
// object are well defined. Expected values from a host C compiler.
func TestC89SelfReferentialInitializerNative(t *testing.T) {
	const source = `struct S { unsigned char a, b; };
unsigned char f(unsigned char b) { unsigned char y = b; { unsigned char y = (unsigned char)(y + 1); y = (unsigned char)(y ^ 2); } return y; }
unsigned char g(unsigned char b) { unsigned char n = b; { unsigned char n = sizeof n + 3; b += n; } return n + b; }
unsigned char h(unsigned char b) { struct S s = {1,2}; { struct S s = { sizeof s, b }; b += s.a + s.b; } return s.b + b; }
unsigned char k(unsigned char b) { unsigned char *p = 0; { unsigned char *p = (unsigned char*)&p; b += (p != 0); } return b + (p != 0); }
unsigned char m(unsigned char b) { unsigned char y = b; { unsigned char y = (y = 4, y + 2); b += y; } return y + b; }
unsigned short w(unsigned char b) { unsigned short n = sizeof n + b; return n; }
// assert f(0)==0 via z80
// assert g(0)==4 via z80
// assert h(0)==4 via z80
// assert k(0)==1 via z80
// assert m(0)==6 via z80
// assert w(0)==2 via z80
// assert f(5)==5 via z80
// assert g(5)==14 via z80
// assert h(5)==14 via z80
// assert k(5)==6 via z80
// assert m(5)==16 via z80
// assert w(5)==7 via z80
// assert f(200)==200 via z80
// assert g(200)==148 via z80
// assert h(200)==148 via z80
// assert k(200)==201 via z80
// assert m(200)==150 via z80
// assert w(200)==202 via z80
// assert f(255)==255 via z80
// assert g(255)==2 via z80
// assert h(255)==2 via z80
// assert k(255)==0 via z80
// assert m(255)==4 via z80
// assert w(255)==257 via z80
unsigned char self(unsigned char b) { unsigned char y = y + 1; return b; }
// assert self(7)==7 via z80
`
	runCAssertsNative(t, "self_init.c", source, 25)
}
