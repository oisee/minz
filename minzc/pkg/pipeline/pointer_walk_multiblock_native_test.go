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
// mem reads the byte the kernel sees at an address.
type indexedLoopCase struct {
	name   string
	source string
	kernel string
	byteN  bool
	walk   bool // the hot loop must use a pointer walk (default profile)
	oracle func(mem func(int) byte, src int, n, base int) uint16
}

func indexedLoopCases() []indexedLoopCase {
	return []indexedLoopCase{
		{
			name: "fold_max", kernel: "execute", walk: true,
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
			name: "fold_min_break", kernel: "execute", walk: true,
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
			name: "find_return", kernel: "find", walk: true,
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
			name: "count_less_wrap", kernel: "count_less", byteN: true, walk: true,
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
			name: "window_sum", kernel: "execute",
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
			name: "index_after_loop", kernel: "execute",
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
				if tc.walk && mode == "default" {
					var kernel *mir2.Func
					for _, f := range s.MIR2Module.Funcs {
						if f.Name == tc.kernel {
							kernel = f
						}
					}
					if kernel == nil {
						t.Fatalf("no kernel %s\n%s", tc.kernel, s.MIR2Opt)
					}
					bumps, indexed := 0, 0
					for _, l := range mir2.AnalyzeNaturalLoops(kernel) {
						for _, b := range kernel.Blocks {
							if !l.Blocks[b.Label] {
								continue
							}
							for _, x := range b.Insts {
								switch x.Op {
								case mir2.OpPtrBump:
									bumps++
								case mir2.OpPtrAdd:
									indexed++
								}
							}
						}
					}
					if bumps != 1 || indexed != 0 {
						t.Fatalf("want one walk and no indexed address, got %d/%d\n%s", bumps, indexed, s.MIR2Opt)
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
					for _, base := range bases {
						z := emulator.NewRemogattoZ80()
						mem := z.Memory()
						copy(mem[0x8000:], a.Binary)
						src := addr("src")
						for k := 0; k < 256; k++ {
							mem[src+k] = indexedLoopPattern(k)
						}
						if base >= 0 {
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
						// The untransformed MIR in the VM, for inputs inside src.
						if base >= 0 {
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
						vm.WriteHeapSlice(vm.GlobalAddr("base"), []byte{byte(vsrc), byte(vsrc >> 8)})
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
