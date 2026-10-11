package lizp_test

import (
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

// Peek/poke are byte memory operations, and progn returns its final value
// after running the prefix. The expected values follow byte truncation and
// ordered Lisp evaluation, independently of either execution backend.
func TestPeekPokeReturnPrognVMZ80(t *testing.T) {
	src := `(defglobal cell u8)
 (defun test ((x u8)) -> u16
 (poke (addr cell) 99)
 (return (cond
 ((= x 0) (progn (poke (addr cell) 300) (+ (peek (addr cell)) 256)))
 (t (progn (poke (addr cell) x) (peek (addr cell)))))))
 (assert test 0 == 300)
 (assert test 1 == 1)
 (assert test 255 == 255)`
	h, err := lizp.Compile(src, "memory_progn")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIR(h); err != nil {
		t.Fatal(err)
	}
}
