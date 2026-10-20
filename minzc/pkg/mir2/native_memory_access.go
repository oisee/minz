package mir2

import (
	"fmt"
	"strings"
)

// GlobalMemoryOrigin is an independently supplied instruction identity proof.
// It authorizes semantic data accesses, never spills or scratch traffic.
type GlobalMemoryOrigin struct {
	Symbol      string `json:"symbol"`
	Offset      int    `json:"offset"`
	Width       int    `json:"width"`
	Write       bool   `json:"write"`
	Function    string `json:"function"`
	Block       string `json:"block"`
	Instruction int    `json:"instruction"`
}
type NativeMemoryAccess struct {
	ID          string             `json:"id"`
	Root        GlobalMemoryOrigin `json:"root"`
	ByteOffset  int                `json:"byte_offset"`
	Bytes       int                `json:"bytes"`
	Write       bool               `json:"write"`
	StorageKind string             `json:"storage_kind"`
}

func (g *z80cg) emitNativeMemory(inst *Inst, byteOffset, bytes int, s string) {
	if root, ok := g.nativeMemoryOrigins[inst]; ok {
		if byteOffset < 0 || bytes <= 0 || byteOffset+bytes > root.Width {
			if g.err == nil {
				g.err = fmt.Errorf("native memory access: emitted extent exceeds root")
			}
			return
		}
		id := fmt.Sprintf("f%d_b%d_i%d_a%d", g.nativeFunctionIndex, g.nativeBlockIndex, root.Instruction, g.nativeAccessCount[inst])
		g.nativeAccessCount[inst]++
		record := NativeMemoryAccess{ID: id, Root: root, ByteOffset: byteOffset, Bytes: bytes, Write: inst.Op == OpStore, StorageKind: "global"}
		*g.nativeMemorySink = append(*g.nativeMemorySink, record)
		s += " ; @minz-memory-access:" + id
	}
	g.emit(s)
}
func (g *z80cg) emitNativeMemoryf(inst *Inst, byteOffset, bytes int, format string, args ...any) {
	g.emitNativeMemory(inst, byteOffset, bytes, fmt.Sprintf(format, args...))
}

func validateNativeMemoryOrigins(m *Module, origins map[*Inst]GlobalMemoryOrigin, sink *[]NativeMemoryAccess) error {
	if origins == nil && sink == nil {
		return nil
	}
	if origins == nil || sink == nil {
		return fmt.Errorf("native memory access: origins and output sink required together")
	}
	for _, f := range m.Funcs {
		if strings.HasPrefix(sanitizeIdent(f.Name), "_spill_") || strings.HasPrefix(sanitizeIdent(f.Name), "__call_result_") {
			return fmt.Errorf("native memory access: source function uses protected spill namespace")
		}
	}
	for _, global := range m.Globals {
		if strings.HasPrefix(sanitizeIdent(global.Name), "_spill_") || strings.HasPrefix(sanitizeIdent(global.Name), "__call_result_") {
			return fmt.Errorf("native memory access: source global uses protected spill namespace")
		}
	}
	seen := map[*Inst]bool{}
	for fi, f := range m.Funcs {
		for bi, b := range f.Blocks {
			for ii, i := range b.Insts {
				if root, ok := origins[i]; ok {
					if seen[i] || root.Function != f.Name || root.Block != b.Label || root.Instruction != ii || root.Offset < 0 || root.Width != ByteWidth(i.Ty) || root.Width < 1 || root.Width > 2 || root.Write != (i.Op == OpStore) || (i.Op != OpLoad && i.Op != OpStore) {
						return fmt.Errorf("native memory access: invalid root f%d/b%d/i%d", fi, bi, ii)
					}
					size := 0
					for _, global := range m.Globals {
						if global.Name == root.Symbol {
							size = ByteWidth(global.Ty)
							if len(global.Init) > size {
								size = len(global.Init)
							}
						}
					}
					if size == 0 || root.Offset > size-root.Width {
						return fmt.Errorf("native memory access: root outside global")
					}
					seen[i] = true
				}
			}
		}
	}
	if len(seen) != len(origins) {
		return fmt.Errorf("native memory access: detached root")
	}
	// Sink contents belong to this call, not earlier compiler output.
	*sink = (*sink)[:0]
	return nil
}

