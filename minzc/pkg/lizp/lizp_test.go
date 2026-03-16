package lizp

import (
	"testing"

	"github.com/minz/minzc/pkg/pipeline"
)

func TestCompileMinimal(t *testing.T) {
	src := `(defun main () -> void (return))`
	mod, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if mod == nil {
		t.Fatal("expected non-nil module")
	}
}

func TestCompileDefun(t *testing.T) {
	src := `
(defun add ((a u8) (b u8)) -> u8
  (return (+ a b)))
`
	mod, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if mod == nil {
		t.Fatal("expected non-nil module")
	}
}

func TestCompileLet(t *testing.T) {
	src := `
(defun main () -> void
  (let ((x u8 42))
    (return)))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileCond(t *testing.T) {
	src := `
(defun classify ((n u8)) -> u8
  (cond
    ((= n 0) 0)
    ((< n 10) 1)
    (t 2)))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileDotimes(t *testing.T) {
	src := `
(defglobal acc u8 0)
(defun main () -> void
  (dotimes (i 5)
    (set acc (1+ acc)))
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileDefstruct(t *testing.T) {
	src := `
(defstruct Point (x u8) (y u8))
(defun main () -> void (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileDefglobal(t *testing.T) {
	src := `
(defglobal counter u8 0)
(defun main () -> void (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileWhenUnless(t *testing.T) {
	src := `
(defglobal x u8 0)
(defun main () -> void
  (when (= x 0)
    (set x 1))
  (unless (= x 0)
    (set x 0))
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileHexBinary(t *testing.T) {
	src := `
(defglobal val u8 0)
(defun main () -> void
  (set val #xFF)
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileSetq(t *testing.T) {
	src := `
(defglobal x u8 0)
(defun main () -> void
  (setq x 42)
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileProgn(t *testing.T) {
	src := `
(defglobal a u8 0)
(defglobal b u8 0)
(defun main () -> void
  (progn
    (set a 1)
    (set b 2))
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileBoolOps(t *testing.T) {
	src := `
(defun check ((a u8) (b u8)) -> u8
  (return (and a b)))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileDefextern(t *testing.T) {
	src := `
(defextern putchar ((c u8)) void)
(defun main () -> void
  (call putchar 65)
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileDefmacro(t *testing.T) {
	src := `
(defmacro inc! (x) (set x (+ x 1)))
(defglobal counter u8 0)
(defun main () -> void
  (inc! counter)
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileThreadFirst(t *testing.T) {
	src := `
(defun double ((x u8)) -> u8 (return (+ x x)))
(defun inc ((x u8)) -> u8 (return (+ x 1)))
(defun main () -> void
  (var r u8 (-> 3 (double) (inc)))
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompileThreadLast(t *testing.T) {
	src := `
(defun sub ((a u8) (b u8)) -> u8 (return (- a b)))
(defun main () -> void
  (var r u8 (->> 1 (sub 10)))
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestMacroExpansionRecursive(t *testing.T) {
	// Macro that uses another Lizp form (setq)
	src := `
(defmacro zero! (x) (setq x 0))
(defglobal val u8 42)
(defun main () -> void
  (zero! val)
  (return))
`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestAssert_Parse(t *testing.T) {
	src := `
(defun double ((x u8)) -> u8
  (return (+ x x)))

(assert double 5 == 10)
(assert double 0 == 0)
`
	mod, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(mod.Asserts) != 2 {
		t.Fatalf("expected 2 asserts, got %d", len(mod.Asserts))
	}
	a := mod.Asserts[0]
	if a.FuncName != "double" {
		t.Errorf("func = %q, want double", a.FuncName)
	}
	if len(a.Args) != 1 || a.Args[0] != 5 {
		t.Errorf("args = %v, want [5]", a.Args)
	}
	if a.Expected != 10 {
		t.Errorf("expected = %d, want 10", a.Expected)
	}
}

func TestAssert_HexLiteral(t *testing.T) {
	src := `
(defun id ((x u8)) -> u8
  (return x))

(assert id #xFF == 255)
`
	mod, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(mod.Asserts) != 1 {
		t.Fatalf("expected 1 assert, got %d", len(mod.Asserts))
	}
	if mod.Asserts[0].Args[0] != 255 {
		t.Errorf("arg = %d, want 255 (#xFF)", mod.Asserts[0].Args[0])
	}
}

func TestAssert_E2E(t *testing.T) {
	src := `
(defun double ((x u8)) -> u8
  (return (+ x x)))

(assert double 0 == 0)
(assert double 5 == 10)
(assert double 127 == 254)
`
	mod, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = pipeline.CompileHIR(mod)
	if err != nil {
		t.Fatalf("pipeline (asserts should pass): %v", err)
	}
}

func TestFuncall(t *testing.T) {
	// Lizp funcall + #'function-reference (Common Lisp style)
	src := `
(defun double ((x u8)) -> u8
  (return (+ x x)))

(defun apply ((f ptr) (x u8)) -> u8
  (return (funcall f x)))

(defun test-fp () -> u8
  (return (apply #'double 5)))

(assert test-fp == 10 via mir2)
`
	mod, err := Compile(src, "test_funcall")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = pipeline.CompileHIR(mod)
	if err != nil {
		t.Fatalf("funcall pipeline: %v", err)
	}
}
