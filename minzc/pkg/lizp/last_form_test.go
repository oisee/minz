package lizp_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestLastFormReturnContracts(t *testing.T) {
	src := `(defglobal effects u8 0)
 (defun nested ((n u8)) -> u8
   (setq effects (1+ effects))
   (let ((x u8 (+ n 1)))
     (progn (setq effects (1+ effects))
       (cond ((= n 0) (return 7)) (t (+ x 2))))))
 (define (scheme (n u8)) -> u8 (+ n 3))
 (defun early ((n u8)) -> u8 (if (= n 0) (return 9)) (+ n 1))
 (defun mixed ((n u8)) -> u8 (if (= n 0) (return 8) (+ n 2)))`
	h, e := lizp.Compile(src, "last-form")
	if e != nil {
		t.Fatal(e)
	}
	if e = hir.Verify(h); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 256; n++ {
		vm := mir2.NewVM(hir.LowerModule(h))
		for _, fn := range []string{"nested", "scheme", "early", "mixed"} {
			want := (n + 3) & 255
			if fn == "nested" && n == 0 {
				want = 7
			}
			if fn == "early" {
				want = (n + 1) & 255
				if n == 0 {
					want = 9
				}
			}
			if fn == "mixed" {
				want = (n + 2) & 255
				if n == 0 {
					want = 8
				}
			}
			got, e := vm.Call(fn, []mir2.Value{{I: int64(n)}})
			if e != nil || len(got) != 1 || got[0].I != int64(want) {
				t.Fatal(fn, n, got, want, e)
			}
		}
		if vm.ReadHeap(vm.GlobalAddr("effects"), 1)[0] != 2 {
			t.Fatal("prefix effects must execute exactly once")
		}
	}
}

func TestLastFormRejectsMissingValues(t *testing.T) {
	for _, src := range []string{
		`(defun bad () -> u8)`,
		`(defun bad () -> u8 (return))`,
		`(defun bad ((n u8)) -> u8 (if (= n 0) 1))`,
		`(defun bad () -> u8 (progn))`,
		`(defglobal x u8 0) (defun bad () -> u8 (setq x 1))`,
		`(defun sink () -> void (return)) (defun bad () -> u8 (call sink))`,
		`(defun bad ((n u8)) -> u8 (if (= n 0) (return)) n)`,
	} {
		if _, e := lizp.Compile(src, "missing-value"); e == nil {
			t.Fatalf("accepted missing value: %s", src)
		}
	}
}
