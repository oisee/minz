package pipeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// boundLoadsInLoops counts loads of the global named bound (or its v_ alias)
// that still execute inside a natural loop of any function.
func boundLoadsInLoops(m *mir2.Module, bound string) int {
	count := 0
	for _, f := range m.Funcs {
		defs := map[mir2.Reg]*mir2.Inst{}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.Dst != mir2.NoReg {
					defs[x.Dst] = x
				}
			}
		}
		for _, l := range mir2.AnalyzeNaturalLoops(f) {
			for _, b := range f.Blocks {
				if !l.Blocks[b.Label] {
					continue
				}
				for _, x := range b.Insts {
					if x.Op != mir2.OpLoad {
						continue
					}
					a := defs[x.Src[0]]
					for a != nil && a.Op == mir2.OpMove {
						a = defs[a.Src[0]]
					}
					if a != nil && a.Op == mir2.OpAddrOf && (a.Sym == bound || a.Sym == "v_"+bound) {
						count++
					}
				}
			}
		}
	}
	return count
}

func inBoundsCount(m *mir2.Module, fn string) int {
	count := 0
	for _, f := range m.Funcs {
		if fn != "" && f.Name != fn {
			continue
		}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.InBounds {
					count++
				}
			}
		}
	}
	return count
}

// TestHoistInBoundsStoreNative checks, end to end from C and Nanz, which
// stores let the loop bound `n` leave the loop, and that every program still
// computes the oracle's memory on both VMs and on the Z80 for zero-trip,
// short, 255/256 and (with a 300-byte array) longer in-bounds counts.
func TestHoistInBoundsStoreNative(t *testing.T) {
	type program struct {
		name, lang, source string
		length             int  // array length of source/target
		hoisted            bool // n no longer loaded inside a loop
		inBounds           bool // the raw MIR carries InBounds ptr_adds
		// oracle returns target bytes and result for count n.
		oracle func(src []byte, n int) ([]byte, uint16)
	}
	mapStore := func(length int) func(src []byte, n int) ([]byte, uint16) {
		return func(src []byte, n int) ([]byte, uint16) {
			out := bytes.Repeat([]byte{0xa5}, length)
			for i := 0; i < n; i++ {
				out[i] = src[i] ^ 255
			}
			return out, uint16(n)
		}
	}
	lastByte := func(length int) func(src []byte, n int) ([]byte, uint16) {
		return func(src []byte, n int) ([]byte, uint16) {
			out := bytes.Repeat([]byte{0xa5}, length)
			last := uint16(0)
			for i := 0; i < n; i++ {
				last = uint16(src[i])
			}
			return out, last
		}
	}
	cMapStore := func(length int) string {
		return fmt.Sprintf(`unsigned char source[%d];
unsigned short n, result;
unsigned char target[%d];
void execute(void) {
    unsigned short i;
    for (i = 0; i < n; ++i) target[i] = (unsigned char)(source[i] ^ 255);
    result = n;
}
`, length, length)
	}
	programs := []program{
		{name: "c-declared-arrays-256", lang: "c", source: cMapStore(256), length: 256, hoisted: true, inBounds: true, oracle: mapStore(256)},
		{name: "c-declared-arrays-300", lang: "c", source: cMapStore(300), length: 300, hoisted: true, inBounds: true, oracle: mapStore(300)},
		// The same loop in Nanz: indexing carries no declared-object rule, so
		// the u16 store index proves nothing and n stays in the loop.
		{name: "nanz-same-loop", lang: "nanz", length: 300, oracle: mapStore(300), source: `global source:[u8;300]
global n:u16
global result:u16
global target:[u8;300]
fun execute() {
  var i:u16 = 0
  while i < n {
    target[i] = source[i] xor 255
    i = i + 1
  }
  result = n
}
`},
		// A store through a pointer parameter may write n. (The load
		// source[i] still carries InBounds; only stores matter.)
		{name: "c-pointer-parameter", lang: "c", length: 300, inBounds: true, oracle: mapStore(300), source: `unsigned char source[300];
unsigned short n, result;
unsigned char target[300];
void kernel(unsigned char *p) {
    unsigned short i;
    for (i = 0; i < n; ++i) p[i] = (unsigned char)(source[i] ^ 255);
}
void execute(void) { kernel(target); result = n; }
`},
		// The range proof needs no language rule: the store address is a
		// constant inside another global, in either frontend.
		{name: "nanz-range-proof-constant-store", lang: "nanz", length: 300, hoisted: true, oracle: lastByte(300), source: `global source:[u8;300]
global n:u16
global result:u16
global target:[u8;300]
fun execute() {
  var i:u16 = 0
  while i < n {
    result = u16(source[i])
    i = i + 1
  }
}
`},
	}
	for _, p := range programs {
		for _, mode := range []string{"default", "size", "grace"} {
			t.Run(p.name+"/"+mode, func(t *testing.T) {
				var h *hir.Module
				var err error
				if p.lang == "c" {
					h, err = c89.CompileWithOpts(p.source, "hoist.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse(p.source, "hoist.nanz")
				}
				if err != nil {
					t.Fatal(err, p.source)
				}
				raw := hir.LowerModule(h)
				if got := inBoundsCount(raw, "") > 0; got != p.inBounds {
					t.Fatalf("InBounds present=%v, want %v\n%s", got, p.inBounds, raw.Dump())
				}
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size", UseGrace: mode == "grace", UseHIRGrace: mode == "grace"})
				if err != nil {
					t.Fatal(err)
				}
				// The size profile keeps whichever of the walked and indexed
				// forms is smaller, so only the speed modes pin the decision.
				if mode != "size" {
					if got := boundLoadsInLoops(s.MIR2Module, "n") == 0; got != p.hoisted {
						t.Fatalf("n hoisted=%v, want %v\n%s", got, p.hoisted, s.MIR2Opt)
					}
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(a.Errors) > 0 {
					t.Fatal(err, a.Errors)
				}
				addr := func(name string) int {
					v := a.Symbols[name]
					if v == 0 {
						v = a.Symbols["v_"+name]
					}
					if v == 0 {
						t.Fatal(name)
					}
					return v
				}
				src := make([]byte, p.length)
				for i := range src {
					src[i] = byte(i*37 + 11)
				}
				vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
				counts := []int{0, 1, 2, 127, 255, 256}
				if p.length > 256 {
					counts = append(counts, p.length)
				}
				for _, n := range counts {
					want, wantResult := p.oracle(src, n)
					for vi, vm := range vms {
						vm.WriteHeapSlice(vm.GlobalAddr("source"), src)
						vm.WriteHeapSlice(vm.GlobalAddr("target"), bytes.Repeat([]byte{0xa5}, p.length))
						vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n), byte(n >> 8)})
						vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0, 0})
						if _, err := vm.Call("execute", nil); err != nil {
							t.Fatal("VM", vi, n, err)
						}
						r := vm.ReadHeap(vm.GlobalAddr("result"), 2)
						if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("target"), p.length), want) || binary.LittleEndian.Uint16(r) != wantResult {
							t.Fatalf("VM %d n=%d", vi, n)
						}
					}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], a.Binary)
					copy(mem[addr("source"):], src)
					copy(mem[addr("target"):], bytes.Repeat([]byte{0xa5}, p.length))
					binary.LittleEndian.PutUint16(mem[addr("n"):], uint16(n))
					binary.LittleEndian.PutUint16(mem[addr("result"):], 0)
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					z.SetRegisters(emulator.Registers{PC: uint16(addr("execute")), SP: 0xeffe})
					for step := 0; step < 200000 && z.GetPC() != 0xf100; step++ {
						z.Step()
					}
					if z.GetPC() != 0xf100 || z.GetRegisters().SP != 0xf000 ||
						!bytes.Equal(mem[addr("target"):addr("target")+p.length], want) ||
						binary.LittleEndian.Uint16(mem[addr("result"):]) != wantResult {
						t.Fatalf("Z80 n=%d registers=%+v\n%s", n, z.GetRegisters(), s.Assembly)
					}
				}
			})
		}
	}
}

