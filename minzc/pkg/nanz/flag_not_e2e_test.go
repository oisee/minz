package nanz_test

import "testing"

// A bool NOT of a comparison held in F must read the comparison's own
// condition, not the carry flag: x == 100 is a zero-flag predicate.
func TestFlagNotOfComparison(t *testing.T) {
	for name, expr := range map[string]string{
		"not-eq":     "~(x == 100)",
		"not-ne":     "~(x != 100)",
		"not-ult":    "~(x < 100)",
		"not-not-eq": "~~(x == 100)",
	} {
		t.Run(name, func(t *testing.T) {
			src := `
fun pick(x: u8) -> u8 {
    let b: bool = ` + expr + `
    if b { return 1 }
    return 7
}
`
			want := map[int]bool{} // input -> b
			for _, x := range []int{0, 5, 99, 100, 101, 200, 255} {
				var v bool
				switch name {
				case "not-eq":
					v = x != 100
				case "not-ne":
					v = x == 100
				case "not-ult":
					v = x >= 100
				case "not-not-eq":
					v = x == 100
				}
				want[x] = v
			}
			for x, v := range want {
				r := 7
				if v {
					r = 1
				}
				src += "assert pick(" + itoa(x) + ") == " + itoa(r) + "\n"
			}
			if err := g0RunAsserts(t, src); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

// A comparison fused with the preceding subtraction lives in the carry flag.
// It must survive a later flag writer and be readable as a byte.
func TestFlagSubCarryPredicateSurvivesFlagWriter(t *testing.T) {
	src := `
global g: u8 = 0
global gb: bool = false
fun f2(a: u8, b: u8, c: u8) -> u8 {
    let d: u8 = a - b
    var lt: bool = a < b
    g = d
    if c == 3 { lt = false }
    if lt { return 1 }
    return 2
}
fun f5(a: u8, b: u8, c: u8) -> u8 {
    let d: u8 = a - b
    let lt: bool = a < b
    g = d
    let e: u8 = c + 200
    g = e
    if lt { return 1 }
    return 2
}
fun f1(a: u8, b: u8) -> u8 {
    let d: u8 = a - b
    let lt: bool = a < b
    g = d
    gb = lt
    return d
}
assert f2(3, 5, 1) == 1
assert f2(3, 5, 3) == 2
assert f2(5, 3, 1) == 2
assert f2(5, 3, 4) == 2
assert f2(7, 7, 1) == 2
assert f5(3, 5, 1) == 1
assert f5(5, 3, 1) == 2
assert f5(3, 5, 100) == 1
assert f5(5, 3, 100) == 2
assert f5(0, 255, 56) == 1
assert f1(3, 5) == 254
assert f1(5, 3) == 2
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
}
