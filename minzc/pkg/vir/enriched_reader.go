// enriched_reader.go — binary format reader for enriched regalloc tables.
//
// Supports two formats from github.com/oisee/z80-optimizer:
//
// ENRT v1 (enriched table, from pipeline/enrich step):
//
//	Header: "ENRT"(4) + version=1(u32le) + count(u32le) + maxVregs(u8) + nMetrics(u8) + reserved(u16) = 16B
//	Records: 0xFF=infeasible | nVregs(u8) + cost(u16le) + assignment[nVregs] + flags(u16le) + nMetrics×u16le
//
// Z80T v2 (ix-expanded table, from build-ix-table cmd):
//
//	Header: "Z80T"(4) + version=2(u32le) + nLocSets8(u8) + nLocSets16(u8) + maxVregs(u8) + n_entries(u64le) = 19B
//	Records: 0xFF=infeasible | nVregs(u8) + cost(u16le) + assignment[nVregs]
//	         (no flags or metrics — leaner format, includes IX-half locs)
//
// Both formats use the same enumeration index (EnrichedIndexOf) for O(1) lookup.
// Z80T v2 "ix_expanded" tables include IXH/IXL as allocation targets (+12% coverage).
package vir

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// EnrichedEntry is a single register allocation result from ENRT binary table.
type EnrichedEntry struct {
	Cost         int      // T-states cost, -1 if infeasible
	Assignment   []byte   // physical location per vreg (nil if infeasible)
	Flags        uint16   // feasibility flags (no_accumulator, mul8_safe, etc.)
	PatternCosts []uint16 // per-pattern-category costs (12 metrics)
}

// Infeasible returns true if no valid assignment exists for this shape.
func (e *EnrichedEntry) Infeasible() bool { return e.Cost < 0 }

// EnrichedBinaryTable holds all entries from a Z80T binary file, indexed by enumeration order.
type EnrichedBinaryTable struct {
	Entries []EnrichedEntry
}

