// Package analysis provides IDA Pro-like static analysis for Z80 binaries.
//
// It builds on the existing disasm package, adding recursive descent
// disassembly, code/data separation, cross-references, label generation,
// string detection, and cycle counting.
package analysis

import (
	"fmt"
	"sort"
)

// ByteClass classifies every byte in the address space.
type ByteClass uint8

const (
	ByteUndefined ByteClass = iota // Not yet analyzed
	ByteCodeStart                  // First byte of an instruction
	ByteCode                       // Interior byte of an instruction
	ByteData                       // Known data
	ByteString                     // Part of a detected string
)

// XRefType classifies a cross-reference.
type XRefType uint8

const (
	XRefCall     XRefType = iota // CALL instruction
	XRefJump                     // Unconditional JP
	XRefCondJump                 // Conditional JP/JR
	XRefRead                     // Data read: LD r,(nn)
	XRefWrite                    // Data write: LD (nn),r
)

// XRef represents a cross-reference between two addresses.
type XRef struct {
	From uint16
	To   uint16
	Type XRefType
}

// Function represents a detected function boundary.
type Function struct {
	Entry uint16
	End   uint16 // Address of last instruction's first byte
	Name  string
	Size  int
}

// DetectedString represents a string found in the binary.
type DetectedString struct {
	Addr       uint16
	Content    string
	Terminator byte // 0x00, 0x0D, 0x24, or 0x80 (bit-7)
	Length     int  // Total bytes including terminator
}

// Label represents a named address.
type Label struct {
	Addr   uint16
	Name   string
	Source string // "auto", "platform", "user"
}

// Analysis holds all state for a binary analysis session.
type Analysis struct {
	Data   []byte
	Origin uint16

	ByteMap [65536]ByteClass

	Functions   map[uint16]*Function
	EntryPoints []uint16

	XRefsTo   map[uint16][]XRef // key=target
	XRefsFrom map[uint16][]XRef // key=source

	Strings map[uint16]*DetectedString
	Labels  map[uint16]*Label

	// User overrides
	CodeOverrides map[uint16]uint16 // start->end forced code
	DataOverrides map[uint16]uint16 // start->end forced data
	Comments      map[uint16]string

	// Statistics
	CodeBytes      int
	DataBytes      int
	StringBytes    int
	UndefinedBytes int

	Platform string
}

// NewAnalysis creates a new analysis context for the given binary.
func NewAnalysis(data []byte, origin uint16) *Analysis {
	return &Analysis{
		Data:          data,
		Origin:        origin,
		Functions:     make(map[uint16]*Function),
		XRefsTo:       make(map[uint16][]XRef),
		XRefsFrom:     make(map[uint16][]XRef),
		Strings:       make(map[uint16]*DetectedString),
		Labels:        make(map[uint16]*Label),
		CodeOverrides: make(map[uint16]uint16),
		DataOverrides: make(map[uint16]uint16),
		Comments:      make(map[uint16]string),
	}
}

// AddEntryPoint adds a manual entry point for analysis.
func (a *Analysis) AddEntryPoint(addr uint16) {
	for _, ep := range a.EntryPoints {
		if ep == addr {
			return
		}
	}
	a.EntryPoints = append(a.EntryPoints, addr)
}

// DetectEntryPoints seeds entry points based on the platform.
func (a *Analysis) DetectEntryPoints(platform string) {
	a.Platform = platform
	endAddr := int(a.Origin) + len(a.Data)

	inRange := func(addr uint16) bool {
		return int(addr) >= int(a.Origin) && int(addr) < endAddr
	}

	switch platform {
	case "cpm":
		a.AddEntryPoint(0x0100)
	case "agon":
		a.AddEntryPoint(a.Origin)
	default: // "generic", "spectrum", ""
		a.AddEntryPoint(a.Origin)
		// RST vectors
		rstAddrs := []uint16{0x00, 0x08, 0x10, 0x18, 0x20, 0x28, 0x30, 0x38}
		for _, addr := range rstAddrs {
			if inRange(addr) {
				a.AddEntryPoint(addr)
			}
		}
		// NMI vector
		if inRange(0x66) {
			a.AddEntryPoint(0x66)
		}
	}
}

// IsCode returns true if the byte at addr is classified as code.
func (a *Analysis) IsCode(addr uint16) bool {
	return a.ByteMap[addr] == ByteCodeStart || a.ByteMap[addr] == ByteCode
}

// GetFunctions returns all detected functions sorted by entry address.
func (a *Analysis) GetFunctions() []*Function {
	fns := make([]*Function, 0, len(a.Functions))
	for _, fn := range a.Functions {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Entry < fns[j].Entry
	})
	return fns
}

// AddXRef records a cross-reference.
func (a *Analysis) AddXRef(from, to uint16, xtype XRefType) {
	xref := XRef{From: from, To: to, Type: xtype}
	a.XRefsTo[to] = append(a.XRefsTo[to], xref)
	a.XRefsFrom[from] = append(a.XRefsFrom[from], xref)
}

// GetXRefsTo returns all references pointing to the given address.
func (a *Analysis) GetXRefsTo(addr uint16) []XRef {
	refs := a.XRefsTo[addr]
	sort.Slice(refs, func(i, j int) bool { return refs[i].From < refs[j].From })
	return refs
}

// GetXRefsFrom returns all references originating from the given address.
func (a *Analysis) GetXRefsFrom(addr uint16) []XRef {
	return a.XRefsFrom[addr]
}

// ComputeStats counts code/data/string/undefined bytes within the binary range.
func (a *Analysis) ComputeStats() {
	a.CodeBytes = 0
	a.DataBytes = 0
	a.StringBytes = 0
	a.UndefinedBytes = 0

	end := int(a.Origin) + len(a.Data)
	for addr := int(a.Origin); addr < end && addr <= 0xFFFF; addr++ {
		switch a.ByteMap[addr] {
		case ByteCodeStart, ByteCode:
			a.CodeBytes++
		case ByteData:
			a.DataBytes++
		case ByteString:
			a.StringBytes++
		default:
			a.UndefinedBytes++
		}
	}
}

// StatsString returns a human-readable summary of analysis statistics.
func (a *Analysis) StatsString() string {
	total := len(a.Data)
	pct := func(n int) float64 {
		if total == 0 {
			return 0
		}
		return float64(n) * 100.0 / float64(total)
	}
	return fmt.Sprintf("Analysis: %d bytes total, %d code (%.1f%%), %d data (%.1f%%), %d string (%.1f%%), %d undefined (%.1f%%), %d functions",
		total, a.CodeBytes, pct(a.CodeBytes),
		a.DataBytes, pct(a.DataBytes),
		a.StringBytes, pct(a.StringBytes),
		a.UndefinedBytes, pct(a.UndefinedBytes),
		len(a.Functions))
}

// InRange returns true if addr is within the loaded binary.
func (a *Analysis) InRange(addr uint16) bool {
	return int(addr) >= int(a.Origin) && int(addr) < int(a.Origin)+len(a.Data)
}

// ReadByte reads a byte from the binary at the given address.
// Returns 0, false if out of range.
func (a *Analysis) ReadByte(addr uint16) (byte, bool) {
	if !a.InRange(addr) {
		return 0, false
	}
	return a.Data[int(addr)-int(a.Origin)], true
}

// ReadBytes reads up to n bytes starting at addr.
func (a *Analysis) ReadBytes(addr uint16, n int) []byte {
	if !a.InRange(addr) {
		return nil
	}
	offset := int(addr) - int(a.Origin)
	end := offset + n
	if end > len(a.Data) {
		end = len(a.Data)
	}
	return a.Data[offset:end]
}
