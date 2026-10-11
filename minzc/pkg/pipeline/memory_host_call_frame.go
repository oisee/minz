package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
)

type MemoryHostResolvedCallFrame struct {
	Descriptor mir2.NativeCallFrameSnapshot `json:"descriptor"`
	PCs        []uint16                     `json:"pcs"`
	Opcodes    [][]byte                     `json:"opcodes"`
}
type memoryHostSnapshotStep struct {
	Frame                     *MemoryHostResolvedCallFrame
	Index, Delta, AnchorDelta int
	Slot                      int
	Write                     bool
	Anchor, Retire            bool
}
type memoryHostActiveSnapshot struct {
	Frame         *MemoryHostResolvedCallFrame
	Entry, Anchor uint16
	Next          int
}

func snapshotPair(name string) string {
	switch name {
	case "A", "F":
		return "AF"
	case "B", "C":
		return "BC"
	case "D", "E":
		return "DE"
	case "H", "L":
		return "HL"
	case "AF", "BC", "DE", "HL", "IX", "IY":
		return name
	}
	return ""
}
func snapshotHigh(name string) bool { return name == "A" || name == "B" || name == "D" || name == "H" }
func snapshotPushPair(code []byte) (string, bool) {
	prefix := byte(0)
	if len(code) == 2 && (code[0] == 0xdd || code[0] == 0xfd) {
		prefix = code[0]
		code = code[1:]
	}
	if len(code) != 1 || code[0]&0xcf != 0xc5 {
		return "", false
	}
	pair := []string{"BC", "DE", "HL", "AF"}[(code[0]>>4)&3]
	if pair == "HL" && prefix != 0 {
		if prefix == 0xdd {
			pair = "IX"
		} else {
			pair = "IY"
		}
	}
	return pair, true
}
func snapshotPop(code []byte) bool {
	prefix := len(code) == 2 && (code[0] == 0xdd || code[0] == 0xfd)
	if prefix {
		code = code[1:]
	}
	return len(code) == 1 && code[0]&0xcf == 0xc1
}

