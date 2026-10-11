package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/remogatto/z80"
	"reflect"
	"testing"
)

// These controls call the permission preflight without executing a CPU step.
// Invalid activation metadata must refuse before any RAM effect.
func TestCallSpillLifecycleRefusals(t *testing.T) {
	for _, name := range []string{"positive", "middle", "wrong_sp", "uninitialized", "return_overlap", "wrong_call", "wrong_depth", "wrong_slot", "corrupt_value", "no_call", "no_saved"} {
		t.Run(name, func(t *testing.T) {
			m := &memoryHostZ80Memory{}
			m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
			pc, sp := uint16(0x400), uint16(0xe100)
			f := &MemoryHostResolvedCallSpill{Descriptor: mir2.NativeCallSpillSnapshot{ID: "restore", CallID: "call", Symbol: "home", Width: 1, Stage: "restore"}, PCs: []uint16{pc}, Opcodes: [][]byte{{0xe3}}}
			m.callSpills = map[uint16]memoryHostCallSpillStep{pc: {Frame: f, Index: 0, Delta: 0}}
			m.callFrames = []memoryHostCallFrame{{SP: 0xe200, Return: 0x100}}
			m.cpu.SetSP(sp)
			m.stack[sp] = true
			m.stack[sp+1] = true
			m.stackInitialized[sp] = true
			m.stackInitialized[sp+1] = true
			m.stackPair[sp] = "HL"
			m.data[sp] = 7
			m.data[pc] = 0xe3
			m.cpu.SetPC(pc)
			m.instructionPC = pc
			m.instructionSP = sp
			// Bind only fields common to the current final runtime; explicit new
			// activation-call fields are set by the fixture helper below.
			m.savedCallSpills = []memoryHostSavedCallSpill{{CallID: "call", Symbol: "home", Width: 1, Depth: 1, Slot: sp}}
			primeCallSpillSavedFixture(m, f)
			switch name {
			case "middle":
				m.callSpills[pc] = memoryHostCallSpillStep{Frame: f, Index: 1}
			case "wrong_sp":
				m.activeCallSpill = &memoryHostActiveCallSpill{Frame: f, Entry: sp + 2, Next: 1}
				m.callSpills[pc] = memoryHostCallSpillStep{Frame: f, Index: 1, Delta: 0}
			case "uninitialized":
				m.stackInitialized[sp+1] = false
			case "return_overlap":
				m.callFrames[0].SP = sp
			case "wrong_call":
				m.savedCallSpills[0].CallID = "different"
			case "wrong_depth":
				m.savedCallSpills[0].Depth = 2
			case "wrong_slot":
				m.savedCallSpills[0].Slot = sp + 2
			case "corrupt_value":
				m.data[sp] = 8
			case "no_call":
				clearCallSpillDidCall(m)
			case "no_saved":
				m.savedCallSpills = nil
			}
			before := m.data
			rejected := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						if _, ok := r.(memoryHostNativeTrap); !ok {
							t.Fatalf("unexpected panic %T %v", r, r)
						}
						rejected = true
					}
				}()
				m.beginCallSpill(pc)
			}()
			if name == "positive" {
				if rejected {
					t.Fatal("valid preflight refused")
				}
				if m.activeCallSpill == nil || len(m.pending) != 4 {
					t.Fatal("valid recipe not installed")
				}
			} else if !rejected {
				t.Fatal("invalid activation accepted")
			}
			if !reflect.DeepEqual(before, m.data) {
				t.Fatal("RAM changed before refusal")
			}
		})
	}
}

func primeCallSpillSavedFixture(m *memoryHostZ80Memory, f *MemoryHostResolvedCallSpill) {
	m.savedCallSpills[0].CallPC = f.CallPC
	m.savedCallSpills[0].Value = 7
	m.savedCallSpills[0].DidCall = true
}
func clearCallSpillDidCall(m *memoryHostZ80Memory) { m.savedCallSpills[0].DidCall = false }
