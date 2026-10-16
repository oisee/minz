package c89

import (
	"strings"
	"testing"
)

func TestCExtensionsPreserveNonCodeTokens(t *testing.T) {
	// C translation phases 2/3 and 6.4.5: apparent extension spellings inside
	// comments, string/character literals and header names are not code tokens.
	src := "/* _BitInt(4) 1'234 [[nodiscard]] */\n" +
		"// continued comment \\\n_BitInt(4) 1'234 [[nodiscard]]\n" +
		"#include <a'b.h>\n" +
		"#include<a'b.h>\n" +
		"# /* gap */ include /* gap */ <a'b.h>\n" +
		"#define SPELL a'b'\n" +
		"char *s=\"a'b _BitInt(4) [[nodiscard]]\"; char c='A';\n"
	got := stripDigitSeparators(preprocessBitInt(stripC23Attributes(src)))
	if got != src {
		t.Fatalf("protected source changed:\n%s", got)
	}
}

func TestCExtensionsPreserveSourceLines(t *testing.T) {
	src := "[[\nnodiscard\n]] _BitInt(\n8\n) value(void){return 7;}\n"
	got := preprocessBitInt(stripC23Attributes(src))
	if strings.Count(got, "\n") != strings.Count(src, "\n") || !strings.Contains(got, "uint8_t") {
		t.Fatalf("source lines changed: %q", got)
	}
}

func TestCExtensionsStillTransformCode(t *testing.T) {
	src := "[[deprecated(\"old\")]] _BitInt(8) x=1'234;\n"
	want := " uint8_t x=1234;\n"
	if got := stripDigitSeparators(preprocessBitInt(stripC23Attributes(src))); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