// Resolve a finite instruction sequence and independently derive its stack
// geometry and result destinations. Descriptor text alone grants no memory.
func resolveMemoryHostCallFrames(a *z80asm.Result, assembly string, descriptors []mir2.NativeCallFrameSnapshot) (map[uint16]memoryHostSnapshotStep, []MemoryHostResolvedCallFrame, error) {
	if e := mir2.CheckNativeCallFrameMarkers(assembly, descriptors); e != nil {
		return nil, nil, e
	}
	result := map[uint16]memoryHostSnapshotStep{}
	resolved := make([]MemoryHostResolvedCallFrame, 0, len(descriptors))
	seen := map[string]bool{}
	for _, d := range descriptors {
		if d.ID == "" || seen[d.ID] || len(d.SnapshotPairs) < 1 || d.SnapshotPairs[0] != "AF" || len(d.Instructions) < 6 {
			return nil, nil, fmt.Errorf("invalid call snapshot")
		}
		seen[d.ID] = true
		snapshots, restores := map[string]int{}, map[string]int{}
		base := 2 * len(d.SnapshotPairs)
		snapshotSeen := map[string]bool{}
		for j, p := range d.SnapshotPairs {
			if snapshotPair(p) != p || snapshotSeen[p] {
				return nil, nil, fmt.Errorf("invalid snapshot pair")
			}
			snapshotSeen[p] = true
			snapshots[p] = 2 * (len(d.SnapshotPairs) - 1 - j)
		}
		pairseen := map[string]bool{}
		for j, p := range d.CallerSaved {
			if snapshotPair(p) != p || pairseen[p] {
				return nil, nil, fmt.Errorf("invalid caller pair")
			}
			pairseen[p] = true
			restores[p] = base + 2 + 2*(len(d.CallerSaved)-1-j)
		}
		writes := map[int]bool{}
		reads := map[int]bool{}
		for _, o := range snapshots {
			reads[o] = true
			reads[o+1] = true
		}
		for _, r := range d.Results {
			sp, dp := snapshotPair(r.Source), snapshotPair(r.Destination)
			so, ok := snapshots[sp]
			if !ok || (r.Width != 8 && r.Width != 16) || dp == "" {
				return nil, nil, fmt.Errorf("unsupported snapshot result")
			}
			dest, patch := restores[dp]
			if dp == "IX" && !patch {
				dest = base
				patch = true
			}
			if r.Width == 16 {
				if r.Source != sp || r.Destination != dp {
					return nil, nil, fmt.Errorf("bad word snapshot result")
				}
				if patch {
					writes[dest] = true
					writes[dest+1] = true
				}
			} else {
				if r.Source == sp || r.Destination == dp {
					return nil, nil, fmt.Errorf("pair used as byte result")
				}
				if snapshotHigh(r.Source) {
					so++
				}
				_ = so
				if patch {
					if snapshotHigh(r.Destination) {
						dest++
					}
					writes[dest] = true
				}
				if r.Destination == "A" && !pairseen["AF"] {
					writes[snapshots["AF"]+1] = true
				}
			}
		}
		frame := MemoryHostResolvedCallFrame{Descriptor: d}
		lines := []*z80asm.ListingLine{}
		ids := map[string]bool{}
		for _, i := range d.Instructions {
			if ids[i.ID] || i.ID == "" {
				return nil, nil, fmt.Errorf("duplicate snapshot step")
			}
			ids[i.ID] = true
			marker := "@minz-call-frame:" + i.ID
			var found *z80asm.ListingLine
			for j := range a.Listing {
				l := &a.Listing[j]
				parts := strings.SplitN(l.SourceLine, ";", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) == marker {
					if len(parts) != 2 || strings.TrimSpace(parts[1]) != marker || strings.TrimSpace(parts[0]) != i.Assembly || found != nil {
						return nil, nil, fmt.Errorf("malformed/duplicate snapshot marker")
					}
					found = l
				}
			}
			// ADD IX,SP assembler listing canonicalization drops its comment.
			// Recover only this mandatory anchor at the exact next contiguous
			// address, while checking the marker in the exact original assembly argument.
			if found == nil && i.Assembly == "ADD IX, SP" && len(lines) > 0 {
				next := lines[len(lines)-1].Address + len(lines[len(lines)-1].Bytes)
				for j := range a.Listing {
					l := &a.Listing[j]
					if l.Address == next && bytes.Equal(l.Bytes, []byte{0xdd, 0x39}) {
						found = l
					}
				}
			}
			if found == nil || len(found.Bytes) == 0 || found.Address < 0 || found.Address+len(found.Bytes) > 65536 {
				return nil, nil, fmt.Errorf("missing snapshot step %s", i.ID)
			}
			expected, e := z80asm.NewAssembler().AssembleString("ORG 0\n " + i.Assembly + "\n")
			if e != nil || len(expected.Errors) > 0 || !bytes.Equal(expected.Binary, found.Bytes) {
				return nil, nil, fmt.Errorf("snapshot instruction bytes differ from bound operation")
			}
			if len(lines) > 0 && lines[len(lines)-1].Address+len(lines[len(lines)-1].Bytes) != found.Address {
				return nil, nil, fmt.Errorf("noncontiguous snapshot")
			}
			lines = append(lines, found)
			frame.PCs = append(frame.PCs, uint16(found.Address))
			frame.Opcodes = append(frame.Opcodes, append([]byte{}, found.Bytes...))
		}
		// The prologue must capture IX and every listed pair in exact order.
		prologue := append([]string{"IX"}, d.SnapshotPairs...)
		for j, p := range prologue {
			actual, ok := snapshotPushPair(lines[j].Bytes)
			if !ok || actual != p {
				return nil, nil, fmt.Errorf("snapshot push roles differ")
			}
		}
		anchorIndex := len(prologue) + 1
		if len(lines) <= anchorIndex || !bytes.Equal(lines[len(prologue)].Bytes, []byte{0xdd, 0x21, 0, 0}) || !bytes.Equal(lines[anchorIndex].Bytes, []byte{0xdd, 0x39}) {
			return nil, nil, fmt.Errorf("missing IX stack anchor")
		}
		anchorDelta := -base - 2
		delta := 0
		retired := 0
		steps := []memoryHostSnapshotStep{}
		for j, l := range lines {
			code := l.Bytes
			s := memoryHostSnapshotStep{Index: j, Delta: delta, AnchorDelta: anchorDelta, Slot: -1, Anchor: j == anchorIndex}
			if _, ok := snapshotPushPair(code); ok {
				delta -= 2
			} else if snapshotPop(code) {
				delta += 2
			} else if len(code) == 1 && code[0] == 0x33 {
				if delta != anchorDelta+retired || retired >= base {
					return nil, nil, fmt.Errorf("invalid snapshot discard")
				}
				s.Retire = true
				retired++
				delta++
			} else if bytes.Equal(code, []byte{0xdd, 0x21, 0, 0}) {
				if j != len(prologue) {
					return nil, nil, fmt.Errorf("IX reset outside anchor")
				}
			} else if bytes.Equal(code, []byte{0xdd, 0x39}) {
				if j != anchorIndex {
					return nil, nil, fmt.Errorf("IX changed outside anchor")
				}
			} else if len(code) == 3 && code[0] == 0xdd && (code[1]&0xc7 == 0x46 || code[1]&0xf8 == 0x70) {
				s.Slot = int(int8(code[2]))
				s.Write = code[1]&0xf8 == 0x70
				if j <= anchorIndex || s.Slot < 0 || s.Slot >= base+2+2*len(d.CallerSaved) || s.Write && !writes[s.Slot] || !s.Write && !reads[s.Slot] {
					return nil, nil, fmt.Errorf("unauthorized snapshot slot")
				}
				if s.Write && code[1] != 0x77 {
					return nil, nil, fmt.Errorf("snapshot store scratch must be A")
				}
			} else if len(code) == 1 && code[0] >= 0x40 && code[0] <= 0x7f && code[0] != 0x76 && code[0]&7 != 6 && (code[0]>>3)&7 != 6 {
				// Register-only moves may bridge an indexed source to a returned register.
			} else {
				return nil, nil, fmt.Errorf("unsupported snapshot instruction %x", code)
			}
			steps = append(steps, s)
		}
		if retired != base || delta != 2*len(d.CallerSaved) {
			return nil, nil, fmt.Errorf("unclosed snapshot geometry")
		}
		// Final restores consume only the saved IX and listed caller-save pairs.
		suffix := append([]string{"IX"}, reverseSnapshotPairs(d.CallerSaved)...)
		for j, p := range suffix {
			l := lines[len(lines)-len(suffix)+j]
			push := append([]byte{}, l.Bytes...)
			push[len(push)-1] += 4
			actual, ok := snapshotPushPair(push)
			if !ok || actual != p {
				return nil, nil, fmt.Errorf("snapshot restore roles differ")
			}
		}
		resolved = append(resolved, frame)
		f := &resolved[len(resolved)-1]
		for j, s := range steps {
			pc := f.PCs[j]
			if _, ok := result[pc]; ok {
				return nil, nil, fmt.Errorf("overlapping snapshots")
			}
			s.Frame = f
			result[pc] = s
		}
	}
	return result, resolved, nil
}
func reverseSnapshotPairs(p []string) []string {
	r := append([]string{}, p...)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

func (m *memoryHostZ80Memory) beginSnapshot(pc uint16) {
	step, ok := m.snapshots[pc]
	if !ok {
		if m.activeSnapshot != nil {
			panic(memoryHostNativeTrap{"snapshot control escaped finite sequence"})
		}
		return
	}
	if step.Index == 0 {
		if m.activeSnapshot != nil {
			panic(memoryHostNativeTrap{"nested snapshot entry"})
		}
		entry := m.cpu.SP()
		size := 2 * len(step.Frame.Descriptor.CallerSaved)
		for j, p := range reverseSnapshotPairs(step.Frame.Descriptor.CallerSaved) {
			address := int(entry) + 2*j
			if address+1 >= 65536 || !m.stack[uint16(address)] || !m.stack[uint16(address+1)] || m.stackPair[uint16(address)] != p || !m.stackInitialized[uint16(address)] || !m.stackInitialized[uint16(address+1)] {
				panic(memoryHostNativeTrap{"snapshot entry lacks initialized caller-save roles"})
			}
		}
		for _, call := range m.callFrames {
			if int(call.SP) < int(entry)+size && int(call.SP)+2 > int(entry) {
				panic(memoryHostNativeTrap{"snapshot overlaps return frame"})
			}
		}
		m.activeSnapshot = &memoryHostActiveSnapshot{Frame: step.Frame, Entry: entry}
	}
	active := m.activeSnapshot
	if active == nil || active.Frame != step.Frame || active.Next != step.Index || int(m.cpu.SP()) != int(active.Entry)+step.Delta {
		panic(memoryHostNativeTrap{"snapshot step identity/SP mismatch"})
	}
	if step.Index > len(step.Frame.Descriptor.SnapshotPairs)+2 && step.Index < len(step.Frame.PCs)-len(step.Frame.Descriptor.CallerSaved)-1 && m.cpu.IX() != active.Anchor {
		panic(memoryHostNativeTrap{"snapshot IX anchor changed"})
	}
	if step.Slot >= 0 {
		address := int(active.Entry) + step.AnchorDelta + step.Slot
		if address < 0 || address >= 65536 || !m.stack[uint16(address)] || !m.stackInitialized[uint16(address)] {
			panic(memoryHostNativeTrap{"snapshot slot outside initialized frame"})
		}
		for _, call := range m.callFrames {
			if uint16(address) == call.SP || uint16(address) == call.SP+1 {
				panic(memoryHostNativeTrap{"snapshot slot is return address"})
			}
		}
		recipe, e := memoryHostMemoryRecipe(m, pc)
		if e != nil || len(recipe) != 1 || recipe[0].Address != uint16(address) || recipe[0].Write != step.Write {
			panic(memoryHostNativeTrap{"snapshot opcode effective slot mismatch"})
		}
		m.pending = append(m.pending, recipe...)
		m.hasProtectedRecipe = true
		m.snapshotData = true
		if step.Write {
			m.frameWrites = map[uint16]byte{uint16(address): m.cpu.A}
		}
	}
	if step.Retire {
		address := m.cpu.SP()
		if !m.stack[address] || !m.stackInitialized[address] {
			panic(memoryHostNativeTrap{"discard uninitialized snapshot byte"})
		}
	}
}
func (m *memoryHostZ80Memory) finishSnapshot() error {
	step, ok := m.snapshots[m.instructionPC]
	if !ok {
		return nil
	}
	active := m.activeSnapshot
	if active == nil {
		return fmt.Errorf("missing active snapshot")
	}
	expectedDelta := step.Delta
	code := step.Frame.Opcodes[step.Index]
	if pair, ok := snapshotPushPair(code); ok {
		expectedDelta -= 2
		m.stackPair[m.cpu.SP()] = pair
	} else if snapshotPop(code) {
		expectedDelta += 2
		m.stackPair[m.instructionSP] = ""
	} else if step.Retire {
		expectedDelta++
		m.stackInitialized[m.instructionSP] = false
		m.stackPair[m.instructionSP] = ""
	}
	if int(m.cpu.SP()) != int(active.Entry)+expectedDelta {
		return fmt.Errorf("snapshot post-SP differs")
	}
	if step.Anchor {
		active.Anchor = m.cpu.IX()
		if int(active.Anchor) != int(active.Entry)+step.AnchorDelta {
			return fmt.Errorf("snapshot anchor differs")
		}
	}
	active.Next++
	if active.Next == len(step.Frame.PCs) {
		m.activeSnapshot = nil
	} else if m.cpu.PC() != step.Frame.PCs[active.Next] {
		return fmt.Errorf("snapshot post-PC differs")
	}
	return nil
}
