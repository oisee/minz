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
