package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
)

func compileSpeedCopyC(t *testing.T, src string, opts Options) Steps {
	t.Helper()
	h, err := c89.CompileWithOpts(src, "sc.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// k is a parameterless one-block export (a narrow copy main keeps); h is a
// speed candidate whose copy the static gate refuses. The refusal must
// leave main's product: k inlined, the same assembly as with speed copies
// off. Before, the refusal shipped the product with every copy disabled
// (CALL k, PUSH/POP, 39 B instead of 33 B).
func TestSpeedCopyRefusalKeepsNarrowCopies(t *testing.T) {
	src := `unsigned char G;
unsigned char k(void) { return G + 1; }
unsigned char h(unsigned char p, unsigned char q) { return p < q ? q : p; }
unsigned char f(unsigned char a, unsigned char b) { G = a; unsigned char t = k(); G = 5; return t + h(a, b); }`
	s := compileSpeedCopyC(t, src, DefaultOptions())
	if strings.Contains(s.Assembly, "CALL k") {
		t.Fatalf("narrow copy of k lost:\n%s", s.Assembly)
	}
	ref := DefaultOptions()
	ref.disableSpeedCopies = true
	if r := compileSpeedCopyC(t, src, ref); r.Assembly != s.Assembly {
		t.Fatalf("refused speed copy changed the product:\n--- speed off\n%s\n--- default\n%s", r.Assembly, s.Assembly)
	}
	for _, x := range [][3]int64{{1, 2, 4}, {9, 2, 19}, {200, 7, 145}} {
		v, e := mir2.NewVM(s.MIR2Module).Call("f", []mir2.Value{{I: x[0]}, {I: x[1]}})
		if e != nil || v[0].I != x[2] {
			t.Fatal(x, v, e)
		}
	}
}

// One module, two speed candidates: in_interval (kept: e1 gets faster) and
// pick, whose caller e2 also calls g and cannot be scored (refused). The
// refusal of pick must not veto in_interval's copy.
func TestSpeedCopyDecidedPerCandidate(t *testing.T) {
	src := `unsigned char a, b, r1, r2;
unsigned char g(unsigned char x) { unsigned char s = 0; while (x) { s += x; x >>= 1; } return s; }
unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low + 31;
    return value >= low && value <= high;
}
unsigned char pick(unsigned char p, unsigned char q) { return p < q ? q : p; }
void e1(void) { r1 = in_interval(a, b); }
void e2(void) { r2 = g(a) + pick(a, b) + g(b); }`
	s := compileSpeedCopyC(t, src, DefaultOptions())
	if strings.Contains(s.Assembly, "CALL in_interval") || !strings.Contains(s.Assembly, "CALL pick") {
		t.Fatalf("want in_interval copied and pick called:\n%s", s.Assembly)
	}
	size := DefaultOptions()
	size.OptSize = true
	if z := compileSpeedCopyC(t, src, size); !strings.Contains(z.Assembly, "CALL in_interval") {
		t.Fatalf("--Osize copied in_interval:\n%s", z.Assembly)
	}
	vm := mir2.NewVM(s.MIR2Module)
	for v := int64(0); v < 256; v += 5 {
		for low := int64(0); low < 256; low += 3 {
			got, e := vm.Call("in_interval", []mir2.Value{{I: v}, {I: low}})
			want := int64(0)
			if v >= low && v <= int64(byte(low+31)) {
				want = 1
			}
			if e != nil || got[0].I != want {
				t.Fatal(v, low, got, e)
			}
		}
	}
}
