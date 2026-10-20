package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Negation must preserve a computed byte that is also used after the negation.
func TestZ80NegLiveAccumulatorE2E(t *testing.T) {
	for name, source := range map[string]string{
		"minimal": `fun repro(a: u8, b: u8) -> u16 {
let d: u8 = a xor b
let n: u8 = 0 - d
return ((n as u16) + (d as u16))
}
assert repro(62, 39) == 256 via mir2
assert repro(62, 39) == 256 via z80
`,
		"seed7998": `fun h0(p0: u16) -> u16 {
let b0: u8 = ((87 + 67) xor (238 & (p0 as u8)))
let t1: u16 = (p0 | (32768 - p0))
return (t1)
}
fun h1(p0: u16, p1: u16, p2: u16) -> u16 {
let b2: u8 = (25 & (214 & (p0 as u8)))
return ((((b2 as u16) - (b2 as u16)) % 256))
}
fun h2(p0: u16, p1: u16) -> u16 {

return (65535)
}
fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
let d: u8 = ((a as u8) xor (c as u8))
var v0: u16 = a
var v1: u16 = (b xor c)
let b3: u8 = d
let t4: u16 = ((29120 - c) - (v1 & v1))
let t5: u16 = ((1000 + t4) + t4)
let t6: u16 = 32768
let b7: u8 = ((t5 as u8) & ((t6 as u8) - b3))
if a >= v0 {
let b8: u8 = ((b7 + b3) - (b3 - 190))
let t9: u16 = ((v0 + t4) + t4)
} else {
let t10: u16 = (h0(a) + t5)
}
return (((d as u16) xor 1) + (a xor 38) + (v0 xor 75) + (t6 xor 112) + (v1 xor 149) + (b xor 186) + ((b7 as u16) xor 223) + ((b3 as u16) xor 260))
}
fun g() -> u16 { return fuzz_entry(4158, 54623, 295) }
assert fuzz_entry(4158, 54623, 295) == 19969 via z80
`,
	} {
		t.Run(name, func(t *testing.T) {
			hm, err := nanz.Parse(source, name+".nanz")
			if err != nil {
				t.Fatal(err)
			}
			hm.AssertStats = &hir.AssertStats{}
			if _, err := CompileHIRSteps(hm, DefaultOptions()); err != nil {
				t.Fatal(err)
			}
			want := 1
			if name == "minimal" {
				want = 2
			}
			if *hm.AssertStats != (hir.AssertStats{Executed: want, Passed: want}) {
				t.Fatalf("assert stats = %+v", hm.AssertStats)
			}
		})
	}
}

// Keep d live from the entry block across destructive byte operations, including
// the loop back-edge. The independent oracle wraps each operation to a byte.
func TestZ80InPlaceByteLiveAccumulatorE2E(t *testing.T) {
	for _, op := range []struct {
		name, expr string
		eval       func(byte) byte
	}{
		{"neg", "0 - i", func(i byte) byte { return -i }},
		{"not", "~i", func(i byte) byte { return ^i }},
		{"shift", "i << 1", func(i byte) byte { return i << 1 }},
		{"bitget", "i.1", func(i byte) byte { return (i >> 1) & 1 }},
	} {
		for _, shape := range []string{"loop", "loop_branch", "branch", "operand"} {
			t.Run(op.name+"/"+shape, func(t *testing.T) {
				body := "var s: u16 = 0\nvar i: u8 = 0\nwhile i < 3 {\nlet n: u8 = " + op.expr + "\ns = s + (n as u16) + (d as u16)\ni = i + 1\n}\nreturn s"
				want := 0
				for i := byte(0); i < 3; i++ {
					want += int(op.eval(i)) + 25
				}
				if shape == "loop_branch" {
					body = "var s: u16 = 0\nvar i: u8 = 0\nwhile i < 3 {\nvar n: u8 = i\nif a > b { n = " + op.expr + " } else { n = i }\ns = s + (n as u16) + (d as u16)\ni = i + 1\n}\nreturn s"
				}
				if shape == "branch" {
					body = "var s: u16 = 0\nif a > b {\nlet i: u8 = b\nlet n: u8 = " + op.expr + "\ns = n as u16\n} else { s = a as u16 }\nreturn s + (d as u16)"
					want = int(op.eval(39)) + 25
				}
				if shape == "operand" {
					body = "let i: u8 = d\nlet n: u8 = " + op.expr + "\nreturn ((n as u16) + (d as u16))"
					want = int(op.eval(25)) + 25
				}
				source := fmt.Sprintf("fun g(a: u8, b: u8) -> u16 {\nlet d: u8 = a xor b\n%s\n}\nassert g(62, 39) == %d via mir2\nassert g(62, 39) == %d via z80\n", body, want, want)
				hm, err := nanz.Parse(source, "live-byte.nanz")
				if err != nil {
					t.Fatal(err)
				}
				hm.AssertStats = &hir.AssertStats{}
				if _, err := CompileHIRSteps(hm, DefaultOptions()); err != nil {
					t.Fatal(err)
				}
				if *hm.AssertStats != (hir.AssertStats{Executed: 2, Passed: 2}) {
					t.Fatalf("assert stats = %+v", hm.AssertStats)
				}
			})
		}
	}
}
