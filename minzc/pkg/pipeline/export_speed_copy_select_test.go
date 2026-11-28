package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
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

// speedCopyProducts compiles src in the default profile and as the
// reference (speed copies off), counting the candidate compiles of the
// default one.
func speedCopyProducts(t *testing.T, src string) (def, ref Steps, compiles int) {
	t.Helper()
	speedCopyCompileHook = func() { compiles++ }
	def = compileSpeedCopyC(t, src, DefaultOptions())
	speedCopyCompileHook = nil
	off := DefaultOptions()
	off.disableSpeedCopies = true
	ref = compileSpeedCopyC(t, src, off)
	return def, ref, compiles
}

// speedCopyRun calls execute once with the given globals (16-bit stores)
// and returns the 16-bit global result (or the byte r when there is no
// result) and the T-states of the call.
func speedCopyRun(t *testing.T, asm string, globals map[string]int) (int, int) {
	t.Helper()
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	addr := func(name string) (int, bool) {
		if p, ok := a.Symbols[name]; ok {
			return p, true
		}
		p, ok := a.Symbols["v_"+name]
		return p, ok
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	for name, v := range globals {
		p, ok := addr(name)
		if !ok {
			t.Fatalf("missing symbol %s", name)
		}
		mem[p] = byte(v)
		if v > 255 {
			mem[p+1] = byte(v >> 8)
		}
	}
	entry, ok := addr("execute")
	if !ok {
		t.Fatal("missing execute")
	}
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
	start := z.GetCycles()
	for step := 0; step < 1_000_000 && z.GetPC() != 0xf100; step++ {
		z.Step()
	}
	if z.GetPC() != 0xf100 {
		t.Fatalf("execute did not return\n%s", asm)
	}
	cycles := z.GetCycles() - start
	if p, ok := addr("result"); ok {
		return int(mem[p]) | int(mem[p+1])<<8, cycles
	}
	p, ok := addr("r")
	if !ok {
		t.Fatal("missing result")
	}
	return int(mem[p]), cycles
}

// speedCopyFunction returns the "; fun" line and the instructions of a
// function (comments dropped).
func speedCopyFunction(asm, name string) (string, string) {
	header := ""
	for _, line := range strings.Split(asm, "\n") {
		if strings.HasPrefix(line, "; fun "+name+"(") {
			header = line
		}
	}
	return header, strings.Join(asmFunction(asm, name), "\n")
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
// pick, whose caller e2 also calls g: the pre-filter refuses it. That
// refusal must not veto in_interval's copy.
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

// The benchmark case: in_interval is copied into execute after one
// candidate compile, the module grows by 12 bytes (36 -> 48 B in the
// benchmark's count), every input sampled is faster on the Z80, and the
// retained in_interval is the reference's: the same contract line and the
// same instructions.
func TestSpeedCopyInIntervalBenchmark(t *testing.T) {
	src := `unsigned char a, b;
unsigned short result;
unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low + 31;
    return value >= low && value <= high;
}
void execute(void) { result = in_interval(a, b); }`
	def, ref, compiles := speedCopyProducts(t, src)
	if compiles != 1 || strings.Contains(def.Assembly, "CALL in_interval") {
		t.Fatalf("compiles=%d, want in_interval copied after one candidate compile:\n%s", compiles, def.Assembly)
	}
	dh, db := speedCopyFunction(def.Assembly, "in_interval")
	rh, rb := speedCopyFunction(ref.Assembly, "in_interval")
	if dh != rh || db != rb || dh == "" {
		t.Fatalf("retained in_interval differs:\n%s\n%s\n--- reference\n%s\n%s", dh, db, rh, rb)
	}
	defBytes, _ := Assemble(def.Assembly, "z80")
	refBytes, _ := Assemble(ref.Assembly, "z80")
	if len(defBytes)-len(refBytes) != 12 {
		t.Fatalf("growth %d B, want 12", len(defBytes)-len(refBytes))
	}
	var defT, refT int
	for v := 0; v < 256; v += 7 {
		for low := 0; low < 256; low += 11 {
			want := 0
			if v >= low && v <= int(byte(low+31)) {
				want = 1
			}
			g := map[string]int{"a": v, "b": low}
			got, dt := speedCopyRun(t, def.Assembly, g)
			gotRef, rt := speedCopyRun(t, ref.Assembly, g)
			if got != want || gotRef != want || dt >= rt {
				t.Fatalf("value=%d low=%d: %d/%d want %d, T %d vs reference %d", v, low, got, gotRef, want, dt, rt)
			}
			defT += dt
			refT += rt
		}
	}
	t.Logf("in_interval: %d B -> %d B, sampled total %d T -> %d T", len(refBytes), len(defBytes), refT, defT)
}

// Rule 1, retained callee: in_iv loses its only caller, so PFCCO alone
// would re-choose hi (B -> D) and codegen would declare the full clobber
// set (review F5). The plan pins both to the reference: run gets the copy
// and in_iv is exactly the reference's.
func TestSpeedCopyRetainedCalleeKeepsContract(t *testing.T) {
	src := `unsigned char x, r;
unsigned char in_iv(unsigned char v, unsigned char lo, unsigned char hi) {
  if (v < lo) return 0;
  if (v > hi) return 0;
  return 1;
}
unsigned char run(unsigned char a) { return in_iv(a, 10, 20) + 3; }
void execute(void) { r = run(x); }`
	def, ref, _ := speedCopyProducts(t, src)
	if strings.Contains(def.Assembly, "CALL in_iv") || !strings.Contains(ref.Assembly, "CALL in_iv") {
		t.Fatalf("want the copy in run:\n%s", def.Assembly)
	}
	dh, db := speedCopyFunction(def.Assembly, "in_iv")
	rh, rb := speedCopyFunction(ref.Assembly, "in_iv")
	if dh != rh || db != rb || !strings.Contains(rh, "hi: u8 = B") || !strings.HasSuffix(rh, "clobbers: A, F") {
		t.Fatalf("retained in_iv differs:\n%s\n%s\n--- reference\n%s\n%s", dh, db, rh, rb)
	}
	for v := 0; v < 256; v++ {
		want := 3
		if v >= 10 && v <= 20 {
			want = 4
		}
		got, dt := speedCopyRun(t, def.Assembly, map[string]int{"x": v})
		gotRef, rt := speedCopyRun(t, ref.Assembly, map[string]int{"x": v})
		if got != want || gotRef != want || dt > rt {
			t.Fatalf("x=%d: %d/%d want %d, T %d vs reference %d", v, got, gotRef, want, dt, rt)
		}
	}
}

// Rule 1, another function: copying f0 into c0 changes the registers
// execute, which calls c0, uses around the call (review F2(b), seed 257
// reduced). The copy is refused and the product is the reference.
func TestSpeedCopyRefusedWhenAnotherFunctionChanges(t *testing.T) {
	src := `unsigned int result;
unsigned char G0, G1;
unsigned char tab[4] = {210, 51, 137, 108};
unsigned char f0(unsigned char p0, unsigned char p1, unsigned char p2) { if (p2 > 200) { return p1 + p0; } return p1 >> 3; }
unsigned int c0(unsigned char a, unsigned char b) { unsigned char x; G0 = a; x = f0(G0, 172, 0); G0 = b; G1 = x; return x + G0; }
unsigned int c1(unsigned char a, unsigned char b) { G0 = a; G1 = b; return a + b; }
void execute(void) { unsigned char i; unsigned int acc = 0; for (i = 0; i < 14; i++) { unsigned char a = tab[i & 3] + i, b = tab[(i * 7) & 3] ^ i; acc = acc * 3 + c0(a, b); acc = acc * 3 + c1(a, b); } result = acc; }`
	def, ref, compiles := speedCopyProducts(t, src)
	if ref.speedCandidates["f0"] != "c0" || compiles != 1 {
		t.Fatalf("want f0 a candidate judged once: %v, %d compiles", ref.speedCandidates, compiles)
	}
	if def.Assembly != ref.Assembly {
		t.Fatalf("copy kept although execute changed:\n%s", def.Assembly)
	}
	got, dt := speedCopyRun(t, def.Assembly, nil)
	_, rt := speedCopyRun(t, ref.Assembly, nil)
	if got != 6180 || dt > rt {
		t.Fatalf("result %d want 6180, T %d vs reference %d", got, dt, rt)
	}
}

// Rule 2, unknown trip count (review F2(c), seed 317): c2's loop has no
// proved trip count (the model charges 16 trips) and runs once. The copy
// let LICM hoist f2's loads of G0 and G1 out of the loop, which the model
// credited 16 times; that product was 66 T slower. A caller with such a
// loop is refused before any compile.
func TestSpeedCopyUnknownTripLoopRefused(t *testing.T) {
	src := `unsigned int result;
unsigned char G0, G1;
unsigned char tab[16] = {10, 197, 9, 102, 126, 228, 242, 224, 146, 168, 135, 132, 211, 249, 179, 60};
unsigned char f2(unsigned char p0, unsigned char p1) { p0 = (p0 > p1 ? p0 : p0); if (G0 > 122) return G1; return p1; }
unsigned int c2(unsigned char a, unsigned char b) { unsigned char x = 0; unsigned int s = 0; unsigned char k; for (k = 0; k < 1; k++) { x = f2((x > x ? b : x), (b != x ? x : 243)); s += x; a += 3; } return s; }
void execute(void) { unsigned char i; unsigned int acc = 0; for (i = 0; i < 4; i++) { unsigned char a = tab[i & 15] + i, b = tab[(i * 7) & 15] ^ i; G0 = a; G1 = b; acc = acc * 3 + c2(a, b); } result = acc; }`
	def, ref, compiles := speedCopyProducts(t, src)
	if compiles != 0 || len(ref.speedCandidates) != 0 || def.Assembly != ref.Assembly {
		t.Fatalf("want no candidate and the reference: %v, %d compiles\n%s", ref.speedCandidates, compiles, def.Assembly)
	}
	got, dt := speedCopyRun(t, def.Assembly, nil)
	_, rt := speedCopyRun(t, ref.Assembly, nil)
	if dt > rt {
		t.Fatalf("result %d, T %d vs reference %d", got, dt, rt)
	}
}

// Rule 3, per copy: the copy of hh makes execute faster under the model but
// grows it by 46 bytes, beyond 24 per copy. Refused.
func TestSpeedCopyGrowthPerCopyBounded(t *testing.T) {
	src := `unsigned int x, y, result;
unsigned int hh(unsigned int p, unsigned int q) { if (p > q) return p - q + 1000; if (p == q) return q ^ 0x5a5a; return (q - p) + (p << 1); }
void execute(void) { result = hh(x, y); }`
	def, ref, compiles := speedCopyProducts(t, src)
	if ref.speedCandidates["hh"] != "execute" || compiles != 1 || def.Assembly != ref.Assembly {
		t.Fatalf("want hh judged once and refused: %v, %d compiles\n%s", ref.speedCandidates, compiles, def.Assembly)
	}
}

// Rule 3, module cap: ten entries each copy a leaf; e0 grows by 13 bytes,
// e1..e9 by 10. All ten would grow the module by 103 > 96 bytes; the copies
// kept are chosen smallest growth first, then by caller name: e1..e9 (90 B),
// and in0 stays a call. A second compile confirms the reduced set.
func TestSpeedCopyModuleGrowthCap(t *testing.T) {
	var src strings.Builder
	src.WriteString("unsigned char a, b;\nunsigned char r[10];\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&src, "unsigned char in%d(unsigned char v, unsigned char lo) { unsigned char hi = lo + %d; return v >= lo && v <= hi; }\nvoid e%d(void) { r[%d] = in%d(a, b); }\n", i, i+3, i, i, i)
	}
	def, ref, compiles := speedCopyProducts(t, src.String())
	if len(ref.speedCandidates) != 10 || compiles != 2 {
		t.Fatalf("candidates %v, %d compiles", ref.speedCandidates, compiles)
	}
	calls := strings.Count(def.Assembly, "CALL in")
	if calls != 1 || !strings.Contains(def.Assembly, "CALL in0") || len(def.speedCopies) != 9 {
		t.Fatalf("want only in0 called, %d calls, copies %v:\n%s", calls, def.speedCopies, def.Assembly)
	}
	defBytes, _ := Assemble(def.Assembly, "z80")
	refBytes, _ := Assemble(ref.Assembly, "z80")
	if growth := len(defBytes) - len(refBytes); growth > exportSpeedCopyModuleGrowth || growth != 90 {
		t.Fatalf("growth %d B", growth)
	}
	again, _, _ := speedCopyProducts(t, src.String())
	if again.Assembly != def.Assembly {
		t.Fatal("nondeterministic choice")
	}
}

// The pre-filter: pick's caller e2 also calls g, so the static model could
// never score it; the module compiles once, with no candidate compile.
func TestSpeedCopyPrefilterCompilesNothing(t *testing.T) {
	src := `unsigned char a, b, r2;
unsigned char g(unsigned char x) { unsigned char s = 0; while (x) { s += x; x >>= 1; } return s; }
unsigned char pick(unsigned char p, unsigned char q) { return p < q ? q : p; }
void e2(void) { r2 = g(a) + pick(a, b) + g(b); }`
	def, ref, compiles := speedCopyProducts(t, src)
	if compiles != 0 || len(ref.speedCandidates) != 0 || def.Assembly != ref.Assembly {
		t.Fatalf("want no candidate compile: %v, %d compiles", ref.speedCandidates, compiles)
	}
}

// Rule 2 has no no-growth bypass (review F2(a)): the copy of m shrinks w by
// 3 bytes, but the reference has no call to m left to charge (InlineTrivial
// already inlined it), so the model cannot show w faster. Refused.
func TestSpeedCopyNoGrowthNeedsFaster(t *testing.T) {
	src := `unsigned char a, r;
unsigned char m(unsigned char p, unsigned char q) { unsigned char t = p + q; t = t ^ q; t = t + q; return t | q; }
unsigned char w(unsigned char x) { return m(x, 0); }
void execute(void) { r = w(a) + 1; }`
	def, ref, compiles := speedCopyProducts(t, src)
	if ref.speedCandidates["m"] != "w" || compiles != 1 || def.Assembly != ref.Assembly {
		t.Fatalf("want m judged once and refused: %v, %d compiles\n%s", ref.speedCandidates, compiles, def.Assembly)
	}
}

// Rule 1 with a called caller: w receives m's copy and is itself called by
// execute and e2. w keeps its parameter registers and declares the clobber set
// the reference declared (the full ABI here), so its callers are unchanged
// and the copy is kept.
func TestSpeedCopyCallerKeepsDeclaredClobbers(t *testing.T) {
	src := `unsigned char a, r, r2;
unsigned char m(unsigned char p, unsigned char q) { if (p > q) return p; return q; }
static unsigned char w(unsigned char x) { return m(x, x); }
void execute(void) { r = w(a); }
void e2(void) { r2 = w(a + 1); }`
	def, ref, _ := speedCopyProducts(t, src)
	// The reference reaches m by a tail jump.
	if strings.Contains(def.Assembly, "JP m\n") || !strings.Contains(ref.Assembly, "JP m\n") {
		t.Fatalf("want m copied into w:\n%s", def.Assembly)
	}
	for _, name := range []string{"m", "execute", "e2"} {
		dh, db := speedCopyFunction(def.Assembly, name)
		rh, rb := speedCopyFunction(ref.Assembly, name)
		if dh != rh || db != rb {
			t.Fatalf("%s differs:\n%s\n%s\n--- reference\n%s\n%s", name, dh, db, rh, rb)
		}
	}
	dh, _ := speedCopyFunction(def.Assembly, "w")
	rh, _ := speedCopyFunction(ref.Assembly, "w")
	if dh != rh {
		t.Fatalf("w's contract line changed: %s vs %s", dh, rh)
	}
	for v := 0; v < 256; v++ {
		got, dt := speedCopyRun(t, def.Assembly, map[string]int{"a": v})
		gotRef, rt := speedCopyRun(t, ref.Assembly, map[string]int{"a": v})
		if got != v || gotRef != v || dt > rt {
			t.Fatalf("a=%d: %d/%d, T %d vs reference %d", v, got, gotRef, dt, rt)
		}
	}
}
