package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

const auditAtoiSource = `fun atoi(str: ^u8) -> u16 {
 var result: u16 = 0
 var ptr: ^u8 = str
 while ptr^ >= 48 {
  if ptr^ > 57 { return result }
  result = result * 10 + ptr^ - 48
  ptr = ptr + 1
 }
 return result
}`

func TestGraceAtoiLoopLoadRegression(t *testing.T) {
	for _, grace := range []bool{false, true} {
		s, err := CompileHIRSteps(graceMeasureParse(t, auditAtoiSource), Options{UseGrace: grace})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			input string
			want  int64
		}{{"", 0}, {"0", 0}, {"12345", 12345}, {"12x34", 12}, {"999", 999}, {"65536", 0}} {
			vm := mir2.NewVM(s.MIR2Module)
			vm.EnsureHeap(0x4100)
			vm.WriteHeapSlice(0x4000, append([]byte(c.input), 0))
			got, err := vm.Call("atoi", []mir2.Value{{I: 0x4000}})
			if err != nil || len(got) != 1 || got[0].I != c.want {
				t.Fatalf("grace=%v %q: %v %v", grace, c.input, got, err)
			}
		}
	}
}

func TestGraceTailRecursionSuccessorParameters(t *testing.T) {
	src := `fun fib_tail(n: u8, a: u8, b: u8) -> u8 {
 if n == 0 { return a }
 return fib_tail(n - 1, b, a + b)
}
fun fib(n: u8) -> u8 { return fib_tail(n, 0, 1) }`
	for _, grace := range []bool{false, true} {
		s, err := CompileHIRSteps(graceMeasureParse(t, src), Options{UseGrace: grace})
		if err != nil {
			t.Fatal(err)
		}
		vm := mir2.NewVM(s.MIR2Module)
		a, b := int64(0), int64(1)
		for n := 0; n <= 30; n++ {
			got, err := vm.Call("fib", []mir2.Value{{I: int64(n)}})
			if err != nil || len(got) != 1 || got[0].I != a {
				t.Fatalf("grace=%v n%d: %v %v want%d", grace, n, got, err, a)
			}
			a, b = b, (a+b)&255
		}
	}
}

func TestZ80UnsignedDivisionOperandSetup(t *testing.T) {
	sources := map[string]string{
		"gcd": `fun gcd(a: u8, b: u8) -> u8 {
 while b != 0 { let t = b
 b = a % b
 a = t }
 return a
}`,
		"quotient":  `fun quotient(a: u8, b: u8) -> u8 { return a / b }`,
		"remainder": `fun remainder(a: u8, b: u8) -> u8 { return a % b }`,
	}
	for name, src := range sources {
		for _, grace := range []bool{false, true} {
			for _, args := range [][2]int64{{12, 8}, {48, 18}, {0, 8}, {255, 254}, {240, 120}, {17, 17}, {7, 255}, {255, 1}} {
				a, b := args[0], args[1]
				want := a % b
				if name == "quotient" {
					want = a / b
				}
				if name == "gcd" {
					for b != 0 {
						a, b = b, a%b
					}
					want = a
				}
				t.Run(fmt.Sprintf("%s/grace%v/%d,%d", name, grace, args[0], args[1]), func(t *testing.T) {
					measureGraceZ80HIR(t, graceMeasureParse(t, src), name, Options{UseGrace: grace, ContractOpt: true}, args[:], nil, want)
				})
			}
		}
	}
}
