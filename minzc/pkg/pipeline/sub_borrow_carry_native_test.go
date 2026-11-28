package pipeline

import "testing"

// CmpSubCarryNot(d, a) reads the borrow of d = y - a from the carry flag.
// With a in A the subtraction was emitted as NEG / ADD A, y, whose carry is
// (a != 0 && y >= a), not the borrow, so e = y >= a came out inverted (and
// wrong for a = 0). Found by gen13 seed 195; the loop leaves y in a block
// parameter so the split helper receives a in A.
func TestSubBorrowCarryAfterNegAddNative(t *testing.T) {
	want := func(in []byte) byte {
		a, b, c := in[0], in[1], in[2]
		y := b
		if c > 0 && b != a {
			y = 128
		}
		if y >= c {
			e := byte(0)
			if y >= a {
				e = 1
			}
			c = y - a + e
		}
		return c
	}
	var inputs [][]byte
	for a := 0; a < 256; a++ {
		for _, b := range []byte{0, 1, 7, 127, 128, 129, 208, 255} {
			for _, c := range []byte{0, 1, 2, 31, 128, 200} {
				inputs = append(inputs, []byte{byte(a), b, c})
			}
		}
	}
	runByteGlobalsNative(t, []byteGlobalsCase{
		{"neg_add_borrow", "c", `unsigned char i0, i1, i2, out;
unsigned char f0(unsigned char a, unsigned char b, unsigned char c) {
  unsigned char x = a, y = b;
  unsigned char k, n = c;
  k = 0;
  while (k++ < n) {
    unsigned char t = x;
    if (b != t) y = 128;
  }
  unsigned char d = (unsigned char)(y - a);
  unsigned char e = y >= a;
  if (y >= c) c = (unsigned char)(d + e);
  return c;
}
void execute(void) { out = f0(i0, i1, i2); }`, inputs, want},
	})
}
