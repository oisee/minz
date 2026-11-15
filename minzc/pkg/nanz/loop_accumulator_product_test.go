package nanz_test

import "testing"

// A word accumulator multiplied by a zero-extended byte in a loop keeps its
// high byte: the accumulator starts as a small constant, but the product that
// replaces it is a word.
func TestLoopAccumulatorTimesByteKeepsWord(t *testing.T) {
	src := `
fun factorial(n: u8) -> u16 {
    var acc: u16 = 1
    var i: u8 = 0
    while i < n {
        acc = acc * ((i + 1) as u16)
        i = i + 1
    }
    return acc
}
assert factorial(0) == 1
assert factorial(5) == 120
assert factorial(6) == 720
assert factorial(7) == 5040
assert factorial(8) == 40320
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}

// A word loop counter that runs past 255 is multiplied as a word, and a
// sign-extended byte with its sign bit set keeps its 0xFF high byte.
func TestWordCounterAndSextByteMultiplyAsWords(t *testing.T) {
	src := `
fun tri(n: u16, k: u8) -> u16 {
    var s: u16 = 0
    var i: u16 = 0
    while i < n {
        s = s + i * (k as u16)
        i = i + 1
    }
    return s
}
fun sextmul(p: u8, w: u16) -> i16 {
    let s: i16 = (p as i8) as i16
    return (w as i16) * s
}
assert tri(10, 3) == 135
assert tri(300, 1) == 44850
assert sextmul(100, 3) == 300
assert sextmul(200, 3) == -168
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}
