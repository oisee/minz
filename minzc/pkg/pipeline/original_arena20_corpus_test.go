package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/asmutil"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type arena20State struct{ ptr, end uint16 }

func (a arena20State) bytes() []byte {
	return []byte{byte(a.ptr), byte(a.ptr >> 8), byte(a.end), byte(a.end >> 8)}
}
func TestOriginalArena20Corpus(t *testing.T) {
	path := "reports/showcase-src/2026-03-14/ex20_arena_allocator.nanz"
	data, e := os.ReadFile("../../../" + path)
	if e != nil {
		t.Fatal(e)
	}
	sha := "886e1b3fb2e92c1f16ebf585a03a81f6fd1b2d0bcd2ed24559b4a9584adf21d9"
	if fmt.Sprintf("%x", sha256.Sum256(data)) != sha {
		t.Fatal("sourceSHA")
	}
	export := func(n string, b []byte) {
		if d := os.Getenv("ARENA20_EXPORT"); d != "" {
			p := filepath.Join(d, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(data), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	original := hir.LowerModule(parse())
	entries := []string{"Arena_init", "Arena_alloc", "Arena_reset", "Arena_remaining", "arena_split", "test_split_and_alloc", "init_a", "alloc_enemy"}
	if len(original.Funcs) != len(entries) {
		t.Fatal("entries", len(original.Funcs))
	}
	export("raw.mir", []byte(original.Dump()))
	export("source.nanz", data)
	bounds := []uint16{0, 1, 2, 7, 127, 128, 254, 255, 256, 257, 32767, 32768, 65534, 65535}
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			s, e := CompileHIRSteps(parse(), mode.o)
			if e != nil {
				t.Fatal(e)
			}
			export(mode.name+".a80", []byte(s.Assembly))
			export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || a == nil || len(a.Errors) > 0 {
				t.Fatalf("assemble %v %+v", e, a)
			}
			export(mode.name+".bin", a.Binary)
			info, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": a.Symbols}, "", "  ")
			export(mode.name+"-abi.json", info)
			raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
			raw.MaxSteps = 0
			final.MaxSteps = 0
			raw.EnsureHeap(65536)
			final.EnsureHeap(65536)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			profiler := emulator.NewProfiler()
			z.SetProfiler(profiler)
			for i := range z.Memory() {
				v := byte(i*73 + 29)
				z.Memory()[i] = v
				raw.WriteHeap(int64(i), v)
				final.WriteHeap(int64(i), v)
			}
			z.LoadMemory(0x8000, a.Binary)
			drivers := map[string]uint16{}
			for i, name := range entries {
				target, ok := a.Symbols[name]
				if !ok {
					t.Fatal("missingentry", name)
				}
				d := uint16(0x7000 + 4*i)
				b := []byte{0xcd, byte(target), byte(target >> 8), 0x76}
				z.LoadMemory(d, b)
				drivers[name] = d
				export(mode.name+"-call-"+name+".bin", b)
			}
			before := append([]byte(nil), z.Memory()...)
			allowed := map[int]bool{}
			globalKeys := []string{"perm", "level", "frame", "a"}
			globalAddr := map[string]uint16{}
			for _, g := range globalKeys {
				v, ok := a.Symbols[asmutil.SanitizeZ80Label(g)]
				if !ok {
					t.Fatal("global", g)
				}
				globalAddr[g] = uint16(v)
				for j := 0; j < 4; j++ {
					allowed[v+j] = true
				}
			}
			for _, f := range s.MIR2Module.Funcs {
				types := map[mir2.Reg]mir2.Ty{}
				for _, p := range f.Contract.Params {
					types[p.Reg] = p.Ty
				}
				for _, b := range f.Blocks {
					for _, p := range b.Params {
						types[p.Dst] = p.Ty
					}
					for _, i := range b.Insts {
						if i.Dst != mir2.NoReg {
							types[i.Dst] = i.Ty
						}
					}
				}
				for r, ty := range types {
					if v, ok := a.Symbols[mir2.Z80SpillLabel(f.Name, r)]; ok {
						for j := 0; j < mir2.NativeSpillWidth(ty); j++ {
							allowed[v+j] = true
						}
					}
				}
			}
			counts := map[string]int{}
			cost := map[string]int64{}
			depths := map[string]int{}
			minSP := uint16(0xf000)
			// Address-independent exact per-byte source effect stream. Plain RAM reads may be forwarded.
			run := func(name string, args []uint16, in, out map[string]arena20State, stores []struct {
				key    string
				offset int
				v      byte
			}, want []uint16, ptr uint16) {
				counts[name]++
				resolve := func(key string, vm *mir2.VM) uint16 {
					if key == "object" {
						return ptr
					}
					if vm != nil {
						return uint16(vm.GlobalAddr(key))
					}
					return globalAddr[key]
				}
				for _, vm := range []*mir2.VM{raw, final} {
					vmGuard := map[uint16]byte{}
					if _, object := in["object"]; object {
						vmGuard[ptr-1] = vm.ReadHeap(int64(ptr-1), 1)[0]
						vmGuard[ptr+4] = vm.ReadHeap(int64(uint16(ptr+4)), 1)[0]
					}
					for k, st := range in {
						addr := resolve(k, vm)
						for j, v := range st.bytes() {
							if vm.ReadHeap(int64(uint16(int(addr)+j)), 1)[0] != v {
								if name == "alloc_enemy" {
									t.Fatal("persistent VM state changed", name, k)
								}
								vm.WriteHeap(int64(uint16(int(addr)+j)), v)
							}
						}
					}
					ev := []mir2.VMDataAccess{}
					vm.ObserveDataAccess = func(e mir2.VMDataAccess) { ev = append(ev, e) }
					vv := make([]mir2.Value, len(args))
					for i, v := range args {
						vv[i] = mir2.Value{I: int64(v)}
					}
					if _, object := in["object"]; len(vv) > 0 && object {
						vv[0].I = int64(ptr)
					}
					values, e := vm.Call(name, vv)
					if e != nil || len(values) != len(want) {
						t.Fatalf("VM %s%v got%v want%v %v", name, args, values, want, e)
					}
					for i, w := range want {
						if values[i].I != int64(w) {
							t.Fatalf("VM%s%v got%v want%v", name, args, values, want)
						}
					}
					state := map[string][]byte{}
					for k, st := range in {
						state[k] = st.bytes()
					}
					seen := []struct {
						key    string
						offset int
						v      byte
					}{}
					for _, e := range ev {
						key := ""
						off := 0
						for k := range state {
							addr := resolve(k, vm)
							d := int(uint16(e.Addr) - addr)
							if d < 4 {
								key = k
								off = d
								break
							}
						}
						if key == "" {
							t.Fatalf("VM outsideArena%s %+v", name, e)
						}
						if e.Op == mir2.OpLoad {
							if state[key][off] != e.Value {
								t.Fatal("VMreadstate", name, e, state)
							}
						} else if e.Op == mir2.OpStore {
							state[key][off] = e.Value
							seen = append(seen, struct {
								key    string
								offset int
								v      byte
							}{key, off, e.Value})
						} else {
							t.Fatal("event", e)
						}
					}
					if len(seen) != len(stores) || (len(stores) > 0 && !reflect.DeepEqual(seen, stores)) {
						t.Fatalf("VMstoreorder%s got%v want%v", name, seen, stores)
					}
					for addr, w := range vmGuard {
						if vm.ReadHeap(int64(addr), 1)[0] != w {
							t.Fatal("VM adjacent guard", name, addr)
						}
					}
					for k, st := range out {
						addr := resolve(k, vm)
						for j, v := range st.bytes() {
							if vm.ReadHeap(int64(uint16(int(addr)+j)), 1)[0] != v {
								t.Fatal("VMwholeArena", name, k)
							}
						}
					}
				}
				nativeGuard := map[uint16]byte{}
				if _, object := in["object"]; object {
					nativeGuard[ptr-1] = z.Memory()[ptr-1]
					nativeGuard[ptr+4] = z.Memory()[ptr+4]
				}
				for k, st := range in {
					addr := resolve(k, nil)
					for j, v := range st.bytes() {
						d := uint16(int(addr) + j)
						if z.Memory()[d] != v {
							if name == "alloc_enemy" {
								t.Fatal("persistent native state changed", z.Memory()[addr:addr+4], st)
							}
							z.Memory()[d] = v
						}
						allowed[int(d)] = true
					}
				}
				f := s.MIR2Module.FuncByName(name)
				z.SetRegisters(emulator.Registers{PC: drivers[name], SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				for i, p := range f.Contract.Params {
					v := args[i]
					loc := assertParamLocation(p, s.Allocation)
					if loc.Kind == mir2.LocMem {
						vaddr, ok := a.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
						if !ok {
							t.Fatal("paramspill")
						}
						z.Memory()[vaddr] = byte(v)
						if mir2.NativeSpillWidth(p.Ty) > 1 {
							z.Memory()[vaddr+1] = byte(v >> 8)
						}
					} else if mir2.NativeSpillWidth(p.Ty) > 1 {
						h := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
						if h[0] == "" {
							t.Fatal(loc)
						}
						z.SetRegister8(h[0], byte(v>>8))
						z.SetRegister8(h[1], byte(v))
					} else {
						z.SetRegister8(loc.Name, byte(v))
					}
				}
				state := map[string][]byte{}
				for k, st := range in {
					state[k] = st.bytes()
				}
				seen := []struct {
					key    string
					offset int
					v      byte
				}{}
				done := false
				callMin := uint16(0xf000)
				cycles := int64(0)
				for step := 0; step < 200000; step++ {
					if z.GetPC() == drivers[name]+3 && z.GetSP() == 0xf000 {
						done = true
						break
					}
					r := z.GetRegisters()
					ev := widemathNativeByteAccess(z.Memory(), r.PC, r)
					op := z.Memory()[r.PC]
					if op == 0x32 || op == 0x3a {
						addr := uint16(z.Memory()[r.PC+1]) | uint16(z.Memory()[r.PC+2])<<8
						kind := mir2.OpLoad
						v := z.Memory()[addr]
						if op == 0x32 {
							kind = mir2.OpStore
							v = r.A
						}
						ev = []mir2.VMDataAccess{{Op: kind, Addr: int64(addr), Value: v}}
					}
					for _, e := range ev {
						key := ""
						off := 0
						for k := range state {
							d := int(uint16(e.Addr) - resolve(k, nil))
							if d < 4 {
								key = k
								off = d
								break
							}
						}
						if key == "" {
							continue
						}
						if e.Op == mir2.OpLoad {
							if state[key][off] != e.Value {
								t.Fatal("native readstate", name, e)
							}
						} else {
							state[key][off] = e.Value
							seen = append(seen, struct {
								key    string
								offset int
								v      byte
							}{key, off, e.Value})
						}
					}
					cycles += int64(z.Step())
					if z.GetSP() < callMin {
						callMin = z.GetSP()
					}
				}
				if !done {
					t.Fatalf("CALL/SP%s %+v", name, z.GetRegisters())
				}
				if len(seen) != len(stores) || (len(stores) > 0 && !reflect.DeepEqual(seen, stores)) {
					t.Fatalf("native storeorder%s got%v want%v", name, seen, stores)
				}
				for addr, w := range nativeGuard {
					if z.Memory()[addr] != w {
						t.Fatal("native adjacent guard", name, addr)
					}
				}
				for k, st := range out {
					addr := resolve(k, nil)
					for j, v := range st.bytes() {
						if z.Memory()[uint16(int(addr)+j)] != v {
							t.Fatal("nativewholeArena", name, k)
						}
					}
				}
				if len(want) > 0 {
					ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
					r := z.GetRegisters()
					got := uint16(r.A)
					switch ret {
					case "HL":
						got = r.HL
					case "DE":
						got = r.DE
					case "BC":
						got = r.BC
					case "IX":
						got = r.IX
					case "IY":
						got = r.IY
					case "A":
					default:
						t.Fatal(ret)
					}
					if got != want[0] {
						t.Fatalf("nativeret%s%v got%x want%x", name, args, got, want)
					}
				}
				if callMin < minSP {
					minSP = callMin
				}
				if d := 0xf000 - int(callMin); d > depths[name] {
					depths[name] = d
				}
				cost[name] += cycles
			}
			storeField := func(key string, off int, v uint16) []struct {
				key    string
				offset int
				v      byte
			} {
				return []struct {
					key    string
					offset int
					v      byte
				}{{key, off, byte(v)}, {key, off + 1, byte(v >> 8)}}
			}
			for _, ptr := range []uint16{0x4000, 0xfffe, 0xffff} {
				for _, base := range bounds {
					for _, size := range bounds {
						in := map[string]arena20State{"object": {0x1234, 0x5678}}
						out := map[string]arena20State{"object": {base, base + size}}
						st := append(storeField("object", 0, base), storeField("object", 2, base+size)...)
						run("Arena_init", []uint16{ptr, base, size}, in, out, st, nil, ptr)
						run("arena_split", []uint16{ptr, base, size}, in, out, st, []uint16{base + size}, ptr)
					}
				}
				for _, p := range bounds {
					for _, end := range bounds {
						in := map[string]arena20State{"object": {p, end}}
						run("Arena_remaining", []uint16{ptr}, in, in, nil, []uint16{end - p}, ptr)
						for _, base := range bounds {
							run("Arena_reset", []uint16{ptr, base}, in, map[string]arena20State{"object": {base, end}}, storeField("object", 0, base), nil, ptr)
						}
						for _, n := range bounds {
							next := p + n
							want := uint16(0)
							out := in
							var st []struct {
								key    string
								offset int
								v      byte
							}
							if next <= end {
								want = p
								out = map[string]arena20State{"object": {next, end}}
								st = storeField("object", 0, next)
							}
							run("Arena_alloc", []uint16{ptr, n}, in, out, st, []uint16{want}, ptr)
						}
					}
				}
			}
			// Original wrapper/sandbox models follow in a separate continuation below.
			in := map[string]arena20State{"perm": {1, 2}, "level": {3, 4}, "frame": {5, 6}}
			out := map[string]arena20State{"perm": {0xc008, 0xc100}, "level": {0xc100, 0xc900}, "frame": {0xc920, 0xcd00}}
			st := []struct {
				key    string
				offset int
				v      byte
			}{}
			for _, x := range []struct {
				k string
				o int
				v uint16
			}{{"perm", 0, 0xc000}, {"perm", 2, 0xc100}, {"level", 0, 0xc100}, {"level", 2, 0xc900}, {"frame", 0, 0xc900}, {"frame", 2, 0xcd00}, {"perm", 0, 0xc004}, {"perm", 0, 0xc008}, {"frame", 0, 0xc940}, {"frame", 0, 0xc900}, {"frame", 0, 0xc920}} {
				st = append(st, storeField(x.k, x.o, x.v)...)
			}
			run("test_split_and_alloc", nil, in, out, st, []uint16{0}, 0)
			state := arena20State{17, 29}
			run("init_a", nil, map[string]arena20State{"a": state}, map[string]arena20State{"a": {0xc000, 0xc400}}, append(storeField("a", 0, 0xc000), storeField("a", 2, 0xc400)...), []uint16{1024}, 0)
			state = arena20State{0xc000, 0xc400}
			for i := 0; i < 258; i++ {
				next := state.ptr + 4
				want := uint16(0)
				var st []struct {
					key    string
					offset int
					v      byte
				}
				old := state
				if next <= state.end {
					want = state.ptr
					state.ptr = next
					st = storeField("a", 0, next)
				}
				run("alloc_enemy", nil, map[string]arena20State{"a": old}, map[string]arena20State{"a": state}, st, []uint16{want}, 0)
			}
			for _, vm := range []*mir2.VM{raw, final} {
				vm.ObserveDataAccess = nil
				for addr := 0; addr < 65536; addr++ {
					object := false
					for _, p := range []uint16{0x4000, 0xfffe, 0xffff} {
						if uint16(uint16(addr)-p) < 4 {
							object = true
						}
					}
					global := false
					for _, g := range globalKeys {
						p := uint16(vm.GlobalAddr(g))
						if uint16(uint16(addr)-p) < 4 {
							global = true
						}
					}
					if !object && !global && vm.ReadHeap(int64(addr), 1)[0] != byte(addr*73+29) {
						t.Fatalf("fullVMRAM changed%x", addr)
					}
				}
			}
			for addr, v := range before {
				if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if z.Memory()[addr] != v {
					t.Fatalf("fullRAMchanged%x", addr)
				}
			}
			for addr, w := range profiler.WriteCount {
				if w > 0 && !allowed[addr] && !(addr >= int(minSP) && addr < 0xf000) {
					t.Fatalf("writeoutside%x", addr)
				}
			}
			summary, _ := json.MarshalIndent(map[string]any{"source_sha256": sha, "entries": counts, "module_bytes": len(a.Binary), "workload_tstates": cost, "stack_depths": depths}, "", "  ")
			export(mode.name+"-results.json", summary)
			t.Log(string(summary))
		})
	}
}
