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

// EnrichedEntry is a single register allocation result from Z80T binary table.
type EnrichedEntry struct {
	Cost       int    // T-states cost, -1 if infeasible
	Assignment []byte // physical location per vreg (nil if infeasible)
}

// Infeasible returns true if no valid assignment exists for this shape.
func (e *EnrichedEntry) Infeasible() bool { return e.Cost < 0 }

// EnrichedBinaryTable holds all entries from a Z80T binary file, indexed by enumeration order.
type EnrichedBinaryTable struct {
	Entries []EnrichedEntry
}

// LoadBinary reads a Z80T binary table (optionally zstd-compressed).
// Returns a Table with entries indexed by enumeration order.
func LoadEnrichedBinary(path string) (*EnrichedBinaryTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read header
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != "Z80T" {
		return nil, fmt.Errorf("bad magic: %q (expected Z80T)", magic)
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read all records
	var entries []EnrichedEntry
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
			entries = append(entries, EnrichedEntry{Cost: int(cost), Assignment: assign})
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
