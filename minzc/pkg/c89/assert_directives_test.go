package c89

import (
	"strings"
	"testing"
)

func TestAssertTrailingCommentAndMalformed(t *testing.T) {
	for _, suffix := range []string{"", " via z80 // explanation", " via mir2 // explanation"} {
		m, err := Compile("unsigned char f(void) { return 42; }\n// assert f() == 42"+suffix, "test.c")
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Asserts) != 1 || m.Asserts[0].Line != 2 || m.Asserts[0].Expected != 42 {
			t.Fatalf("asserts: %+v", m.Asserts)
		}
	}
	for _, directive := range []string{"// assert", "// assert f(nope) == 42", "// assert f() = 42", "// assert f() == 42 via typo", "// assert f() == 999999999999999999999"} {
		_, err := Compile("unsigned char f(void) { return 42; }\n"+directive, "test.c")
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("%s: %v", directive, err)
		}
	}
}
