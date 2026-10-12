package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestHIRGraceScalarHoistProofBoundaries(t *testing.T) {
	cases := []struct {
		name, source string
		eligible     bool
	}{
		{"plain", `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 i=i+1}
 return s}`, true},

		{"bit-write", `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {a.0=1
 s=s+(a+b)
 i=i+1}
 return s}`, false},
		{"mutated", `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 a=a+1
 i=i+1}
 return s}`, false},
		{"shadowed", `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {let a:u16=i
 s=s+(a+b)
 i=i+1}
 return s}`, false},
		{"call", `fun helper(a:u16)->u16{return a}
 fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {s=s+helper(a+b)
 i=i+1}
 return s}`, false},
		{"before", `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 a=a+1
 var i:u8=0
 while i<n {s=s+(a+b)
 i=i+1}
 return s}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := graceMeasureParse(t, c.source)
			after := graceMeasureParse(t, c.source)
			r, err := hir.RunGrace(after, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(after.Dump(), "__grace_base_"); got != c.eligible {
				t.Fatalf("hoisted=%v eligible=%v rules=%v", got, c.eligible, r)
			}
			for _, pair := range [][2]int64{{0, 0}, {1, 2}, {65535, 1}, {65535, 65535}, {32768, 32768}} {
				for _, n := range []int64{0, 1, 5, 16, 64, 255} {
					args := []mir2.Value{{I: pair[0]}, {I: pair[1]}, {I: n}}
					a, err := mir2.NewVM(hir.LowerModule(before)).Call("f", args)
					if err != nil {
						t.Fatal(err)
					}
					b, err := mir2.NewVM(hir.LowerModule(after)).Call("f", args)
					if err != nil {
						t.Fatal(err)
					}
					if !jointEqual(a, b) {
						t.Fatalf("%v n%d before%v after%v", pair, n, a, b)
					}
				}
			}
		})
	}
}

func TestHIRBitMutationLoopCarriesCurrentValue(t *testing.T) {
	const source = `fun f(a:u16,n:u8)->u16 {var i:u8=0
 var sum:u16=0
 while i<n {sum=sum+a
 a.0=1
 i=i+1}
 return sum}`
	for _, grace := range []bool{false, true} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			m := graceMeasureParse(t, source)
			if grace {
				if _, err := hir.RunGrace(m, 0); err != nil {
					t.Fatal(err)
				}
			}
			raw := mir2.NewVM(hir.LowerModule(m))
			result, err := raw.Call("f", []mir2.Value{{I: 2}, {I: n}})
			if err != nil {
				t.Fatal(err)
			}
			want := int64(0)
			if n > 0 {
				want = 2 + (n-1)*3
			}
			if result[0].I != want {
				t.Fatalf("grace%v n%d got%v want%d", grace, n, result, want)
			}
		}
	}
}

func TestHIRNestedBitAssignmentRejectedBeforeLowering(t *testing.T) {
	source := `fun f(a:u16)->u16 {a.0.0=1
 return a}`
	m, err := nanz.Parse(source, "nested-bit.nanz")
	if err != nil {
		return
	}
	if err = hir.Verify(m); err == nil || !strings.Contains(err.Error(), "addressable scalar base") {
		t.Fatalf("nested bit assignment accepted: %v", err)
	}
}
