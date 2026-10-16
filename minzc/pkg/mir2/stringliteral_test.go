package mir2

import (
	"bytes"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
	"time"
)

// TestEmitStringLiteral covers the DB-payload encoder, including the case that
// used to hang: a double quote can't live inside DB "...", and the encoder
// previously broke out of its printable run without advancing the cursor, so
// the outer loop re-entered on the same byte forever.
func TestEmitStringLiteral(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"printable", "Hello", `"Hello"`},
		{"empty", "", ``},
		{"control only", "\x00\x01", `0, 1`},
		{"leading quote", `"x`, `34, "x"`},
		{"trailing quote", `x"`, `"x", 34`},
		{"embedded quote", `say "hi" ok`, `"say ", 34, "hi", 34, " ok"`},
		{"only quotes", `""`, `34, 34`},
		{"mixed control", "a\nb", `"a", 10, "b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			done := make(chan struct{})
			go func() {
				emitStringLiteral(&sb, []byte(tc.in))
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("emitStringLiteral(%q) did not terminate", tc.in)
			}
			if got := sb.String(); got != tc.want {
				t.Errorf("emitStringLiteral(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestStringLiteralAssembledBytes(t *testing.T) {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	for _, data := range [][]byte{all, []byte(";#"), []byte(`\n\r\t\0\\`), []byte(`tail\`)} {
		var text strings.Builder
		emitStringLiteral(&text, data)
		result, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nDB " + text.String())
		if err != nil || len(result.Errors) > 0 {
			t.Fatal(err, result.Errors)
		}
		if !bytes.Equal(result.Binary, data) {
			t.Fatalf("string bytes changed: got %x want %x ASM %s", result.Binary, data, text.String())
		}
	}
}
