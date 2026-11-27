package pipeline

import (
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/nanz"
)

// Review repros (W5 shift-carry / bit-accumulate). Each once returned a wrong value on the Z80; each
// carries its expected values as "via z80" assertions, checked here in the
// speed and size profiles.
//   - shiftcarry, shiftcarry_osize, handunroll: wrong with the shift-carry
//     preference before #339 (a spilled byte moved through a live A); right
//     with #339 and the preference unchanged.
//   - bitaccum_h88: a fused bit-accumulate step whose shift is homed in
//     another register wrote that register before the shift's position.
var reviewRepros = map[string]string{
	// Straight-line CRC and population-count chains (hand-unrolled loops).
	"handchain": `// assert crc(1, 7) == 7 via z80
// assert crc(255, 49) == 172 via z80
// assert pop(255) == 8 via z80
// assert pop(165) == 4 via z80
// assert pop(0) == 0 via z80
// assert crc(128, 29) == 38 via z80
unsigned char crc(unsigned char x, unsigned char k) {
#define S if (x & 128) x = (x << 1) ^ k; else x = x << 1;
    S S S S S S S S
    return x;
}
unsigned short pop(unsigned char v) {
    unsigned short c = 0;
#define P c += v & 1; v = v >> 1;
    P P P P P P P P
    return c;
}
`,
	"shiftcarry": `// assert f0(28, 166) == 1609 via z80
// assert f0(216, 103) == 1744 via z80
// assert f0(137, 50) == 790 via z80
// assert f0(92, 201) == 697 via z80
// assert f0(78, 115) == 1957 via z80
// assert f0(45, 79) == 1171 via z80
// assert f0(0, 0) == 1 via z80
// assert f0(255, 255) == 2658 via z80
// assert f0(90, 3) == 277 via z80
// assert f0(112, 92) == 1401 via z80
// assert f0(1, 2) == 67 via z80
// assert f0(128, 64) == 1032 via z80
unsigned short f0(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a ^ 0x5a, r, i, j, n = b & 3;
    unsigned short c = 0, h = 1;
    z = (112 & y);
    if (x & 128) x = (x << 1) ^ 7; else x = x << 1;
    if (z > (x - z)) { x |= (z | (y ^ y)); } else { y += ((y * 3) + (z ^ z)); c += z & 1; z = z >> 1; }
    h += z & 1;
    return c + h + x + (y << 3) + (unsigned short)(z * 7);
}
`,
	"shiftcarry_osize": `// assert f1(28, 166) == 1699 via z80
// assert f1(216, 103) == 2408 via z80
// assert f1(137, 50) == 2032 via z80
// assert f1(92, 201) == 1180 via z80
// assert f1(78, 115) == 1444 via z80
// assert f1(45, 79) == 1351 via z80
// assert f1(0, 0) == 1044 via z80
// assert f1(255, 255) == 3054 via z80
// assert f1(90, 3) == 1912 via z80
// assert f1(112, 92) == 1265 via z80
// assert f1(1, 2) == 1181 via z80
// assert f1(128, 64) == 1342 via z80
unsigned short f1(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a ^ 0x5a, r, i, j, n = b & 3;
    unsigned short c = 0, h = 1;
    z = y >> 1;
    if (x & 128) x = (x << 1) ^ 0x1d; else x = x << 1;
    x |= (z & (17 + 113));
    if ((124 ^ x) < (46 - z)) { if (y & 128) y = (y << 1) ^ 0x1d; else y = y << 1; if (z & 128) z = (z << 1) ^ 0x1d; else z = z << 1; }
    y = (z | (y << 2));
    h += z & 1;
    z = z >> 1;
    z -= (((153 & 128) ? z : 219) & (x + z));
    c += z & 1;
    if (y & 128) y = (y << 1) ^ 0x31; else y = y << 1;
    h += z & 1;
    z = 149;
    if ((85 * 3) == (x ^ z)) { h += x & 1; x = x >> 1; z = ((8 * 3) - (y + 243)); }
    c += x & 1;
    y = (y - x);
    x = y >> 1;
    return c + h + x + (y << 3) + (unsigned short)(z * 7);
}
`,
	"handunroll": `// assert f3(112, 102) == 76 via z80
// assert f3(156, 138) == 114 via z80
// assert f3(49, 62) == 237 via z80
// assert f3(10, 239) == 157 via z80
// assert f3(88, 241) == 161 via z80
// assert f3(5, 6) == 153 via z80
// assert f3(255, 255) == 30 via z80
// assert f3(1, 2) == 118 via z80
// assert f3(128, 66) == 117 via z80
unsigned char f3(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a ^ 0x5a, r, n = b & 3;
    unsigned short c = 0, h = 1;
    if (n) for (r = 0; r < n; r++) { z += ((233 | x) + x); z |= ((z + 102) & (z | y)); h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1; h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1; h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1; h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1; h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1; h += x & 1; x = x >> 1; z += 226; c += z & 1; z = z >> 1;  x += 7; }
    return (unsigned char)(c + h + x + (y << 3) + (z << 1));
}
`,
	"bitaccum_h88": `// assert h(5, 9, 3) == 35 via z80
// assert h(255, 200, 17) == 393 via z80
// assert h(165, 60, 255) == 378 via z80
// assert h(1, 2, 3) == 26 via z80
// assert h(128, 77, 250) == 365 via z80
// assert h(90, 129, 16) == 240 via z80
unsigned short h(unsigned char v, unsigned char x, unsigned char y) { unsigned short c = y; unsigned char q = x + 3, q2 = y ^ 9, w; c += v & 1; y = q2 + x; w = v >> 1; x = w + q2; return c + x + y; }
`,
}

func TestReviewReprosZ80(t *testing.T) {
	for name, src := range reviewRepros {
		for _, size := range []bool{false, true} {
			h, err := c89.CompileWithOpts(src, name+".c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(h.Asserts) == 0 {
				t.Fatalf("%s: no assertions", name)
			}
			o := DefaultOptions()
			o.OptSize = size
			if _, err := CompileHIRSteps(h, o); err != nil {
				t.Errorf("%s size=%v: %v", name, size, err)
			}
		}
	}
}

// Blockless functions reach the allocator; these modules once panicked in
// the shift-carry preference. They must fail with the ordinary diagnostic.
func TestZMachineModulesDoNotPanic(t *testing.T) {
	for _, path := range []string{"../../../examples/zmachine/zork-west-demo/zvm.nanz", "../../../examples/zmachine/spectrum128/zvm_core.nanz"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dir := path[:strings.LastIndex(path, "/")]
		h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{BaseDir: dir, StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		o := DefaultOptions()
		o.AssertMode = "none"
		if _, err := CompileHIRSteps(h, o); err == nil || !strings.Contains(err.Error(), "exceeds 16-bit address space") {
			t.Errorf("%s: %v", path, err)
		}
	}
}
