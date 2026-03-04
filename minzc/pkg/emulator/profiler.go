package emulator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// Profiler collects execution, memory, and I/O heatmaps during emulation.
// It optionally writes a basic-block trace to a JSONL file.
//
// Ported from pkg/spectrum/profiler.go with simplifications:
// no frame gating (MZE runs programs to completion, not real-time).
//
// Usage:
//
//	p := NewProfiler()
//	p.SetTraceOutput("trace.jsonl")  // optional
//	z80.Profiler = p
//	// ... run ...
//	p.ExportProfile("profile.mzprof.json")
//	p.Close()
type Profiler struct {
	// Memory access heatmaps — one counter per 16-bit address.
	ExecCount  [65536]uint32 // Instruction execution at each PC
	ReadCount  [65536]uint32 // Memory reads
	WriteCount [65536]uint32 // Memory writes

	// Stack access heatmaps — tracks PUSH/POP by SP-delta detection.
	StackPush [65536]uint32 // SP decremented → write to this address
	StackPop  [65536]uint32 // SP incremented → read from this address

	// I/O heatmaps — sparse, keyed by port address.
	IORead  map[uint16]uint32
	IOWrite map[uint16]uint32

	// Basic-block trace (JSONL output).
	traceFile *os.File
	traceBuf  *bufio.Writer

	// Basic block tracking state.
	blockStartPC uint16 // PC where current basic block began
	blockStartT  int64  // T-state at block start
	prevPC       uint16 // PC of last executed instruction
	blockInstrs  int    // instructions in current block
	inBlock      bool   // whether we're inside a block

	// SP tracking for stack heatmap.
	prevSP    uint16
	spTracked bool // false until first BeforeOpcode sets prevSP

	// Memory snapshot — captured at export time via SetMemorySnapshot().
	memSnapshot []byte // nil until snapshot is taken

	// Stats.
	TotalInstrs uint64
	StackDepth  uint16 // high-water mark: lowest SP seen
}

// NewProfiler creates a profiler that collects heatmaps.
func NewProfiler() *Profiler {
	return &Profiler{
		IORead:  make(map[uint16]uint32),
		IOWrite: make(map[uint16]uint32),
	}
}

// SetTraceOutput opens a JSONL file for basic-block trace output.
func (p *Profiler) SetTraceOutput(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating trace file: %w", err)
	}
	p.traceFile = f
	p.traceBuf = bufio.NewWriterSize(f, 256*1024) // 256KB buffer
	return nil
}

// BeforeOpcode is called before each CPU instruction.
// Records execution count, detects stack operations via SP delta, and tracks basic blocks.
func (p *Profiler) BeforeOpcode(pc uint16, tstates int64) {
	p.ExecCount[pc]++
	p.TotalInstrs++

	// Basic block tracking for trace
	if p.traceBuf == nil {
		return
	}

	if !p.inBlock {
		// Start a new basic block
		p.blockStartPC = pc
		p.blockStartT = tstates
		p.prevPC = pc
		p.blockInstrs = 1
		p.inBlock = true
		return
	}

	// Check if PC is sequential with previous instruction.
	// Z80 instructions are 1-4 bytes, so if the new PC is within
	// prevPC+1..prevPC+4, it's sequential (same basic block).
	diff := int(pc) - int(p.prevPC)
	if diff >= 1 && diff <= 4 {
		// Sequential — extend the block
		p.prevPC = pc
		p.blockInstrs++
		return
	}

	// Non-sequential: basic block boundary (jump/call/ret/interrupt)
	p.emitBlock(tstates)
	p.emitJump(p.prevPC, pc, tstates)

	// Start new block
	p.blockStartPC = pc
	p.blockStartT = tstates
	p.prevPC = pc
	p.blockInstrs = 1
}

// TrackSP is called after each instruction with the current SP.
// Detects pushes (SP decreased) and pops (SP increased) by comparing
// with the previous SP value. Uses signed delta to handle uint16 wraparound.
// Only tracks deltas of ±6 bytes (3 words) — larger jumps are LD SP,nn
// re-initialization and are ignored.
func (p *Profiler) TrackSP(sp uint16) {
	if !p.spTracked {
		p.prevSP = sp
		p.StackDepth = sp
		p.spTracked = true
		return
	}
	delta := int16(sp - p.prevSP)
	if delta < 0 && delta >= -6 {
		// SP decreased → push (PUSH, CALL, RST, INT)
		for i := int16(0); i < -delta; i++ {
			p.StackPush[sp+uint16(i)]++
		}
	} else if delta > 0 && delta <= 6 {
		// SP increased → pop (POP, RET)
		for i := int16(0); i < delta; i++ {
			p.StackPop[p.prevSP+uint16(i)]++
		}
	}
	// Track high-water mark (lowest SP = deepest stack usage)
	if p.StackDepth == 0 || sp < p.StackDepth {
		// Only update if it looks like a real stack address (not initial 0)
		if sp != 0 {
			p.StackDepth = sp
		}
	}
	p.prevSP = sp
}

