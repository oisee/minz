package mir2

import "testing"

// A body spliced in after Z80Codegen (checked local tables, conditional
// layouts) must write nothing beyond the clobber set its header declares to
// the callers narrowed against it.
func TestSpliceKeepsClobbers(t *testing.T) {
	m := &Module{}
	f := exactLeaf(m, "leaf", &Return{Ty: TyU8, Class: ClassAcc})
	product := func(clobbers, body string) string {
		return "; fun leaf() -> u8 = A ; clobbers: " + clobbers + "\n; [trace] backend=checked-local-table\nleaf:\n" + body + "\n; fun next() ; clobbers: A\nnext:\n    RET\n"
	}
	for _, tc := range []struct {
		name, clobbers, body string
		want                 bool
	}{
		{"subset", "A, C, F", "    LD A, C\n    ADD A, 7\n    RET", true},
		{"same pair, other half", "A, C, F", "    LD B, A\n    ADD A, B\n    RET", false},
		{"extra pair", "A, F", "    LD L, A\n    ADD A, L\n    RET", false},
		{"extra index half", "A, F", "    LD IXH, A\n    ADD A, IXH\n    RET", false},
		{"alternate bank", "A, F", "    EXX\n    LD L, A\n    EXX\n    RET", false},
		{"unprovable", "A, F", "    CALL other\n    RET", false},
		{"full ABI constrains nothing", "A, B, BC, C, D, DE, E, F, H, HL, IX, IXH, IXL, IY, IYH, IYL, L", "    CALL other\n    RET", true},
	} {
		if got := SpliceKeepsClobbers(f, product(tc.clobbers, tc.body)); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if SpliceKeepsClobbers(f, "leaf:\n    RET\n") {
		t.Error("accepted a body without a header")
	}
}
