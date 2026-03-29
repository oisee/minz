// enriched_reader.go — Z80T binary format reader for enriched regalloc tables.
//
// Vendored from github.com/oisee/z80-optimizer/pkg/regalloc/table.go
//
// Tables are indexed by enumeration order — shape N in the file corresponds to
// shape N from regalloc-enum. Callers must compute the enumeration index from
// their constraint shape to perform a lookup.
//
// Binary format (Z80T v1):
//
//	Header:  4 bytes magic "Z80T" + 4 bytes version (uint32 LE)
//	Records: variable-length, one per shape:
//	  Infeasible: 0xFF (1 byte)
//	  Feasible:   nVregs (uint8) + cost (uint16 LE) + assignment (nVregs bytes)
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
	if string(magic[:]) != "ENRT" {
		return nil, fmt.Errorf("bad magic: %q (expected ENRT)", magic)
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
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