// SetMemorySnapshot captures a copy of the full 64KB memory for inclusion
// in the profile export. Call this just before ExportProfile() to see
// what's at the hot addresses.
func (p *Profiler) SetMemorySnapshot(mem []byte) {
	p.memSnapshot = make([]byte, len(mem))
	copy(p.memSnapshot, mem)
}

// OnMemRead increments the read heatmap.
func (p *Profiler) OnMemRead(addr uint16) {
	p.ReadCount[addr]++
}

// OnMemWrite increments the write heatmap.
func (p *Profiler) OnMemWrite(addr uint16) {
	p.WriteCount[addr]++
}

// OnIORead increments the I/O read heatmap.
func (p *Profiler) OnIORead(port uint16) {
	p.IORead[port]++
}

// OnIOWrite increments the I/O write heatmap.
func (p *Profiler) OnIOWrite(port uint16) {
	p.IOWrite[port]++
}

// emitBlock writes the current basic block to the trace.
func (p *Profiler) emitBlock(endT int64) {
	if p.traceBuf == nil || p.blockInstrs == 0 {
		return
	}
	fmt.Fprintf(p.traceBuf, `{"t":%d,"e":"bb","s":"%04X","end":"%04X","n":%d}`+"\n",
		p.blockStartT, p.blockStartPC, p.prevPC, p.blockInstrs)
}

// emitJump writes a non-linear PC transition to the trace.
func (p *Profiler) emitJump(fromPC, toPC uint16, tstates int64) {
	if p.traceBuf == nil {
		return
	}
	fmt.Fprintf(p.traceBuf, `{"t":%d,"e":"jp","s":"%04X","d":"%04X"}`+"\n",
		tstates, fromPC, toPC)
}

// Flush writes any buffered trace data to disk.
func (p *Profiler) Flush() {
	if p.traceBuf != nil {
		// Emit the last basic block
		if p.inBlock && p.blockInstrs > 0 {
			p.emitBlock(0)
			p.inBlock = false
		}
		p.traceBuf.Flush()
	}
}

// Close flushes and closes the trace file.
func (p *Profiler) Close() {
	p.Flush()
	if p.traceFile != nil {
		p.traceFile.Close()
		p.traceFile = nil
		p.traceBuf = nil
	}
}

// ExportProfile writes the heatmap data to a JSON file.
// If SetMemorySnapshot() was called, hot addresses include their byte values.
func (p *Profiler) ExportProfile(path string) error {
	type profileData struct {
		Meta      map[string]interface{} `json:"meta"`
		Exec      map[string]uint32      `json:"exec,omitempty"`
		Read      map[string]uint32      `json:"read,omitempty"`
		Write     map[string]uint32      `json:"write,omitempty"`
		StackPush map[string]uint32      `json:"stack_push,omitempty"`
		StackPop  map[string]uint32      `json:"stack_pop,omitempty"`
		IORead    map[string]uint32      `json:"io_read,omitempty"`
		IOWrite   map[string]uint32      `json:"io_write,omitempty"`
		// Memory snapshot at hot addresses: "ADDR": "XX" (hex byte value)
		MemSnapshot map[string]string `json:"mem_snapshot,omitempty"`
	}

	meta := map[string]interface{}{
		"total_instrs": p.TotalInstrs,
	}
	if p.spTracked {
		meta["stack_depth"] = fmt.Sprintf("%04X", p.StackDepth)
	}

	data := profileData{
		Meta:      meta,
		Exec:      sparseMap(p.ExecCount[:]),
		Read:      sparseMap(p.ReadCount[:]),
		Write:     sparseMap(p.WriteCount[:]),
		StackPush: sparseMap(p.StackPush[:]),
		StackPop:  sparseMap(p.StackPop[:]),
		IORead:    portMap(p.IORead),
		IOWrite:   portMap(p.IOWrite),
	}

	// Include memory values at all addresses that had any activity
	if p.memSnapshot != nil {
		snap := make(map[string]string)
		for i := 0; i < 65536; i++ {
			if p.ExecCount[i] > 0 || p.ReadCount[i] > 0 || p.WriteCount[i] > 0 ||
				p.StackPush[i] > 0 || p.StackPop[i] > 0 {
				snap[fmt.Sprintf("%04X", i)] = fmt.Sprintf("%02X", p.memSnapshot[i])
			}
		}
		if len(snap) > 0 {
			data.MemSnapshot = snap
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating profile: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// sparseMap converts a 64K counter array to a map with only non-zero entries.
// Keys are uppercase hex addresses.
func sparseMap(arr []uint32) map[string]uint32 {
	m := make(map[string]uint32)
	for i, v := range arr {
		if v > 0 {
			m[fmt.Sprintf("%04X", i)] = v
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// portMap converts a port counter map to string-keyed map for JSON.
func portMap(src map[uint16]uint32) map[string]uint32 {
	if len(src) == 0 {
		return nil
	}
	m := make(map[string]uint32, len(src))
	for k, v := range src {
		m[fmt.Sprintf("%04X", k)] = v
	}
	return m
}
