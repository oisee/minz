package pipeline

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// indexedLoopCase is a C kernel over `unsigned char src[256]` with a bound
// global `n`, an optional pointer global `base` and a u16 `result`. oracle
// computes the expected result independently of both VMs and the compiler;
// mem reads the byte the kernel sees at an address before it runs.
type indexedLoopCase struct {
	name   string
	source string
	kernel string
	byteN  bool
	// shape of the hot loop in the default profile: "counter" (walk and
	// down-counter, index gone), "walk" (walk, index kept) or "indexed".
	shape  string
	aliasN bool // base points at n
	oracle func(mem func(int) byte, src int, n, base int) uint16
}

func indexedLoopCases() []indexedLoopCase {
	return []indexedLoopCase{
		{
			name: "fold_max", kernel: "execute", shape: "counter",
			source: `unsigned char src[256]; unsigned short n, base, result;
void execute(void) {
    unsigned short i; unsigned char s = 0;
    for (i = 0; i < n; ++i) if (src[i] > s) s = src[i];
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := byte(0)
				for i := 0; i < n; i++ {
					if v := mem(src + i); v > s {
						s = v
					}
				}
				return uint16(s)
			},
		},
		{
			name: "fold_min_break", kernel: "execute", shape: "counter",
			source: `unsigned char src[256]; unsigned short n, base, result;
void execute(void) {
    unsigned short i; unsigned char s = 255;
    for (i = 0; i < n; ++i) {
        if (src[i] == 0xEE) break;
        if (src[i] < s) s = src[i];
    }
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := byte(255)
				for i := 0; i < n; i++ {
					v := mem(src + i)
					if v == 0xee {
						break
					}
					if v < s {
						s = v
					}
				}
				return uint16(s)
			},
		},
		{
			name: "find_return", kernel: "find", shape: "walk",
			source: `unsigned char src[256]; unsigned short n, base, result;
unsigned short find(void) {
    unsigned short i;
    for (i = 0; i < n; ++i) {
        if (src[i] == 0xEE) return src[i] + 1000;
        if (src[i] == 0x11) result = result + 1;
    }
    return 7;
}
void execute(void) { result = 0; result = find() + result; }`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				hits := 0
				for i := 0; i < n; i++ {
					v := mem(src + i)
					if v == 0xee {
						return uint16(1000 + int(v) + hits)
					}
					if v == 0x11 {
						hits++
					}
				}
				return uint16(7 + hits)
			},
		},
		{
			name: "count_less_wrap", kernel: "count_less", byteN: true, shape: "counter",
			source: `unsigned char src[256], n; unsigned short base, result;
unsigned short count_less(unsigned char *p, unsigned char length, unsigned char threshold) {
    unsigned short count = 0;
    unsigned char i;
    for (i = 0; i < length; ++i) {
        if (p[i] < threshold) ++count;
    }
    return count;
}
void execute(void) { result = count_less((unsigned char *)base, n, 0x80); }`,
			oracle: func(mem func(int) byte, _, n, base int) uint16 {
				c := 0
				for i := 0; i < n; i++ {
					if mem((base+i)&0xffff) < 0x80 {
						c++
					}
				}
				return uint16(c)
			},
		},
		{
			name: "window_sum", kernel: "execute", shape: "counter",
			source: `unsigned char src[256]; unsigned short n, base, result;
void execute(void) {
    unsigned short s = 0, i, count = 0;
    if (n > 4) count = n - 4;
    for (i = 0; i < count; ++i) s += src[i + 4];
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := 0
				for i := 0; i+4 < n; i++ {
					s += int(mem(src + i + 4))
				}
				return uint16(s)
			},
		},
		{
			name: "index_after_loop", kernel: "execute", shape: "walk",
			source: `unsigned char src[256]; unsigned short n, base, result;
void execute(void) {
    unsigned short i; unsigned char s = 0;
    for (i = 0; i < n; ++i) if (src[i] > s) s = src[i];
    result = s + i;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := byte(0)
				for i := 0; i < n; i++ {
					if v := mem(src + i); v > s {
						s = v
					}
				}
				return uint16(int(s) + n)
			},
		},
		{
			// j is computed from the index inside the loop and read only
			// after it, so the index cannot be replaced by a counter.
			name: "derived_after_loop", kernel: "find", shape: "walk",
			source: `unsigned char src[256]; unsigned short n, base, result;
unsigned short find(void) {
    unsigned short i;
    for (i = 0; i < n; ++i) {
        unsigned short j = i + 1;
        if (src[i] == 0xEE) return j;
    }
    return 0;
}
void execute(void) { result = find(); }`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				for i := 0; i < n; i++ {
					if mem(src+i) == 0xee {
						return uint16(i + 1)
					}
				}
				return 0
			},
		},
		{
			name: "scaled_after_loop", kernel: "find", shape: "walk",
			source: `unsigned char src[256]; unsigned short n, base, result;
unsigned short find(void) {
    unsigned short i;
    for (i = 0; i < n; ++i) {
        unsigned short off = i * 40;
        if (src[i] == 0xEE) return off;
    }
    return 0xFFFF;
}
void execute(void) { result = find(); }`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				for i := 0; i < n; i++ {
					if mem(src+i) == 0xee {
						return uint16(i * 40)
					}
				}
				return 0xffff
			},
		},
		{
			// The byte product (x*2 truncated to u8) is narrowed to a byte
			// shift, so the body no longer needs a word temporary for an i16
			// multiply and walk, sum and counter fit: counted walk. Before the
			// narrowing of constant byte products this stayed indexed.
			name: "map_filter_sum", kernel: "execute", shape: "counter",
			source: `unsigned char src[256]; unsigned short n, base, result;
void execute(void) {
    unsigned short s = 0, i; unsigned char y;
    for (i = 0; i < n; ++i) { y = (unsigned char)(src[i] * 2); if (y > 10 && y < 200) s += y; }
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := 0
				for i := 0; i < n; i++ {
					if y := byte(int(mem(src+i)) * 2); y > 10 && y < 200 {
						s += int(y)
					}
				}
				return uint16(s)
			},
		},
		{
			// The loop stores to its bound: no down-counter.
			name: "bound_stored", kernel: "execute", byteN: true, shape: "walk",
			source: `unsigned char src[256], n; unsigned short base, result;
void execute(void) {
    unsigned char i, s = 0;
    for (i = 0; i < n; ++i) { s += src[i]; if (src[i] == 0xEE) n = 3; }
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := byte(0)
				for i := 0; i < n; i++ {
					v := mem(src + i)
					s += v
					if v == 0xee {
						n = 3
					}
				}
				return uint16(s)
			},
		},
		{
			// The loop stores through a pointer that aliases its bound.
			name: "bound_aliased", kernel: "execute", byteN: true, shape: "walk", aliasN: true,
			source: `unsigned char src[256], n; unsigned short base, result;
void execute(void) {
    unsigned char i, s = 0; unsigned char *q = (unsigned char *)base;
    for (i = 0; i < n; ++i) { s += src[i]; *q = (unsigned char)(src[i] | 0x80); }
    result = s;
}`,
			oracle: func(mem func(int) byte, src, n, _ int) uint16 {
				s := byte(0)
				for i := 0; i < n; i++ {
					v := mem(src + i)
					s += v
					n = int(v | 0x80)
				}
				return uint16(s)
			},
		},
	}
}

