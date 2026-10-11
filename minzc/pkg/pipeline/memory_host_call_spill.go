package pipeline

import (
	"bytes"
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

type MemoryHostResolvedCallSpill struct {
	Descriptor mir2.NativeCallSpillSnapshot `json:"descriptor"`
	CallPC     uint16                       `json:"call_pc"`
	PCs        []uint16                     `json:"pcs"`
	Opcodes    [][]byte                     `json:"opcodes"`
}
type memoryHostCallSpillStep struct {
	Frame        *MemoryHostResolvedCallSpill
	Index, Delta int
}
type memoryHostSavedCallSpill struct {
	CallID, Symbol string
	Width, Depth   int
	Slot           uint16
	Value          uint16
	CallPC         uint16
	DidCall        bool
}
type memoryHostActiveCallSpill struct {
	Frame            *MemoryHostResolvedCallSpill
	Entry            uint16
	Next             int
	OldHL, SlotValue uint16
	OldF             byte
}

func resolveMemoryHostCallSpills(a *z80asm.Result, assembly string, ds []mir2.NativeCallSpillSnapshot, module *mir2.Module, allocation *mir2.AllocResult) (map[uint16]memoryHostCallSpillStep, []MemoryHostResolvedCallSpill, error) {
	if err := mir2.CheckNativeCallSpillMarkers(assembly, ds); err != nil {
		return nil, nil, err
	}
	resolved := make([]MemoryHostResolvedCallSpill, 0, len(ds))
	steps := map[uint16]memoryHostCallSpillStep{}
	seen := map[string]bool{}
	pairs := map[string]map[string]bool{}
	for _, d := range ds {
		if d.ID == "" || seen[d.ID] || d.CallID == "" || d.Width < 1 || d.Width > 2 || (d.Stage != "save" && d.Stage != "restore") {
			return nil, nil, fmt.Errorf("invalid call-spill descriptor")
		}
		seen[d.ID] = true
		f := module.FuncByName(d.Function)
		if f == nil {
			return nil, nil, fmt.Errorf("call-spill function missing")
		}
		var block *mir2.Block
		for _, b := range f.Blocks {
			if b.Label == d.Block {
				block = b
				break
			}
		}
		if block == nil || d.CallInstruction < 0 || d.CallInstruction >= len(block.Insts) {
			return nil, nil, fmt.Errorf("call-spill call detached")
		}
		call := block.Insts[d.CallInstruction]
		if call.Op != mir2.OpCall && call.Op != mir2.OpCallIndirect && call.Op != mir2.OpCallCond {
			return nil, nil, fmt.Errorf("call-spill root not call")
		}
		r := mir2.Reg(d.HomeReg)
		fi, bi := -1, -1
		for j, fn := range module.Funcs {
			if fn == f {
				fi = j
			}
		}
		for j, b := range f.Blocks {
			if b == block {
				bi = j
			}
		}
		if d.CallID != fmt.Sprintf("f%d_b%d_i%d", fi, bi, d.CallInstruction) {
			return nil, nil, fmt.Errorf("call-spill canonical call identity")
		}
		live := map[mir2.Reg]bool{}
		lr := mir2.ComputeLiveness(f)
		if out := lr.LiveOutOf(f, block); out != nil {
			for _, rr := range out.Slice() {
				live[rr] = true
			}
		}
		for _, rr := range mir2.NativeTermUses(block.Term) {
			live[rr] = true
		}
		for j := len(block.Insts) - 1; j > d.CallInstruction; j-- {
			i := block.Insts[j]
			delete(live, i.Dst)
			for _, rr := range i.ExtraRets {
				delete(live, rr)
			}
			for _, rr := range i.Uses() {
				live[rr] = true
			}
		}
		delete(live, call.Dst)
		for _, rr := range call.ExtraRets {
			delete(live, rr)
		}
		if !live[r] {
			return nil, nil, fmt.Errorf("call-spill home not live across call")
		}
		info, ok := mir2.CollectRegInfoForTest(f)[r]
		if !ok || d.Symbol != mir2.Z80SpillLabel(f.Name, r) || mir2.NativeSpillWidth(info.Ty) != d.Width {
			return nil, nil, fmt.Errorf("call-spill home type/symbol")
		}
		// A generated bool home may be dynamically allocated by lowering; the
		// independently checked spill sidecar binds the actual memory operation.
		if allocation.Loc(r).Kind != mir2.LocMem {
			return nil, nil, fmt.Errorf("call-spill generated override unsupported by this certificate")
		}
		key := d.CallID + ":" + d.Symbol
		if pairs[key] == nil {
			pairs[key] = map[string]bool{}
		}
		if pairs[key][d.Stage] {
			return nil, nil, fmt.Errorf("duplicate call-spill stage")
		}
		pairs[key][d.Stage] = true
		address, ok := workloadSymbol(a.Symbols, d.Symbol)
		if !ok || address < 0 || address+d.Width > 65536 {
			return nil, nil, fmt.Errorf("call-spill symbol extent")
		}
		lo, hi := byte(address), byte(address>>8)
		var expected [][]byte
		if d.Stage == "save" {
			if d.Width == 2 {
				expected = [][]byte{{0xe5}, {0x2a, lo, hi}, {0xe3}}
			} else {
				expected = [][]byte{{0xe5}, {0xf5}, {0x3a, lo, hi}, {0x6f}, {0xf1}, {0xe3}}
			}
		} else {
			if d.Width == 2 {
				expected = [][]byte{{0xe3}, {0x22, lo, hi}, {0xe1}}
			} else {
				expected = [][]byte{{0xe3}, {0xf5}, {0x7d}, {0x32, lo, hi}, {0xf1}, {0xe1}}
			}
		}
		if len(expected) != len(d.Instructions) {
			return nil, nil, fmt.Errorf("call-spill sequence length")
		}
		var callLine *z80asm.ListingLine
		for li := range a.Listing {
			l := &a.Listing[li]
			if mir2.NativeAnnotationCount(l.SourceLine, "@minz-spill-call:"+d.CallID) == 1 {
				if callLine != nil {
					return nil, nil, fmt.Errorf("duplicate spill call marker")
				}
				callLine = l
			}
		}
		if callLine == nil || len(callLine.Bytes) != 3 || callLine.Bytes[0] != 0xcd {
			return nil, nil, fmt.Errorf("missing direct spill call opcode")
		}
		callOffset := callLine.Address - a.Origin
		if callOffset < 0 || callOffset+3 > len(a.Binary) || !bytes.Equal(a.Binary[callOffset:callOffset+3], callLine.Bytes) {
			return nil, nil, fmt.Errorf("call-spill call listing differs binary")
		}
		target, ok := workloadSymbol(a.Symbols, call.Sym)
		if !ok || target < 0 || target > 65535 || int(callLine.Bytes[1])|int(callLine.Bytes[2])<<8 != target {
			return nil, nil, fmt.Errorf("call-spill call target differs MIR")
		}
		frame := MemoryHostResolvedCallSpill{Descriptor: d, CallPC: uint16(callLine.Address)}
		delta := 0
		deltas := []int{}
		for j, i := range d.Instructions {
			var line *z80asm.ListingLine
			for li := range a.Listing {
				l := &a.Listing[li]
				if mir2.NativeAnnotationCount(l.SourceLine, "@minz-call-spill:"+i.ID) == 1 && len(l.Bytes) > 0 {
					if line != nil {
						return nil, nil, fmt.Errorf("duplicate call-spill listing")
					}
					line = l
				}
			}
			if line == nil || !bytes.Equal(line.Bytes, expected[j]) || line.Address < 0 || line.Address+len(line.Bytes) > 65536 {
				return nil, nil, fmt.Errorf("call-spill opcode/marker mismatch")
			}
			offset := line.Address - a.Origin
			if offset < 0 || offset+len(line.Bytes) > len(a.Binary) || !bytes.Equal(a.Binary[offset:offset+len(line.Bytes)], line.Bytes) {
				return nil, nil, fmt.Errorf("call-spill listing differs binary")
			}
			if j > 0 && line.Address != int(frame.PCs[j-1])+len(frame.Opcodes[j-1]) {
				return nil, nil, fmt.Errorf("call-spill sequence discontinuity")
			}
			frame.PCs = append(frame.PCs, uint16(line.Address))
			frame.Opcodes = append(frame.Opcodes, append([]byte{}, line.Bytes...))
			deltas = append(deltas, delta)
			if expected[j][0] == 0xe5 || expected[j][0] == 0xf5 {
				delta -= 2
			}
			if expected[j][0] == 0xe1 || expected[j][0] == 0xf1 {
				delta += 2
			}
		}
		if d.Stage == "save" && int(frame.PCs[len(frame.PCs)-1])+len(frame.Opcodes[len(frame.Opcodes)-1]) > callLine.Address || d.Stage == "restore" && int(frame.PCs[0]) < callLine.Address+3 {
			return nil, nil, fmt.Errorf("call-spill stage order differs call")
		}
		resolved = append(resolved, frame)
		fr := &resolved[len(resolved)-1]
		for j, pc := range fr.PCs {
			if _, ok := steps[pc]; ok {
				return nil, nil, fmt.Errorf("overlapping call-spill sequence")
			}
			steps[pc] = memoryHostCallSpillStep{fr, j, deltas[j]}
		}
	}
	for _, p := range pairs {
		if !p["save"] || !p["restore"] {
			return nil, nil, fmt.Errorf("unpaired call-spill stage")
		}
	}
	return steps, resolved, nil
}

func (m *memoryHostZ80Memory) beginCallSpill(pc uint16) {
	s, ok := m.callSpills[pc]
	if !ok {
		if m.activeCallSpill != nil {
			panic(memoryHostNativeTrap{"call-spill control escaped stage"})
		}
		return
	}
	d := s.Frame.Descriptor
	if s.Index == 0 {
		if m.activeCallSpill != nil {
			panic(memoryHostNativeTrap{"nested call-spill stage"})
		}
		entry := m.cpu.SP()
		if d.Stage == "restore" {
			if int(entry)+1 >= 65536 {
				panic(memoryHostNativeTrap{"call-spill restore wraps stack"})
			}
			if len(m.savedCallSpills) == 0 {
				panic(memoryHostNativeTrap{"restore without saved spill"})
			}
			last := m.savedCallSpills[len(m.savedCallSpills)-1]
			if last.CallID != d.CallID || last.Symbol != d.Symbol || last.Width != d.Width || last.Depth != len(m.callFrames) || last.Slot != entry || last.CallPC != s.Frame.CallPC || !last.DidCall || uint16(m.data[entry])|uint16(m.data[entry+1])<<8 != last.Value {
				panic(memoryHostNativeTrap{"call-spill restore activation/home mismatch"})
			}
		}
		m.activeCallSpill = &memoryHostActiveCallSpill{Frame: s.Frame, Entry: entry}
	}
	active := m.activeCallSpill
	if active == nil || active.Frame != s.Frame || active.Next != s.Index || int(m.cpu.SP()) != int(active.Entry)+s.Delta {
		panic(memoryHostNativeTrap{"call-spill stage identity/SP"})
	}
	if s.Frame.Opcodes[s.Index][0] == 0xe3 {
		sp := m.cpu.SP()
		if int(sp)+1 >= 65536 || !m.stack[sp] || !m.stack[sp+1] || !m.stackInitialized[sp] || !m.stackInitialized[sp+1] || m.stackPair[sp] != "HL" {
			panic(memoryHostNativeTrap{"call-spill exchange lacks initialized HL slot"})
		}
		for _, f := range m.callFrames {
			if sp == f.SP || sp+1 == f.SP || sp == f.SP+1 || sp+1 == f.SP+1 {
				panic(memoryHostNativeTrap{"call-spill overlaps return frame"})
			}
		}
		active.OldHL = m.cpu.HL()
		active.OldF = m.cpu.F
		active.SlotValue = uint16(m.data[sp]) | uint16(m.data[sp+1])<<8
		r, e := memoryHostMemoryRecipe(m, pc)
		if e != nil || len(r) != 4 {
			panic(memoryHostNativeTrap{"call-spill exchange recipe"})
		}
		m.pending = append(m.pending, r...)
		m.hasProtectedRecipe = true
		m.snapshotData = true
		m.frameWrites = map[uint16]byte{sp: byte(active.OldHL), sp + 1: byte(active.OldHL >> 8)}
	}
}

func (m *memoryHostZ80Memory) finishCallSpill() error {
	s, ok := m.callSpills[m.instructionPC]
	if !ok {
		return nil
	}
	a := m.activeCallSpill
	if a == nil {
		return fmt.Errorf("missing active call-spill")
	}
	op := s.Frame.Opcodes[s.Index][0]
	delta := s.Delta
	if op == 0xe5 || op == 0xf5 {
		delta -= 2
	}
	if op == 0xe1 || op == 0xf1 {
		delta += 2
	}
	if int(m.cpu.SP()) != int(a.Entry)+delta {
		return fmt.Errorf("call-spill post SP")
	}
	if op == 0xe3 && (m.cpu.HL() != a.SlotValue || m.cpu.F != a.OldF) {
		return fmt.Errorf("call-spill exchanged HL differs")
	}
	a.Next++
	if a.Next == len(s.Frame.PCs) {
		d := s.Frame.Descriptor
		if d.Stage == "save" {
			m.savedCallSpills = append(m.savedCallSpills, memoryHostSavedCallSpill{CallID: d.CallID, Symbol: d.Symbol, Width: d.Width, Depth: len(m.callFrames), Slot: m.cpu.SP(), Value: uint16(m.data[m.cpu.SP()]) | uint16(m.data[m.cpu.SP()+1])<<8, CallPC: s.Frame.CallPC})
		} else {
			m.savedCallSpills = m.savedCallSpills[:len(m.savedCallSpills)-1]
		}
		m.activeCallSpill = nil
	} else if m.cpu.PC() != s.Frame.PCs[a.Next] {
		return fmt.Errorf("call-spill post PC")
	}
	return nil
}
