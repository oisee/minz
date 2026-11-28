package pipeline

import (
	"fmt"
	"strings"
	"testing"
)

// A compare may reuse the borrow of an adjacent sub only when it IS that
// borrow: borrow(x - y) == x <u y. FuseAbsDiff also mapped cmp.ule(x, y)
// (and cmp.uge(y, x) / cmp.ugt(x, y) after swapping the sub) onto it, so
// `d = y - c; e = y <= c` gave e = 0 for y == c (reviewer's gx/gy generators).
//
// Truth table checked here, every byte pair, MIR VM and Z80, default and
// --Osize, C and Nanz, with the sub's result live in three shapes:
//
//	compare        same order (sub y,c)   swapped (sub y,c; cmp c,y)
//	y <  c         borrow                 c < y  == !borrow && y != c
//	y <= c         borrow || y == c       c <= y == !borrow
//	y >  c         !borrow && y != c      c > y  == borrow
//	y >= c         !borrow                c >= y == borrow || y == c
//
// plus signed char, 16-bit, a constant operand and the sub's result as an
// operand (none of which may be fused onto the byte borrow).
func TestSubCarryCompareTruthTableNative(t *testing.T) {
	ops := map[string]func(a, b int) bool{
		"<": func(a, b int) bool { return a < b }, "<=": func(a, b int) bool { return a <= b },
		">": func(a, b int) bool { return a > b }, ">=": func(a, b int) bool { return a >= b },
	}
	names := map[string]string{"<": "lt", "<=": "le", ">": "gt", ">=": "ge"}
	b2i := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	shapes := []struct {
		name, c, nz string
		want        func(d, e byte, y, c byte, cmp bool) byte
	}{
		{"arm", "if (y > y) return d; return e;", "if y > y { return d }\n  return e",
			func(d, e, y, c byte, cmp bool) byte { return e }},
		{"sum", "return (unsigned char)(d + e);", "return d + e",
			func(d, e, y, c byte, cmp bool) byte { return d + e }},
		{"branch", "if (L OP R) return (unsigned char)(c - y); return d;", "if L OP R { return c - y }\n  return d",
			func(d, e, y, c byte, cmp bool) byte {
				if cmp {
					return c - y
				}
				return d
			}},
	}
	var cases []byteGlobalsCase
	for _, op := range []string{"<", "<=", ">", ">="} {
		for _, swapped := range []bool{false, true} {
			l, r := "y", "c"
			if swapped {
				l, r = "c", "y"
			}
			for _, sh := range shapes {
				op, swapped, sh := op, swapped, sh
				want := func(in []byte) byte {
					y, c := in[0], in[1]
					a, b := int(y), int(c)
					if swapped {
						a, b = b, a
					}
					cmp := ops[op](a, b)
					return sh.want(y-c, byte(b2i(cmp)), y, c, cmp)
				}
				cBody := strings.NewReplacer("L", l, "OP", op, "R", r).Replace(sh.c)
				nBody := strings.NewReplacer("L", l, "OP", op, "R", r).Replace(sh.nz)
				name := fmt.Sprintf("%s_%s_%v", names[op], sh.name, swapped)
				cases = append(cases,
					byteGlobalsCase{name, "c", fmt.Sprintf(`unsigned char i0, i1, out;
unsigned char f(unsigned char y, unsigned char c) {
  unsigned char d = (unsigned char)(y - c);
  unsigned char e = %s %s %s;
  %s
}
void execute(void) { out = f(i0, i1); }`, l, op, r, cBody), allBytePairs(), want},
					byteGlobalsCase{name, "nanz", fmt.Sprintf(`global i0: u8 = 0
global i1: u8 = 0
global out: u8 = 0
fun f(y: u8, c: u8) -> u8 {
  let d: u8 = y - c
  let e: u8 = (%s %s %s) as u8
  %s
}
fun execute() { out = f(i0, i1) }`, l, op, r, nBody), allBytePairs(), want})
			}
		}
		op := op
		// signed char: the borrow is an unsigned fact, never a signed ordering.
		cases = append(cases, byteGlobalsCase{"signed_" + names[op], "c", fmt.Sprintf(`signed char i0, i1; unsigned char out;
unsigned char f(signed char y, signed char c) {
  signed char d = (signed char)(y - c);
  unsigned char e = y %s c;
  if (y > y) return (unsigned char)d;
  return (unsigned char)(d + e);
}
void execute(void) { out = f(i0, i1); }`, op), allBytePairs(), func(in []byte) byte {
			y, c := int8(in[0]), int8(in[1])
			return byte(y-c) + byte(b2i(ops[op](int(y), int(c))))
		}})
		// constant operand and the sub's own result as an operand.
		cases = append(cases, byteGlobalsCase{"const_" + names[op], "c", fmt.Sprintf(`unsigned char i0, i1, out;
unsigned char f(unsigned char y, unsigned char c) {
  unsigned char d = (unsigned char)(y - 7);
  unsigned char e = y %s 7;
  if (c == 200) return d;
  return (unsigned char)(d + e);
}
void execute(void) { out = f(i0, i1); }`, op), allBytePairs(), func(in []byte) byte {
			y, c := in[0], in[1]
			if c == 200 {
				return y - 7
			}
			return y - 7 + byte(b2i(ops[op](int(y), 7)))
		}})
		cases = append(cases, byteGlobalsCase{"result_" + names[op], "c", fmt.Sprintf(`unsigned char i0, i1, out;
unsigned char f(unsigned char y, unsigned char c) {
  unsigned char d = (unsigned char)(y - c);
  unsigned char e = d %s c;
  if (y > y) return d;
  return (unsigned char)(d + e);
}
void execute(void) { out = f(i0, i1); }`, op), allBytePairs(), func(in []byte) byte {
			d := in[0] - in[1]
			return d + byte(b2i(ops[op](int(d), int(in[1]))))
		}})
		// 16-bit: y = i0:i1, c = i2:i3 (sampled, includes y == c and y == c +/- 1).
		var wide [][]byte
		for _, hi := range []byte{0, 1, 0x7f, 0x80, 0xff} {
			for _, lo := range []byte{0, 1, 2, 0x7f, 0x80, 0xfe, 0xff} {
				for _, dh := range []byte{0, 1, 0xff} {
					for _, dl := range []byte{0, 1, 0xff} {
						wide = append(wide, []byte{hi, lo, hi + dh, lo + dl})
					}
				}
			}
		}
		cases = append(cases, byteGlobalsCase{"word_" + names[op], "c", fmt.Sprintf(`unsigned char i0, i1, i2, i3, out;
unsigned char f(unsigned int y, unsigned int c) {
  unsigned int d = y - c;
  unsigned char e = y %s c;
  if (y > y) return (unsigned char)d;
  return (unsigned char)((d >> 8) ^ d ^ (e << 4));
}
void execute(void) { out = f((unsigned int)i0 << 8 | i1, (unsigned int)i2 << 8 | i3); }`, op), wide, func(in []byte) byte {
			y, c := uint16(in[0])<<8|uint16(in[1]), uint16(in[2])<<8|uint16(in[3])
			d := y - c
			return byte(d>>8) ^ byte(d) ^ byte(b2i(ops[op](int(y), int(c)))<<4)
		}})
	}
	runByteGlobalsNative(t, cases)
}
