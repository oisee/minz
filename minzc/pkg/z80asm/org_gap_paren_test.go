package z80asm

import (
	"bytes"
	"testing"
)

func assembleOK(t *testing.T, src string) *Result {
	t.Helper()
	r, err := NewAssembler().AssembleString(src)
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("assemble: %v %v\n%s", err, r.Errors, src)
	}
	return r
}

// The raw image is contiguous from the origin: a label's bytes sit at
// (label - origin), also after an ORG that skips forward.
func TestForwardORGPadsRawImage(t *testing.T) {
	for _, c := range []struct {
		name, src string
		origin    int
		size      int
	}{
		{"first ORG is the default address", "ORG $8000\nLD A,1\nRET\nORG $8100\ntab: DB 1,2,3,4\n", 0x8000, 0x104},
		{"first ORG elsewhere", "ORG $9000\nNOP\nORG $9010\ntab: DB 1,2,3,4\n", 0x9000, 0x14},
		{"code before the first ORG", "NOP\nORG $8020\ntab: DB 1,2,3,4\n", 0x8000, 0x24},
		{"two gaps", "ORG $8000\nNOP\nORG $8008\nNOP\nORG $8010\ntab: DB 1,2,3,4\n", 0x8000, 0x14},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := assembleOK(t, c.src)
			if r.Origin != c.origin || len(r.Binary) != c.size {
				t.Fatalf("origin %#x size %#x, want %#x %#x", r.Origin, len(r.Binary), c.origin, c.size)
			}
			tab, ok := r.Symbols["tab"]
			if !ok {
				tab = r.Symbols["TAB"]
			}
			at := tab - r.Origin
			if at < 0 || at+4 != len(r.Binary) || !bytes.Equal(r.Binary[at:], []byte{1, 2, 3, 4}) {
				t.Fatalf("tab=%#x is not where its bytes are: % x", tab, r.Binary)
			}
		})
	}
}

// A single ORG, and no ORG at all, produce exactly the emitted bytes.
func TestSingleORGAddsNoPadding(t *testing.T) {
	for _, src := range []string{"ORG $C000\nDB 1,2\n", "DB 1,2\n", "ORG $8000\nDB 1,2\n"} {
		if r := assembleOK(t, src); !bytes.Equal(r.Binary, []byte{1, 2}) {
			t.Fatalf("%q: % x", src, r.Binary)
		}
	}
}

func TestParenthesesInExpressions(t *testing.T) {
	r := assembleOK(t, `    ORG $8000
    NOP
    DS (4-1)*2
    LD A,(2+3)*2
    LD BC,((1+2)*(3+4))<<1
    DB (1+2)&255, 2*(3+4)-1, ~(1|2)&255, (tab-$8000)^L
    DS (16-$)&15
tab: DB 9
`)
	want := []byte{
		0x00,
		0, 0, 0, 0, 0, 0,
		0x3E, 10,
		0x01, 42, 0,
		3, 13, 0xFC, 0x10,
		9,
	}
	if !bytes.Equal(r.Binary, want) {
		t.Fatalf("got  % x\nwant % x", r.Binary, want)
	}
}

// A whole operand in parentheses stays a memory reference.
func TestParenthesisedOperandIsStillMemory(t *testing.T) {
	r := assembleOK(t, "ORG $8000\nLD A,(tab)\nLD HL,(tab+1)\nLD A,(1+2)*2\ntab: DB 0\n")
	want := []byte{0x3A, 0x08, 0x80, 0x2A, 0x09, 0x80, 0x3E, 6, 0}
	if !bytes.Equal(r.Binary, want) {
		t.Fatalf("got  % x\nwant % x", r.Binary, want)
	}
}

func TestUnbalancedParenthesesAreErrors(t *testing.T) {
	for _, src := range []string{"DB (1+2\n", "DB 1+2)\n", "DB ((1)\n", "DB ()+1\n"} {
		r, err := NewAssembler().AssembleString(src)
		if err == nil && (r == nil || len(r.Errors) == 0) {
			t.Fatalf("%q assembled: % x", src, r.Binary)
		}
	}
}
