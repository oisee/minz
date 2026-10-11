package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"reflect"
	"testing"
)

func TestMemoryHostSidecarOpcodeBinding(t *testing.T) {
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x400\n LD HL,(g) ; @minz-memory-access:one\n RET\ng: DW 7\n")
	if e != nil || len(a.Errors) != 0 {
		t.Fatal(e)
	}
	record := mir2.NativeMemoryAccess{ID: "one", Root: mir2.GlobalMemoryOrigin{Symbol: "g", Width: 2}, Bytes: 2, StorageKind: "global"}
	recipes, _, e := resolveMemoryHostSidecar(a, []mir2.NativeMemoryAccess{record})
	if e != nil || len(recipes[0x400]) != 2 {
		t.Fatal(e)
	}
	for _, name := range []string{"direction", "width", "offset", "missing", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			r := record
			records := []mir2.NativeMemoryAccess{r}
			switch name {
			case "direction":
				r.Write = true
				r.Root.Write = true
			case "width":
				r.Bytes = 1
			case "offset":
				r.Root.Offset = 1
			case "missing":
				r.ID = "absent"
			case "duplicate":
				records = append(records, r)
			}
			if name != "duplicate" {
				records[0] = r
			}
			if _, _, e := resolveMemoryHostSidecar(a, records); e == nil {
				t.Fatal("forged sidecar accepted")
			}
		})
	}
}
func TestMemoryHostProtectedAccessRecipes(t *testing.T) {
	for _, kind := range []string{"global", "stack", "spill"} {
		for _, write := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "read", true: "write"}[write], func(t *testing.T) {
				m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 1}, active: true}
				m.data[0x400] = 0x7e
				if write {
					m.data[0x400] = 0x77
				}
				m.executable[0x400] = true
				m.readable[0x8000] = true
				m.writable[0x8000] = true
				switch kind {
				case "global":
					m.global[0x8000] = true
				case "stack":
					m.stack[0x8000] = true
				case "spill":
					m.spill[0x8000] = true
				}
				m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
				m.cpu.SetPC(0x400)
				m.cpu.H = 0x80
				if _, e := memoryHostNativeStep(m); e == nil {
					t.Fatal("numeric protected access accepted")
				}
			})
		}
	}
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3}, protected: map[uint16][]memoryHostByteRecipe{0x400: {{0x8000, false}, {0x8001, false}}}, active: true}
	copy(m.data[0x400:], []byte{0x2a, 0, 0x80})
	m.executable[0x400] = true
	for p := 0x8000; p < 0x8002; p++ {
		m.global[p] = true
		m.readable[p] = true
	}
	m.data[0x8000] = 0x34
	m.data[0x8001] = 0x12
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	if _, e := memoryHostNativeStep(m); e != nil || m.cpu.HL() != 0x1234 || !reflect.DeepEqual(m.pending, []memoryHostByteRecipe{}) {
		t.Fatal("protected word recipe", e)
	}
}

func TestMemoryHostProtectedMisdirectionBeforeEffect(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 1}, protected: map[uint16][]memoryHostByteRecipe{0x400: {{0x9000, true}}}, active: true}
	m.data[0x400] = 0x77
	m.executable[0x400] = true
	m.region[0x8000] = "ram"
	m.writable[0x8000] = true
	m.data[0x8000] = 7
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	m.cpu.H = 0x80
	m.cpu.A = 99
	if _, e := memoryHostNativeStep(m); e == nil || m.data[0x8000] != 7 || len(m.events) != 0 {
		t.Fatal("misdirected protected write mutated region", e)
	}
}
func TestMemoryHostOpcodeRejectsTruncation(t *testing.T) {
	for _, b := range [][]byte{{0x3a}, {0x22, 0}, {0xed, 0x4b}, {0xdd, 0x2a, 0}} {
		if e := validateMemoryHostOpcode(b, 1, false, 0); e == nil {
			t.Fatal("truncated descriptor opcode accepted", b)
		}
	}
}

func TestMemoryHostUninitializedStackPopRefusal(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 1}, active: true}
	m.data[0x400] = 0xe1
	m.executable[0x400] = true
	m.stack[0xe000] = true
	m.stack[0xe001] = true
	m.readable[0xe000] = true
	m.readable[0xe001] = true
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	m.cpu.SetSP(0xe000)
	if _, e := memoryHostNativeStep(m); e == nil {
		t.Fatal("uninitialized stack pop accepted")
	}
}
