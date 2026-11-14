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
