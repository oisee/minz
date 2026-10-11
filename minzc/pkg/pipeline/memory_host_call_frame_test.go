package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"strings"
	"testing"
)

func snapshotTestFixture(t *testing.T) (*z80asm.Result, string, mir2.NativeCallFrameSnapshot) {
	t.Helper()
	d := mir2.NativeCallFrameSnapshot{ID: "test", Function: "f", Block: "entry", CallerSaved: []string{"AF", "BC", "DE", "HL"}, SnapshotPairs: []string{"AF"}, Results: []mir2.NativeCallFrameResult{{Source: "A", Destination: "C", Width: 8}}}
	assembly := []string{"PUSH IX", "PUSH AF", "LD IX, 0", "ADD IX, SP", "LD A, (IX+1)", "LD (IX+8), A", "PUSH HL", "LD L, (IX+0)", "LD H, (IX+1)", "PUSH HL", "POP AF", "POP HL", "INC SP", "INC SP", "POP IX", "POP HL", "POP DE", "POP BC", "POP AF"}
	var s strings.Builder
	s.WriteString("ORG 0x400\n")
	for j, a := range assembly {
		id := fmt.Sprintf("test_s%d", j)
		d.Instructions = append(d.Instructions, mir2.NativeCallFrameInstruction{ID: id, Assembly: a})
		fmt.Fprintf(&s, " %s ; @minz-call-frame:%s\n", a, id)
	}
	a, e := z80asm.NewAssembler().AssembleString(s.String())
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	return a, s.String(), d
}
func TestMemoryHostCallSnapshotBinding(t *testing.T) {
	a, assembly, d := snapshotTestFixture(t)
	_, r, e := resolveMemoryHostCallFrames(a, assembly, []mir2.NativeCallFrameSnapshot{d})
	if e != nil || len(r) != 1 {
		t.Fatal(e)
	}
	for _, name := range []string{"missing", "duplicate", "slot", "return_escape", "wrong_opcode", "saved_pair", "orphan"} {
		t.Run(name, func(t *testing.T) {
			a, assembly, d := snapshotTestFixture(t)
			switch name {
			case "missing":
				d.Instructions[4].ID = "missing"
			case "duplicate":
				d.Instructions[4].ID = d.Instructions[3].ID
			case "slot":
				a.Listing[5].Bytes[2] = 7
			case "return_escape":
				a.Listing[5].Bytes[2] = 12
			case "wrong_opcode":
				a.Listing[4].Bytes[1] = 0x77
			case "saved_pair":
				d.CallerSaved[0] = "HL"
			case "orphan":
				a.Listing = append(a.Listing, a.Listing[4])
			}
			if _, _, e := resolveMemoryHostCallFrames(a, assembly, []mir2.NativeCallFrameSnapshot{d}); e == nil {
				t.Fatal("forged snapshot accepted")
			}
		})
	}
}
func TestMemoryHostCallSnapshotRuntime(t *testing.T) {
	for _, mode := range []string{"positive", "uninitialized", "wrong_role", "wrong_ix", "adjacent_region", "return_slot"} {
		t.Run(mode, func(t *testing.T) {
			a, assembly, d := snapshotTestFixture(t)
			steps, _, e := resolveMemoryHostCallFrames(a, assembly, []mir2.NativeCallFrameSnapshot{d})
			if e != nil {
				t.Fatal(e)
			}
			m := &memoryHostZ80Memory{lengths: map[uint16]int{}, snapshots: steps}
			copy(m.data[0x400:], a.Binary)
			for _, l := range a.Listing {
				if len(l.Bytes) > 0 {
					m.lengths[uint16(l.Address)] = len(l.Bytes)
					for k := range l.Bytes {
						m.executable[uint16(l.Address+k)] = true
					}
				}
			}
			for p := 0xe000; p < 0xeff0; p++ {
				m.stack[p] = true
				m.readable[p] = true
				m.writable[p] = true
			}
			m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
			m.cpu.SetPC(0x400)
			m.cpu.SetSP(0xe100)
			m.cpu.A = 0x5a
			m.cpu.SetHL(0x1234)
			m.cpu.SetIX(0x2345)
			for j, p := range reverseSnapshotPairs(d.CallerSaved) {
				addr := uint16(0xe100 + 2*j)
				m.stackPair[addr] = p
				m.stackInitialized[addr] = true
				m.stackInitialized[addr+1] = true
				m.data[addr] = byte(j)
				m.data[addr+1] = byte(j + 1)
			}
			switch mode {
			case "uninitialized":
				m.stackInitialized[0xe100] = false
			case "wrong_role":
				m.stackPair[0xe100] = "AF"
			case "adjacent_region":
				m.stack[0xe0ff] = false
				m.region[0xe0ff] = "adjacent"
			case "return_slot":
				m.callFrames = []memoryHostCallFrame{{SP: 0xe100, Return: 0x100}}
			}
			m.active = true
			failed := false
			for j := 0; j < len(d.Instructions); j++ {
				if mode == "wrong_ix" && j == 4 {
					m.cpu.SetIX(m.cpu.IX() + 1)
				}
				if _, e := memoryHostNativeStep(m); e != nil {
					failed = true
					break
				}
			}
			if mode == "positive" {
				if failed || m.activeSnapshot != nil || m.cpu.SP() != 0xe108 || m.cpu.C != 0x5a {
					t.Fatalf("snapshot restoration failed %v SP%x C%x", failed, m.cpu.SP(), m.cpu.C)
				}
			} else if !failed {
				t.Fatal("invalid snapshot accepted")
			}
			if mode == "adjacent_region" && len(m.events) != 0 {
				t.Fatal("out of frame write occurred")
			}
		})
	}
}