func indexedLoopPattern(a int) byte {
	v := byte(a*29 + a>>8*7 + 3)
	if a%97 == 40 {
		v = 0xee
	}
	if a%13 == 5 {
		v = 0x11
	}
	return v
}

// TestIndexedLoopPointerWalkNative compiles C indexed loops with branches,
// early exits and escaping indices through the real pipeline and compares
// native Z80 results with the MIR2 VM run of the untransformed module and an
// independent oracle, including zero, one and maximum trip counts and a
// pointer walk that wraps past 0xFFFF.
func TestIndexedLoopPointerWalkNative(t *testing.T) {
	for _, tc := range indexedLoopCases() {
		for _, mode := range []string{"default", "size"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				h, err := c89.CompileWithOpts(tc.source, tc.name+".c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				if mode == "default" {
					var kernel *mir2.Func
					for _, f := range s.MIR2Module.Funcs {
						if f.Name == tc.kernel {
							kernel = f
						}
					}
					if kernel == nil {
						t.Fatalf("no kernel %s\n%s", tc.kernel, s.MIR2Opt)
					}
					bumps, indexed, ult, ne := 0, 0, 0, 0
					for _, l := range mir2.AnalyzeNaturalLoops(kernel) {
						for _, b := range kernel.Blocks {
							if !l.Blocks[b.Label] {
								continue
							}
							for _, x := range b.Insts {
								switch {
								case x.Op == mir2.OpPtrBump:
									bumps++
								case x.Op == mir2.OpPtrAdd:
									indexed++
								case x.Op == mir2.OpCmp && b == l.Header && x.Cond == mir2.CmpNe:
									ne++
								case x.Op == mir2.OpCmp && b == l.Header:
									ult++
								}
							}
						}
					}
					got := "indexed"
					if bumps == 1 && indexed == 0 {
						got = "walk"
						if ne == 1 && ult == 0 {
							got = "counter"
						}
					}
					if got != tc.shape {
						t.Fatalf("loop shape %s, want %s\n%s", got, tc.shape, s.MIR2Opt)
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
				counts := []int{0, 1, 2, 3, 4, 5, 6, 39, 41, 127, 255, 256}
				if tc.byteN {
					counts = counts[:len(counts)-1]
				}
				for _, n := range counts {
					bases := []int{-1}
					if tc.name == "count_less_wrap" {
						bases = []int{-1, 0xff80}
					}
					if tc.aliasN {
						bases = []int{addr("n")}
					}
					for _, base := range bases {
						z := emulator.NewRemogattoZ80()
						mem := z.Memory()
						copy(mem[0x8000:], a.Binary)
						src := addr("src")
						for k := 0; k < 256; k++ {
							mem[src+k] = indexedLoopPattern(k)
						}
						wrap := base >= 0 && !tc.aliasN
						if wrap {
							for k := 0; k < 256; k++ {
								mem[(base+k)&0xffff] = indexedLoopPattern(k + 1000)
							}
						}
						effective := base
						if base < 0 {
							effective = src
						}
						if tc.byteN {
							mem[addr("n")] = byte(n)
						} else {
							binary.LittleEndian.PutUint16(mem[addr("n"):], uint16(n))
						}
						binary.LittleEndian.PutUint16(mem[addr("base"):], uint16(effective))
						want := tc.oracle(func(a int) byte { return mem[a&0xffff] }, src, n, effective)
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{PC: uint16(addr("execute")), SP: 0xeffe})
						for step := 0; step < 2000000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						got := binary.LittleEndian.Uint16(mem[addr("result"):])
						if z.GetPC() != 0xf100 || z.GetRegisters().SP != 0xf000 || got != want {
							t.Fatalf("Z80 n=%d base=%#x: got %d want %d\n%s\n%s", n, base, got, want, s.MIR2Opt, s.Assembly)
						}
						// The untransformed MIR in the VM, for inputs inside its heap.
						if wrap {
							continue
						}
						vm := mir2.NewVM(raw)
						vsrc := vm.GlobalAddr("src")
						for k := 0; k < 256; k++ {
							vm.WriteHeapSlice(vsrc+int64(k), []byte{indexedLoopPattern(k)})
						}
						if tc.byteN {
							vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n)})
						} else {
							vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n), byte(n >> 8)})
						}
						vbase := vsrc
						if tc.aliasN {
							vbase = vm.GlobalAddr("n")
						}
						vm.WriteHeapSlice(vm.GlobalAddr("base"), []byte{byte(vbase), byte(vbase >> 8)})
						if _, err := vm.Call("execute", nil); err != nil {
							t.Fatal(err)
						}
						r := vm.ReadHeap(vm.GlobalAddr("result"), 2)
						if vgot := binary.LittleEndian.Uint16(r); vgot != got {
							t.Fatalf("raw VM n=%d: %d, Z80 %d", n, vgot, got)
						}
					}
				}
			})
		}
	}
}