// CheckNativeMemoryAccessMarkers fails closed when post-emission transformations
// remove, duplicate or orphan any recorded semantic access marker.
func CheckNativeMemoryAccessMarkers(assembly string, records []NativeMemoryAccess) error {
	for _, r := range records {
		if r.ID == "" || NativeAnnotationCount(assembly, "@minz-memory-access:"+r.ID) != 1 {
			return fmt.Errorf("native memory access: missing/duplicate marker %q", r.ID)
		}
	}
	if strings.Count(assembly, "@minz-memory-access:") != len(records) {
		return fmt.Errorf("native memory access: orphan marker")
	}
	return nil
}

func (g *z80cg) emitNativeSpill(symbol string, offset, bytes int, write bool, s string) {
	// A spill home is sized from its vreg type (z80SpillBytes). An access
	// wider than that home reads or writes the neighbouring slot: the data
	// section places the slots back to back, so the stray byte is silently
	// wrong instead of faulting. Reject it here rather than emit bad code.
	if w := g.spillSlotWidth(symbol); w > 0 && (offset < 0 || bytes < 1 || offset+bytes > w) {
		if g.err == nil {
			g.err = fmt.Errorf("spill %q: %d-byte access at offset %d exceeds its %d-byte slot", symbol, bytes, offset, w)
		}
		return
	}
	if g.nativeMemorySink != nil {
		var reg Reg
		found := false
		for r, loc := range g.ar.Locs {
			if loc.Kind == LocMem && g.spillLabel(r) == symbol {
				reg = r
				found = true
				break
			}
		}
		// Bool canonicalization may request a dedicated generated spill without an
		// allocated LocMem: admit it only when the actual function defines that reg.
		if !found {
			for r := range collectRegInfo(g.fn) {
				if g.spillLabel(r) == symbol {
					reg = r
					found = true
					break
				}
			}
		}
		width := g.spillWidth(reg)
		if !found || offset < 0 || bytes < 1 || offset+bytes > width {
			if g.err == nil {
				g.err = fmt.Errorf("native memory access: unknown or overflowing spill %q", symbol)
			}
			return
		}
		id := fmt.Sprintf("f%d_b%d_s%d", g.nativeFunctionIndex, g.nativeBlockIndex, g.nativeSpillCount)
		g.nativeSpillCount++
		root := GlobalMemoryOrigin{Symbol: symbol, Offset: 0, Width: width, Write: write, Function: g.fn.Name, Block: g.fn.Blocks[g.nativeBlockIndex].Label, Instruction: -1}
		*g.nativeMemorySink = append(*g.nativeMemorySink, NativeMemoryAccess{ID: id, Root: root, ByteOffset: offset, Bytes: bytes, Write: write, StorageKind: "spill"})
		s += " ; @minz-memory-access:" + id
	}
	g.emit(s)
}
func (g *z80cg) emitNativeSpillf(symbol string, offset, bytes int, write bool, format string, args ...any) {
	g.emitNativeSpill(symbol, offset, bytes, write, fmt.Sprintf(format, args...))
}

// Provisional TSMC markers transport known generated spill identity through the
// existing orphan-store rewrite. Actual code patching is never authorized.
func (g *z80cg) emitNativeTSMCSpill(pair *tsmcSpillPair, label string, offset int, format string, args ...any) {
	if g.nativeMemorySink == nil {
		g.emitf(format, args...)
		return
	}
	before := len(*g.nativeMemorySink)
	g.emitNativeSpill(g.spillLabel(pair.reg), offset, 1, true, fmt.Sprintf(format, args...))
	if len(*g.nativeMemorySink) > before {
		r := &(*g.nativeMemorySink)[before]
		r.StorageKind = "orphan-tsmc-spill"
		g.nativeOrphanLabels[r.ID] = label
	}
}
func (g *z80cg) resolveNativeOrphanSpills(assembly string) error {
	for i := range *g.nativeMemorySink {
		r := &(*g.nativeMemorySink)[i]
		if r.StorageKind != "orphan-tsmc-spill" {
			continue
		}
		label := g.nativeOrphanLabels[r.ID]
		if label == "" {
			return fmt.Errorf("native memory access: missing provisional spill identity")
		}
		if strings.Contains(assembly, label+":") {
			return fmt.Errorf("native memory access: executable TSMC spill unsupported")
		}
		text := fmt.Sprintf("LD (%s+%d), A", label, r.ByteOffset+1)
		marker := "@minz-memory-access:" + r.ID
		found := false
		for _, line := range strings.Split(assembly, "\n") {
			if strings.Contains(line, marker) && strings.Contains(line, text) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("native memory access: orphan spill operation changed")
		}
		r.StorageKind = "spill"
	}
	return nil
}
