package vir

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type IndexedLegacyImportEvidence struct {
	QuotientGroups [][]int                        `json:"quotient_groups,omitempty"`
	Schema         string                         `json:"schema"`
	Path           string                         `json:"path"`
	HeaderSHA256   string                         `json:"header_sha256"`
	PrefixSHA256   string                         `json:"prefix_sha256"`
	Count          uint32                         `json:"count"`
	Shape          *EnrichedShape                 `json:"shape"`
	RecordIndex    int                            `json:"record_index"`
	BytesRead      int64                          `json:"bytes_read"`
	Import         *LegacyCandidateImportEvidence `json:"import,omitempty"`
	Reason         string                         `json:"reason"`
}

// ImportIndexedENRT obtains one historical coarse-shape candidate without loading
// the table. Only full canonical ENRT v1 enumeration (4 byte /3 word locsets)
// is accepted. Dense-subset tables require their original mask ordering instead.
// Shape/index matching proposes an assignment; AddChecked proves current ABI/ISA.
func ImportIndexedENRT(path string, p *LocalTableProblem, target *RegAllocTable) (*IndexedLegacyImportEvidence, error) {
	if p == nil || target == nil {
		return nil, fmt.Errorf("indexed import missing problem/target")
	}
	context, err := localFunctionContext(p.Function)
	if err != nil || context != p.Options.CheckedContext {
		return nil, fmt.Errorf("indexed import stale function binding")
	}
	shape, e := indexedLegacyShape(p)
	if e != nil {
		return nil, e
	}
	return importIndexedENRTShape(path, shape, p, target)
}

// indexedLegacyShape is a proposal key: it binds representable ABI constraints
// and exact simultaneous value edges. It does not encode instruction identity,
// immediate values, pattern scratch, costs or flag effects; AddChecked does.
func indexedLegacyShape(p *LocalTableProblem) (*EnrichedShape, error) {
	shape, vregs, e := VIRToEnrichedShape(p.Ops, Z80)
	if e != nil {
		return nil, e
	}
	// Historical graphs describe simultaneous values, not the union of an
	// instruction's consumed input and replacement result. A dead tied input
	// may share the destination; a move may coalesce its equal source.
	indices := map[int]int{}
	for i, v := range vregs {
		indices[v] = i
	}
	var edges [][2]int
	pair := func(a, b int) {
		if a > 0 && b > 0 && a != b {
			edges = append(edges, [2]int{indices[a], indices[b]})
		}
	}
	live := map[int]bool{}
	for i := len(p.Ops) - 1; i >= 0; i-- {
		op := p.Ops[i]
		for v := range live {
			if op.Op != OpMove || v != op.Src[0] {
				pair(op.Dst, v)
			}
		}
		delete(live, op.Dst)
		for _, v := range op.Src {
			if v > 0 {
				live[v] = true
			}
		}
		for a := range live {
			for b := range live {
				pair(a, b)
			}
		}
	}
	shape.Interference = InterferenceBitmask(len(vregs), edges)
	for v, loc := range p.Options.ParamLocs {
		i, ok := indices[v]
		if !ok {
			return nil, fmt.Errorf("unbound ABI parameter")
		}
		if loc < 0 || loc >= len(Z80.Locs) {
			return nil, fmt.Errorf("invalid ABI location")
		}
		name := Z80.Locs[loc].Name
		switch name {
		case "A", "HL":
			if (name == "A") != (shape.Widths[i] == 8) {
				return nil, fmt.Errorf("ABI width mismatch")
			}
			shape.LocSetIndex[i] = 0
		case "C", "DE":
			if (name == "C") != (shape.Widths[i] == 8) {
				return nil, fmt.Errorf("ABI width mismatch")
			}
			shape.LocSetIndex[i] = 1
		default:
			return nil, fmt.Errorf("exact ABI location %s unavailable in canonical historical locsets", name)
		}
	}
	return shape, nil
}

