package lizp_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/pipeline"
)

// Language Book v7 §21.4: Lizp sugar must produce the same typed HIR as
// Nanz/Lanz. A declared parameter is a value, not an unbound truth alias.
func TestTruthAliasBindingsVMZ80(t *testing.T) {
	src := `(defun parameter ((t u8) (__lizp_alias_t_1 u8)) -> u8 (return (+ t __lizp_alias_t_1)))
(defun local ((x u8)) -> u8 (let ((t u8 x)) (return (+ t 3))))
(defun nested ((t u8)) -> u8 (let ((nil u8 t)) (return (+ nil 5))))
(defun default_clause ((x u8)) -> u8 (return (cond ((== x 0) 7) (t 9))))
(defun sawtooth ((t u8) (period u8)) -> u8 (return (quotient (* (modulo t period) 255) period)))
(defun live_bool ((ch u8) (shift u8)) -> u8 (return (+ ch (and (>= ch 65) (<= ch 90)))))
`
	for _, x := range []int{0, 1, 9, 10, 64, 65, 89, 90, 91, 255} {
		src += fmt.Sprintf("(assert parameter %d 17 == %d)\n", x, (x+17)&255)
		src += fmt.Sprintf("(assert local %d == %d)\n", x, (x+3)&255)
		src += fmt.Sprintf("(assert nested %d == %d)\n", x, (x+5)&255)
		want := 9
		if x == 0 {
			want = 7
		}
		src += fmt.Sprintf("(assert default_clause %d == %d)\n", x, want)
		truth := 0
		if x >= 65 && x <= 90 {
			truth = 1
		}
		src += fmt.Sprintf("(assert live_bool %d 3 == %d)\n", x, (x+truth)&255)
		for _, p := range []int{1, 2, 10, 255} {
			// Operands are u8: the product wraps before unsigned quotient.
			src += fmt.Sprintf("(assert sawtooth %d %d == %d)\n", x, p, ((x%p)*255&255)/p)
		}
	}
	m, err := lizp.Compile(src, "truth_binding")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(steps.HIR, "__lizp_alias_t_2") {
		t.Fatal("generated alias must avoid the explicit parameter name")
	}
	t.Logf("%d independent assertions verified by MIR2 VM and assembled Z80", len(m.Asserts))
}

// Both boolean operands must be canonical bytes, irrespective of whether a
// predicate is held in flags or is assigned an ordinary byte register by PBQP.
func TestComparisonBooleanALUVMZ80(t *testing.T) {
	ops := []string{"==", "!=", "<", "<=", ">", ">="}
	for i, first := range ops {
		for j, second := range ops {
			for _, alu := range []string{"&", "|", "^"} {
				var src strings.Builder
				name := fmt.Sprintf("truth_%d_%d_%d", i, j, int(alu[0]))
				fmt.Fprintf(&src, "(defun %s ((a u8) (b u8)) -> u8 (return (%s (%s a 65) (%s b 90))))\n", name, alu, first, second)
				for _, a := range []int{0, 64, 65, 66, 255} {
					for _, b := range []int{0, 89, 90, 91, 255} {
						l, r := comparisonTruth(first, a, 65), comparisonTruth(second, b, 90)
						want := l & r
						if alu == "|" {
							want = l | r
						}
						if alu == "^" {
							want = l ^ r
						}
						fmt.Fprintf(&src, "(assert %s %d %d == %d)\n", name, a, b, want)
					}
				}
				m, err := lizp.Compile(src.String(), name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = pipeline.CompileHIRSteps(m); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			}
		}
	}
	t.Log("2700 independent assertions verified by MIR2 VM and assembled Z80")
}

func TestCaesarExampleBoundaryVMZ80(t *testing.T) {
	data, err := os.ReadFile("../../../examples/lizp/crypto.lizp")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, ch := range []int{0, 48, 64, 65, 66, 89, 90, 91, 255} {
		for _, shift := range []int{0, 1, 3, 25, 26, 255} {
			want := ch
			if ch >= 65 && ch <= 90 {
				want = 65 + ((ch-65+shift)&255)%26
			}
			src += fmt.Sprintf("(assert caesar_encrypt %d %d == %d)\n", ch, shift, want)
		}
	}
	m, err := lizp.Compile(src, "crypto_boundaries")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIRSteps(m); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d source assertions verified by MIR2 VM and assembled Z80", len(m.Asserts))
}

func comparisonTruth(op string, a, b int) int {
	var truth bool
	switch op {
	case "==":
		truth = a == b
	case "!=":
		truth = a != b
	case "<":
		truth = a < b
	case "<=":
		truth = a <= b
	case ">":
		truth = a > b
	case ">=":
		truth = a >= b
	}
	if truth {
		return 1
	}
	return 0
}