// LoadEnrichedBinary reads an ENRT binary table.
// Header: 4B magic "ENRT" + 4B version(LE) + 4B count(LE) + 1B maxVregs + 1B nMetrics + 2B reserved = 16 bytes.
// Per-entry: 0xFF=infeasible, or nVregs(1) + cost(2 LE) + assignment[nVregs] + flags(2 LE) + nMetrics×cost(2 LE each).
func LoadEnrichedBinary(path string) (*EnrichedBinaryTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read 16-byte header
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	isENRT := string(magic[:]) == "ENRT"
	isZ80T := string(magic[:]) == "Z80T"
	if !isENRT && !isZ80T {
		return nil, fmt.Errorf("bad magic: %q (expected ENRT or Z80T)", magic)
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	// Z80T v2: remaining header = nLocSets8(u8) + nLocSets16(u8) + maxVregs(u8) + n_entries(u64le).
	// Records: 0xFF=infeasible | nVregs(u8) + cost(u16le) + assignment[nVregs].
	// Simpler than ENRT — no flags or metrics. Includes IX-half locs (+12% coverage).
	if isZ80T && version == 2 {
		var nLocSets8, nLocSets16, maxVregs uint8
		binary.Read(f, binary.LittleEndian, &nLocSets8)
		binary.Read(f, binary.LittleEndian, &nLocSets16)
		binary.Read(f, binary.LittleEndian, &maxVregs)
		var nEntries uint64
		if err := binary.Read(f, binary.LittleEndian, &nEntries); err != nil {
			return nil, fmt.Errorf("Z80T v2 read n_entries: %w", err)
		}
		// Read remaining file data in one shot — avoids 60M individual ReadFull calls.
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("Z80T v2 read body: %w", err)
		}
		// Parse records from in-memory slice. Arena for assignments avoids per-entry allocs.
		mv := int(maxVregs)
		assignArena := make([]byte, 0, int(nEntries)*mv*4/5) // ~80% feasible, avg nv≈mv
		entries := make([]EnrichedEntry, 0, nEntries)
		pos := 0
		for pos < len(data) {
			marker := data[pos]
			pos++
			if marker == 0xFF {
				entries = append(entries, EnrichedEntry{Cost: -1})
			} else {
				nv := int(marker)
				if pos+2+nv > len(data) {
					return nil, fmt.Errorf("Z80T v2 truncated at record %d", len(entries))
				}
				cost := int(data[pos]) | int(data[pos+1])<<8
				pos += 2
				base := len(assignArena)
				assignArena = append(assignArena, data[pos:pos+nv]...)
				pos += nv
				entries = append(entries, EnrichedEntry{Cost: cost, Assignment: assignArena[base : base+nv]})
			}
		}
		return &EnrichedBinaryTable{Entries: entries}, nil
	}

	if !isENRT && !(isZ80T && version == 1) {
		return nil, fmt.Errorf("unsupported format: magic=%q version=%d", magic, version)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported ENRT version: %d (loader supports v1)", version)
	}

	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, fmt.Errorf("read count: %w", err)
	}

	var maxVregs, nMetrics uint8
	binary.Read(f, binary.LittleEndian, &maxVregs)
	binary.Read(f, binary.LittleEndian, &nMetrics)
	var reserved uint16
	binary.Read(f, binary.LittleEndian, &reserved)

	// Read all records
	entries := make([]EnrichedEntry, 0, count)
	buf := make([]byte, 1)

	for {
		_, err := io.ReadFull(f, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read record %d: %w", len(entries), err)
		}

		marker := buf[0]
		if marker == 0xFF {
			// Infeasible
			entries = append(entries, EnrichedEntry{Cost: -1})
		} else {
			// Feasible: marker is nVregs
			nv := int(marker)
			var cost uint16
			if err := binary.Read(f, binary.LittleEndian, &cost); err != nil {
				return nil, fmt.Errorf("read cost at record %d: %w", len(entries), err)
			}
			assign := make([]byte, nv)
			if _, err := io.ReadFull(f, assign); err != nil {
				return nil, fmt.Errorf("read assignment at record %d: %w", len(entries), err)
			}
			// Read flags (2 bytes)
			var flags uint16
			binary.Read(f, binary.LittleEndian, &flags)
			// Read pattern costs (nMetrics × 2 bytes each)
			patternCosts := make([]uint16, nMetrics)
			for i := range patternCosts {
				binary.Read(f, binary.LittleEndian, &patternCosts[i])
			}
			entries = append(entries, EnrichedEntry{
				Cost: int(cost), Assignment: assign,
				Flags: flags, PatternCosts: patternCosts,
			})
		}
	}

	return &EnrichedBinaryTable{Entries: entries}, nil
}

// Lookup returns the entry at the given enumeration index, or nil if out of range.
func (t *EnrichedBinaryTable) Lookup(index int) *EnrichedEntry {
	if index < 0 || index >= len(t.Entries) {
		return nil
	}
	return &t.Entries[index]
}

// Stats returns table statistics.
func (t *EnrichedBinaryTable) Stats() (total, feasible, infeasible int) {
	total = len(t.Entries)
	for i := range t.Entries {
		if t.Entries[i].Infeasible() {
			infeasible++
		} else {
			feasible++
		}
	}
	return
}

// Location name constants matching the binary format.
var enrichedLocNames = [15]string{
	"A", "B", "C", "D", "E", "H", "L",
	"BC", "DE", "HL",
	"IXH", "IXL", "IYH", "IYL",
	"mem0",
}

// FormatAssignment returns a human-readable assignment string like "A=HL, B=DE".
func FormatEnrichedAssignment(assignment []byte) string {
	s := ""
	for i, loc := range assignment {
		if i > 0 {
			s += ", "
		}
		name := "?"
		if int(loc) < len(enrichedLocNames) {
			name = enrichedLocNames[loc]
		}
		s += fmt.Sprintf("v%d=%s", i, name)
	}
	return s
}