// TestHoistInBoundsStoreSameGlobalNative: the bound and the stores live in
// one global (meta[0] bounds stores to meta[1..]). The declared-object rule
// proves the stores stay inside meta, which is exactly the loaded object, so
// the bound must stay in the loop.
func TestHoistInBoundsStoreSameGlobalNative(t *testing.T) {
	source := `unsigned char source[300];
unsigned short meta[301];
void execute(void) {
    unsigned short i;
    for (i = 0; i < meta[0]; ++i) meta[i + 1] = (unsigned short)(source[i] ^ 255);
}
`
	h, err := c89.CompileWithOpts(source, "same.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	if inBoundsCount(hir.LowerModule(h), "execute") == 0 {
		t.Fatal("expected the C subscripts to carry InBounds")
	}
	if boundLoadsInLoops(s.MIR2Module, "meta") == 0 {
		t.Fatalf("the load of meta[0] left the loop although the loop stores into meta\n%s", s.MIR2Opt)
	}
	vm := mir2.NewVM(s.MIR2Module)
	src := make([]byte, 300)
	for i := range src {
		src[i] = byte(i * 5)
	}
	for _, n := range []int{0, 3, 300} {
		vm.WriteHeapSlice(vm.GlobalAddr("source"), src)
		meta := make([]byte, 602)
		binary.LittleEndian.PutUint16(meta, uint16(n))
		vm.WriteHeapSlice(vm.GlobalAddr("meta"), meta)
		if _, err := vm.Call("execute", nil); err != nil {
			t.Fatal(err)
		}
		got := vm.ReadHeap(vm.GlobalAddr("meta"), 602)
		for i := 0; i < n; i++ {
			if v := binary.LittleEndian.Uint16(got[2*i+2:]); v != uint16(src[i]^255) {
				t.Fatalf("n=%d meta[%d]=%d", n, i+1, v)
			}
		}
	}
}

