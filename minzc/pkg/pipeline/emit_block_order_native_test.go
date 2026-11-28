package pipeline

import "testing"

// The Z80 block emitter used to swap `move -> A; 8-bit ALU -> non-A`
// (reorderAccMoves) while liveness, pending-A tracking and scratch choice
// still followed block order. gy seed 81: `r61 = move r55` (D -> A) ran after
// `r62 = add` allocated to D, r55's home once r55 dies at the move. gz seed
// 549: the ALU "saved" A as the not-yet-defined move result into the moved
// value's home (it is free after the move in block order), or lost a source
// already in A to the ALU's scratch use of A.
func TestEmitterKeepsBlockOrderNative(t *testing.T) {
	want := func(in []byte) byte {
		a, b, c := in[0], in[1], in[2]
		x, y := b+2, c-31
		for k := c & 7; k != 0; k-- {
			d7 := b - c
			e8 := byte(0)
			if b <= c {
				e8 = 1
			}
			if 17 > byte(1-x) {
				y = d7 ^ 64
			} else {
				x = d7 + e8
			}
			t9 := 2 & x
			if y-3 != a&b {
				y = t9 + 1
				if 6 != a { // s10 = 1 - 251 (mod 256)
					x = 255 - a
				}
			}
		}
		d11 := b - c
		e12 := byte(0)
		if b > c {
			e12 = 1
		}
		if c > y+x {
			x = d11 + e12
		} else {
			y = e12
		}
		return x ^ y
	}
	var inputs [][]byte
	for c := 0; c < 256; c++ {
		for _, a := range []byte{0, 1, 6, 127, 128, 254, 255} {
			for _, b := range []byte{0, 1, 2, 127, 128, 200, 254, 255} {
				inputs = append(inputs, []byte{a, b, byte(c)})
			}
		}
	}
	var small [][]byte // the MIR VM's step budget spans all calls of one case
	for a := 0; a < 256; a++ {
		for _, b := range []byte{0, 1, 128, 254} {
			for _, c := range []byte{0, 1, 2, 7} {
				small = append(small, []byte{byte(a), b, c})
			}
		}
	}
	h := func(a, b, c byte) (uint16, uint16) { return uint16(a)*3 + uint16(b), uint16(c)*5 + 7 }
	b2b := func(v bool) byte {
		if v {
			return 1
		}
		return 0
	}
	// gz 549, reduced.
	want549b := func(in []byte) byte {
		a, b, c := in[0], in[1], in[2]
		x, y, z0, z1, z2, z3 := b, byte(0), a+a, b, b, a
		h1, h2 := h(a, b, c)
		for k := c; k != 0; k-- {
			if int(x)+int(z0) > 0 {
				x = y + 4
				z1 = c & z0
				z3 = 0
				if y+y <= z1 {
					z3 = a
				}
			}
			z2 = b2b(y <= z0)
		}
		return y ^ z0 ^ z1 ^ z2 ^ z3 ^ byte(h1>>3) ^ byte(h2)
	}
	runByteGlobalsNative(t, []byteGlobalsCase{
		{"gz549b", "c", `unsigned char i0, i1, i2, out;
unsigned char f0(unsigned char a, unsigned char b, unsigned char c) {
  unsigned char x = (unsigned char)(b), y = 0;
  unsigned char z0 = (unsigned char)(a + a);
  unsigned char z1 = (unsigned char)(b);
  unsigned char z2 = (unsigned char)(b);
  unsigned char z3 = (unsigned char)(a);
  unsigned char k, n = (unsigned char)(c);
  unsigned int h1 = (unsigned int)a * 3u + b, h2 = (unsigned int)c * 5u + 7u;
  for (k = n; k != 0; k--) {
    if ((unsigned int)x + z0 > 0) {
      unsigned char t11 = (unsigned char)(b + c);
      unsigned char t12 = (unsigned char)(y + 4);
      unsigned char t13 = (unsigned char)(c & z0);
      if (a < 0) z0 = (unsigned char)(t11);
      else { x = (unsigned char)(t12 + 0); z1 = (unsigned char)(t13 + 0); z3 = ((unsigned char)(y + y) <= (unsigned char)(z1)) ? (unsigned char)(a) : (unsigned char)(0); }
    }
    unsigned char e16 = y <= z0;
    z2 = e16;
  }
  return (unsigned char)(y ^ z0 ^ z1 ^ z2 ^ z3 ^ (h1 >> 3) ^ h2);
}
void execute(void) { out = f0(i0, i1, i2); }`, small, want549b},
		{"gy81", "nanz", `global i0: u8 = 0
global i1: u8 = 0
global i2: u8 = 0
global out: u8 = 0
fun f0(a: u8, b: u8, c: u8) -> u8 {
  var x: u8 = (b + 2)
  var y: u8 = (c - 31)
  var k: u8 = c & 7
  while k != 0 {
      let d7: u8 = b - c
      let e8: u8 = (b <= c) as u8
      if 17 > (1 - x) { y = d7 xor 64 } else { x = d7 + e8 }
    let t9: u8 = (2 & x)
    if (y - 3) == (a & b) {
    } else {
      y = t9 + 1
      let s10: u8 = 1 - 251
      if s10 != a { x = (255 - a) }
    }
    k = k - 1
  }
  let d11: u8 = b - c
  let e12: u8 = (b > c) as u8
  if (c + 0) > (y + x) { x = d11 + e12 } else { y = e12 }
  return x xor y
}
fun execute() { out = f0(i0, i1, i2) }`, inputs, want},
	})
}
