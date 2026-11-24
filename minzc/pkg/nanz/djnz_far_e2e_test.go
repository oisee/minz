package nanz_test

import "testing"

// A byte-counted `for x in a[lo..hi]` loop whose body holds a call, a break
// and, laid out inside the loop, the function's exit code with a 16-bit
// multiply: the DJNZ latch is further than 128 bytes from the body label.
// The compiler used to emit the unassemblable DJNZ anyway.
func TestDJNZBodyOutOfRangeStillAssembles(t *testing.T) {
	src := `
global a: [u8; 5] = [0, 0, 7, 8, 9]
global calls: u16 = 0
fun pos(x: u8) -> bool { calls = calls + 1
  return x > 0 }
fun d1() -> u16 { calls = 0
  var s: u16 = 0
  var c: u8 = 0
  for x in a[0..5] {
    if c == 2 { break }
    if pos(x) { c = c + 1
      s = s + u16(x) }
  }
  return calls * 1000 + s }
assert d1() == 4015
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}