// TestHoistInBoundsFlagOnlyOnDeclaredCArrays pins where the C frontend sets
// InBounds: only a subscript whose base names a declared, complete,
// static-storage array object.
func TestHoistInBoundsFlagOnlyOnDeclaredCArrays(t *testing.T) {
	source := `unsigned char g[16];
unsigned short w[8];
struct S { unsigned char len; unsigned char arr[4]; } st;
unsigned short wide;
void global_byte(unsigned char i) { g[i] = 1; }
void global_word(unsigned char i) { w[i] = 1; }
void static_local(unsigned char i) { static unsigned char s[10]; s[i] = 1; g[0] = s[1]; }
void pointer_param(unsigned char *p, unsigned char i) { p[i] = 1; }
void array_param(unsigned char p[10], unsigned char i) { p[i] = 1; }
void pointer_var(unsigned char i) { unsigned char *q = g; q[i] = 1; }
void user_arith(unsigned char i) { *(g + i) = 1; }
void swapped(unsigned char i) { i[g] = 1; }
void paren(unsigned char i) { (g)[i] = 1; }
void cast_base(unsigned char i) { ((unsigned char *)&wide)[i] = 1; }
void member(unsigned char i) { st.arr[i] = 1; }
void automatic(unsigned char i) { unsigned char a[10]; a[i] = 1; wide = a[2]; }
`
	h, err := c89.CompileWithOpts(source, "flags.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	want := map[string]bool{"global_byte": true, "global_word": true, "static_local": true}
	for _, f := range raw.Funcs {
		if strings.HasPrefix(f.Name, "__") {
			continue
		}
		if got := inBoundsCount(raw, f.Name) > 0; got != want[f.Name] {
			t.Errorf("%s: InBounds=%v, want %v\n%s", f.Name, got, want[f.Name], raw.Dump())
		}
	}
}
