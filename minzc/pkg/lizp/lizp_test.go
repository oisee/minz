package lizp

import (
	"testing"
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
