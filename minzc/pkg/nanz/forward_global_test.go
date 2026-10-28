package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// forwardGlobalCase is one program whose functions reference globals declared
// later in the file.
type forwardGlobalCase struct {
	src  string
	fn   string
	want int64
}

// forwardGlobalCases are programs whose functions reference globals that are
// declared later in the file. The reference must use the declared type, not the
// parse-time u8 fallback, in both the let and the direct-assignment spelling.
func forwardGlobalCases() map[string]forwardGlobalCase {
	return map[string]forwardGlobalCase{
		"u16 let": {
			src: `
global r: u16
fun f() -> u16 { let y = g + 1
 r = y
 return r }
global g: u16 = 1000
`, fn: "f", want: 1001,
		},
		"u16 direct": {
			src: `
global r: u16
fun f() -> u16 { r = g + 1
 return r }
global g: u16 = 1000
`, fn: "f", want: 1001,
		},
		"i16 let": {
			src: `
global r: i16
fun f() -> i16 { let y = g - 1
 r = y
 return r }
global g: i16 = -1000
`, fn: "f", want: -1001,
		},
		"i16 direct": {
			src: `
global r: i16
fun f() -> i16 { r = g - 1
 return r }
global g: i16 = -1000
`, fn: "f", want: -1001,
		},
		"u32 widened": {
			src: `
global r: u32
fun f() -> u32 { r = g + 1
 return r }
global g: u32 = 70000
`, fn: "f", want: 70001,
		},
		"comparison uses declared width": {
			src: `
fun f() -> u8 { if g > 300 { return 1 }
 return 0 }
global g: u16 = 1000
`, fn: "f", want: 1,
		},
		"array element": {
			src: `
fun f() -> u16 { return table[1] }
global table: [u16; 3] = [10, 4660, 30]
`, fn: "f", want: 4660,
		},
		"array store": {
			src: `
fun f() -> u16 { table[2] = 43981
 return table[2] }
global table: [u16; 3] = [10, 4660, 30]
`, fn: "f", want: 43981,
		},
		"struct field": {
			src: `
struct Cell { v: u16, t: u8 }
fun f() -> u16 { return cell.v }
global cell: Cell = Cell{v: 5000, t: 7}
`, fn: "f", want: 5000,
		},
		"address-of": {
			src: `
fun f() -> u16 { let p: ^u16 = &g
 return p^ }
global g: u16 = 900
`, fn: "f", want: 900,
		},
		"const": {
			src: `
fun f() -> u16 { return limit + 1 }
const limit: u16 = 65500
`, fn: "f", want: 65501,
		},
	}
}

// lowerForwardModule lowers hm, turning a lowerer panic (a malformed module
// that reached pkg/hir) into a test failure instead of aborting the package.
func lowerForwardModule(t *testing.T, hm *hir.Module) *mir2.Module {
	t.Helper()
	var m *mir2.Module
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LowerModule panicked: %v", r)
			m = nil
		}
	}()
	m = hir.LowerModule(hm)
	return m
}

// TestForwardGlobalReferencesUseDeclaredType checks that a global declared after
// its use is typed by its declaration, not by the u8 parse-time default.
func TestForwardGlobalReferencesUseDeclaredType(t *testing.T) {
	for name, tc := range forwardGlobalCases() {
		t.Run(name, func(t *testing.T) {
			hm, err := nanz.Parse(tc.src, "forward_global.nanz")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			m := lowerForwardModule(t, hm)
			if m == nil {
				t.Fatal("no module")
			}
			if err := mir2.Verify(m); err != nil {
				t.Fatalf("verify: %v", err)
			}
			got, err := mir2.NewVM(m).Call(tc.fn, nil)
			if err != nil {
				t.Fatalf("VM: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("%s returned %d values, want 1", tc.fn, len(got))
			}
			fn := m.FuncByName(tc.fn)
			if fn == nil || len(fn.Contract.Returns) != 1 {
				t.Fatalf("%s has no single return contract", tc.fn)
			}
			if !hir.AssertValueMatches(got[0].I, tc.want, fn.Contract.Returns[0].Ty) {
				t.Fatalf("%s = %s, want %d", tc.fn, hir.FormatAssertValue(got[0].I, fn.Contract.Returns[0].Ty), tc.want)
			}
		})
	}
}

// forwardGlobalOrderSrc and forwardGlobalOrderSrcReordered are the same program
// with the functions moved after the globals. Declaration order of a global
// relative to its users must not change the generated code.
const forwardGlobalOrderSrc = `global r: u16
fun f() -> u16 { let y = g + 1
 r = y
 return r }
global g: u16 = 1000
global acc: u16
fun h() -> u16 { acc = g * 2
 return acc }
`

const forwardGlobalOrderSrcReordered = `global r: u16
global g: u16 = 1000
global acc: u16
fun f() -> u16 { let y = g + 1
 r = y
 return r }
fun h() -> u16 { acc = g * 2
 return acc }
`

// TestForwardGlobalDeclarationOrderIndependent checks the order-independence
// property: moving the function declarations after the globals leaves the
// lowered module unchanged.
func TestForwardGlobalDeclarationOrderIndependent(t *testing.T) {
	first, err := nanz.Parse(forwardGlobalOrderSrc, "order.nanz")
	if err != nil {
		t.Fatalf("parse order A: %v", err)
	}
	second, err := nanz.Parse(forwardGlobalOrderSrcReordered, "order.nanz")
	if err != nil {
		t.Fatalf("parse order B: %v", err)
	}
	a := hir.LowerModule(first).Dump()
	b := hir.LowerModule(second).Dump()
	if a != b {
		t.Fatalf("lowered modules differ between declaration orders:\n--- functions first ---\n%s\n--- globals first ---\n%s", a, b)
	}
}

// forwardGlobalErrCase is one source that must be rejected by the parser.
type forwardGlobalErrCase struct {
	src     string
	wantErr string
}

// TestForwardGlobalUnknownNameIsDiagnostic checks that an identifier that is
// unknown after the forward-global pre-pass is rejected instead of silently
// becoming a u8 variable reference.
func TestForwardGlobalUnknownNameIsDiagnostic(t *testing.T) {
	cases := map[string]forwardGlobalErrCase{
		"unknown variable in expression": {
			src:     `fun f() -> u8 { return nosuch + 1 }`,
			wantErr: "unknown variable",
		},
		"unknown variable in initializer": {
			src:     `fun f() { let x: u8 = nosuch }`,
			wantErr: "unknown variable",
		},
		// Only a bare name directly before `=` may be an implicit local
		// declaration; an unknown base of a larger lvalue is still rejected.
		"unknown array in assignment target": {
			src:     `fun f() { arr[0] = 5 }`,
			wantErr: "unknown variable",
		},
		// A forward global is typed by its declaration, but the type itself
		// must still be declared before the first use, as everywhere else.
		"forward global with later-declared type": {
			src: `
fun f() -> u16 { return cell.v }
global cell: Cell
struct Cell { v: u16 }
`,
			wantErr: `global "cell" is declared later (line 3) and its type is not visible here`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.Parse(tc.src, "unknown_name.nanz")
			if err == nil {
				t.Fatalf("parse succeeded, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