func importIndexedENRTShape(path string, shape *EnrichedShape, p *LocalTableProblem, target *RegAllocTable) (*IndexedLegacyImportEvidence, error) {
	return importIndexedENRTMapped(path, shape, p, target, nil)
}
func importIndexedENRTMapped(path string, shape *EnrichedShape, p *LocalTableProblem, target *RegAllocTable, groups [][]int) (*IndexedLegacyImportEvidence, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	hash := sha256.New()
	limited := &io.LimitedReader{R: f, N: 256 << 20}
	reader := io.TeeReader(limited, hash)
	header := make([]byte, 16)
	if _, e = io.ReadFull(reader, header); e != nil {
		return nil, e
	}
	if string(header[:4]) != "ENRT" || binary.LittleEndian.Uint32(header[4:8]) != 1 {
		return nil, fmt.Errorf("indexed import requires ENRT v1")
	}
	count := binary.LittleEndian.Uint32(header[8:12])
	maxv, metrics := int(header[12]), int(header[13])
	if maxv < 2 || maxv > 6 || metrics != 12 || header[14] != 0 || header[15] != 0 {
		return nil, fmt.Errorf("invalid ENRT header")
	}
	canonical := 0
	for n := 2; n <= maxv; n++ {
		canonical += countShapesWithLocSets(n, 4, 3)
	}
	if uint64(count) != uint64(canonical) {
		return nil, fmt.Errorf("noncanonical/dense ENRT enumeration requires explicit original mask mapping")
	}
	idx, e := EnrichedIndexOfWithLocSets(*shape, maxv, 4, 3)
	if e != nil {
		return nil, e
	}
	ev := &IndexedLegacyImportEvidence{Schema: "vir-indexed-enrt-import/v1", Path: path, HeaderSHA256: fmt.Sprintf("%x", sha256.Sum256(header)), Count: count, Shape: shape, RecordIndex: idx}
	var assignment []int
	var cost int
	buf := make([]byte, 128)
	for i := 0; i <= idx; i++ {
		if _, e = io.ReadFull(reader, buf[:1]); e != nil {
			return nil, fmt.Errorf("indexed record bounded/truncated: %w", e)
		}
		n := int(buf[0])
		if n == 255 {
			if i == idx {
				ev.Reason = "historical record infeasible"
			}
			continue
		}
		expectedNV, boundary := 2, countShapesWithLocSets(2, 4, 3)
		for i >= boundary && expectedNV < maxv {
			expectedNV++
			boundary += countShapesWithLocSets(expectedNV, 4, 3)
		}
		if n != expectedNV {
			return nil, fmt.Errorf("invalid ENRT record width")
		}
		length := 2 + n + 2 + metrics*2
		if _, e = io.ReadFull(reader, buf[:length]); e != nil {
			return nil, e
		}
		if i == idx {
			if n != shape.NVregs {
				return nil, fmt.Errorf("enum record vreg count mismatch")
			}
			cost = int(binary.LittleEndian.Uint16(buf[:2]))
			for _, v := range buf[2 : 2+n] {
				assignment = append(assignment, int(v))
			}
		}
	}
	ev.BytesRead = (256 << 20) - limited.N
	ev.PrefixSHA256 = fmt.Sprintf("%x", hash.Sum(nil))
	if assignment == nil {
		return ev, nil
	}
	if groups != nil {
		ev.Schema = "vir-indexed-enrt-quotient-import/v1"
		ev.QuotientGroups = groups
		dense := map[int]int{}
		for i, group := range groups {
			loc := assignment[i]
			if loc < 0 || loc >= len(gpuToZ80Loc) || gpuToZ80Loc[loc] < 0 {
				return ev, fmt.Errorf("unmappable quotient location")
			}
			for _, v := range group {
				dense[v] = gpuToZ80Loc[loc]
			}
		}
		ids := []int{}
		for v := range dense {
			ids = append(ids, v)
		}
		sort.Ints(ids)
		lifted := []int{}
		for _, v := range ids {
			lifted = append(lifted, dense[v])
		}
		data, _ := json.Marshal([]RegAllocEntry{{Signature: fmt.Sprintf("ENRT-quotient-record-%d", idx), Cost: cost, Assignment: lifted}})
		ev.Import, e = ImportLegacyCandidates(data, "legacy-native-json", p.Ops, Z80, p.Options, target)
		if e != nil {
			return nil, e
		}
		ev.Reason = "lifted historical quotient rejected by current admission"
		if ev.Import.Admitted > 0 {
			ev.Reason = "lifted historical quotient independently admitted"
		}
		return ev, nil
	}
	data, _ := json.Marshal(map[string]RegAllocEntry{fmt.Sprintf("ENRT-record-%d", idx): {Cost: cost, Assignment: assignment}})
	ev.Import, e = ImportLegacyCandidates(data, "legacy-gpu-json", p.Ops, Z80, p.Options, target)
	if e != nil {
		return nil, e
	}
	if ev.Import.Admitted > 0 {
		ev.Reason = "current problem independently admitted"
	} else {
		ev.Reason = "historical assignment rejected by current admission"
	}
	return ev, nil
}
