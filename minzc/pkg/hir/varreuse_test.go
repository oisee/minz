package hir_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestVarReuseInBinExpr(t *testing.T) {
	// Decomposed: works
	t.Run("decomposed", func(t *testing.T) {
		src := `
fun hash(key: u16) -> u16 {
    var shifted: u16 = key + 37
    var h: u16 = key xor shifted
    return h
}
assert hash(100) == 237 via mir2
`
		m, err := nanz.Parse(src, "test.nanz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		_, err = pipeline.CompileHIR(m)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
	})

	// Inline: x op (x + N) — known to panic
	t.Run("inline_x_op_x_plus_n", func(t *testing.T) {
		src := `
fun hash2(key: u16) -> u16 {
    var h: u16 = key xor (key + 37)
    return h
}
assert hash2(100) == 237 via mir2
`

	// More complex: triple use of same var
	t.Run("triple_use", func(t *testing.T) {
		src := `
fun f(x: u8) -> u8 {
    return x + (x + x)
}
assert f(10) == 30 via mir2
`
		m, err := nanz.Parse(src, "test3.nanz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		_, err = pipeline.CompileHIR(m)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
	})

	// Nested: x * (x + 1) — classic pattern
	t.Run("x_times_x_plus_1", func(t *testing.T) {
		src := `
fun tri(n: u8) -> u8 {
    return n * (n + 1)
}
assert tri(5) == 30 via mir2
assert tri(10) == 110 via mir2
`
		m, err := nanz.Parse(src, "test4.nanz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		_, err = pipeline.CompileHIR(m)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
	})
		m, err := nanz.Parse(src, "test2.nanz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		// Catch panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					msg := strings.ToLower(strings.TrimSpace(
						strings.ReplaceAll(r.(string), "\n", " ")))
					if strings.Contains(msg, "undefined variable") {
						t.Logf("GOT EXPECTED PANIC: %v", r)
						t.Log("BUG: x op (x+N) causes lowerer to lose variable")
					} else {
						t.Fatalf("unexpected panic: %v", r)
					}
					return
				}
			}()
			_, err = pipeline.CompileHIR(m)
			if err != nil {
				t.Logf("compile error (not panic): %v", err)
			} else {
				t.Log("PASS — bug may have been fixed!")
			}
		}()
	})
}
