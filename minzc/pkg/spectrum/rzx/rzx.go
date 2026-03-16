// Package rzx implements reading and writing of RZX (input recording) files
// for ZX Spectrum emulators.
//
// RZX records port IN values frame-by-frame, enabling deterministic replay
// of gameplay when combined with a snapshot of initial machine state.
//
// Spec: https://worldofspectrum.net/RZXformat.html (v0.13, 2005-03-02)
//
// Usage:
//
//	rec, err := rzx.ReadFile("game.rzx")
//	for _, frame := range rec.Frames {
//	    // feed frame.INValues to emulator port reads
//	    // run frame.FetchCount instruction fetches
//	}
package rzx

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ── File structure ──────────────────────────────────────────────────────────

// Recording is a parsed RZX file.
type Recording struct {
	MajorVersion byte
	MinorVersion byte
	Flags        uint32

	Creator     string // creator program name (from Creator block)
	CreatorMaj  uint16
	CreatorMin  uint16

	Snapshot    []byte // embedded snapshot data (SNA/Z80), nil if external
	SnapExt     string // snapshot extension ("sna", "z80")
	SnapFile    string // external snapshot filename (if not embedded)

	Frames      []Frame
	InitTStates uint32 // initial T-state counter value
}

// Frame is one interrupt frame of recorded input.
type Frame struct {
	FetchCount uint16 // R register increments until next interrupt
	INCount    uint16 // number of port reads (0xFFFF = repeat previous)
	INValues   []byte // byte values returned for each IN
}

// IsRepeat returns true if this frame repeats the previous frame's input.
func (f Frame) IsRepeat() bool { return f.INCount == 0xFFFF }

// ── Block IDs ───────────────────────────────────────────────────────────────

const (
	blockCreator        = 0x10
	blockSecurity       = 0x20
	blockSecuritySig    = 0x21
	blockSnapshot       = 0x30
	blockInputRecording = 0x80

	magic = "RZX!"
)

// ── Reader ──────────────────────────────────────────────────────────────────

// ReadFile reads an RZX file from disk.
func ReadFile(path string) (*Recording, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Read(data)
}

// Read parses RZX data from a byte slice.
func Read(data []byte) (*Recording, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("rzx: file too short (%d bytes)", len(data))
	}

	// Header: 4-byte magic + 1 major + 1 minor + 4 flags
	if string(data[0:4]) != magic {
		return nil, fmt.Errorf("rzx: bad magic %q (want %q)", data[0:4], magic)
	}

	rec := &Recording{
		MajorVersion: data[4],
		MinorVersion: data[5],
		Flags:        binary.LittleEndian.Uint32(data[6:10]),
	}

	pos := 10
	for pos < len(data) {
		if pos+5 > len(data) {
			break // not enough for block header
		}
		blockID := data[pos]
		blockLen := binary.LittleEndian.Uint32(data[pos+1 : pos+5])
		if blockLen < 5 {
			return nil, fmt.Errorf("rzx: block at offset %d has invalid length %d", pos, blockLen)
		}
		blockEnd := pos + int(blockLen)
		if blockEnd > len(data) {
			return nil, fmt.Errorf("rzx: block at offset %d overflows file (len %d, file %d)", pos, blockLen, len(data))
		}
		blockData := data[pos+5 : blockEnd]

		switch blockID {
		case blockCreator:
			if err := rec.parseCreator(blockData); err != nil {
				return nil, err
			}
		case blockSnapshot:
			if err := rec.parseSnapshot(blockData); err != nil {
				return nil, err
			}
		case blockInputRecording:
			if err := rec.parseInputRecording(blockData); err != nil {
				return nil, err
			}
		case blockSecurity, blockSecuritySig:
			// Skip security blocks (competition mode signatures)
		default:
			// Unknown block — skip
		}

		pos = blockEnd
	}

	return rec, nil
}

// ── Block parsers ───────────────────────────────────────────────────────────

func (r *Recording) parseCreator(data []byte) error {
	if len(data) < 24 {
		return fmt.Errorf("rzx: creator block too short (%d bytes)", len(data))
	}
	// 20 bytes ASCIIZ creator + 2 major + 2 minor
	name := data[:20]
	if idx := bytes.IndexByte(name, 0); idx >= 0 {
		name = name[:idx]
	}
	r.Creator = string(name)
	r.CreatorMaj = binary.LittleEndian.Uint16(data[20:22])
	r.CreatorMin = binary.LittleEndian.Uint16(data[22:24])
	return nil
}

func (r *Recording) parseSnapshot(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("rzx: snapshot block too short (%d bytes)", len(data))
	}
	flags := binary.LittleEndian.Uint32(data[0:4])
	ext := string(bytes.TrimRight(data[4:8], "\x00"))
	uncompLen := binary.LittleEndian.Uint32(data[8:12])
	snapData := data[12:]

	r.SnapExt = ext

	if flags&1 != 0 {
		// External snapshot: filename follows
		if idx := bytes.IndexByte(snapData, 0); idx >= 0 {
			r.SnapFile = string(snapData[:idx])
		}
		return nil
	}

	// Embedded snapshot — possibly zlib compressed
	if flags&2 != 0 && len(snapData) > 0 {
		zr, err := zlib.NewReader(bytes.NewReader(snapData))
		if err != nil {
			return fmt.Errorf("rzx: snapshot zlib: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return fmt.Errorf("rzx: snapshot decompress: %w", err)
		}
		r.Snapshot = decompressed
	} else {
		r.Snapshot = make([]byte, uncompLen)
		copy(r.Snapshot, snapData)
	}

	return nil
}

func (r *Recording) parseInputRecording(data []byte) error {
	if len(data) < 13 {
		return fmt.Errorf("rzx: input recording block too short (%d bytes)", len(data))
	}

	numFrames := binary.LittleEndian.Uint32(data[0:4])
	// data[4] reserved
	r.InitTStates = binary.LittleEndian.Uint32(data[5:9])
	flags := binary.LittleEndian.Uint32(data[9:13])
	frameData := data[13:]

	// Decompress if compressed (bit 1)
	if flags&2 != 0 && len(frameData) > 0 {
		zr, err := zlib.NewReader(bytes.NewReader(frameData))
		if err != nil {
			return fmt.Errorf("rzx: input zlib: %w", err)
		}
		defer zr.Close()
		decompressed, err := io.ReadAll(zr)
		if err != nil {
			return fmt.Errorf("rzx: input decompress: %w", err)
		}
		frameData = decompressed
	}

	// Parse frames
	pos := 0
	for i := uint32(0); i < numFrames && pos+4 <= len(frameData); i++ {
		fetchCount := binary.LittleEndian.Uint16(frameData[pos : pos+2])
		inCount := binary.LittleEndian.Uint16(frameData[pos+2 : pos+4])
		pos += 4

		var inValues []byte
		if inCount != 0xFFFF && inCount > 0 {
			end := pos + int(inCount)
			if end > len(frameData) {
				return fmt.Errorf("rzx: frame %d IN values overflow (need %d, have %d)", i, inCount, len(frameData)-pos)
			}
			inValues = make([]byte, inCount)
			copy(inValues, frameData[pos:end])
			pos = end
		}

		r.Frames = append(r.Frames, Frame{
			FetchCount: fetchCount,
			INCount:    inCount,
			INValues:   inValues,
		})
	}

	return nil
}

// ── Writer ──────────────────────────────────────────────────────────────────

// Write serializes a Recording to RZX format.
func (r *Recording) Write() ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString(magic)
	buf.WriteByte(r.MajorVersion)
	buf.WriteByte(r.MinorVersion)
	binary.Write(&buf, binary.LittleEndian, r.Flags)

	// Creator block
	if r.Creator != "" {
		writeCreatorBlock(&buf, r.Creator, r.CreatorMaj, r.CreatorMin)
	}

	// Snapshot block (if embedded)
	if r.Snapshot != nil {
		writeSnapshotBlock(&buf, r.Snapshot, r.SnapExt)
	}

	// Input recording block
	if len(r.Frames) > 0 {
		writeInputBlock(&buf, r.Frames, r.InitTStates)
	}

	return buf.Bytes(), nil
}

// WriteFile writes a Recording to an RZX file.
func (r *Recording) WriteFile(path string) error {
	data, err := r.Write()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func writeCreatorBlock(w *bytes.Buffer, name string, maj, min uint16) {
	var block bytes.Buffer
	// 20-byte ASCIIZ name
	nameBytes := make([]byte, 20)
	copy(nameBytes, name)
	block.Write(nameBytes)
	binary.Write(&block, binary.LittleEndian, maj)
	binary.Write(&block, binary.LittleEndian, min)

	// Block header: ID + length (including header)
	w.WriteByte(blockCreator)
	binary.Write(w, binary.LittleEndian, uint32(5+block.Len()))
	w.Write(block.Bytes())
}

func writeSnapshotBlock(w *bytes.Buffer, snap []byte, ext string) {
	var block bytes.Buffer
	binary.Write(&block, binary.LittleEndian, uint32(2)) // flags: compressed
	extBytes := make([]byte, 4)
	copy(extBytes, ext)
	block.Write(extBytes)
	binary.Write(&block, binary.LittleEndian, uint32(len(snap))) // uncompressed length

	// Compress snapshot
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(snap)
	zw.Close()
	block.Write(zbuf.Bytes())

	w.WriteByte(blockSnapshot)
	binary.Write(w, binary.LittleEndian, uint32(5+block.Len()))
	w.Write(block.Bytes())
}

func writeInputBlock(w *bytes.Buffer, frames []Frame, initTStates uint32) {
	// Serialize frame data
	var frameData bytes.Buffer
	for _, f := range frames {
		binary.Write(&frameData, binary.LittleEndian, f.FetchCount)
		binary.Write(&frameData, binary.LittleEndian, f.INCount)
		if f.INCount != 0xFFFF {
			frameData.Write(f.INValues)
		}
	}

	// Compress frame data
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(frameData.Bytes())
	zw.Close()

	var block bytes.Buffer
	binary.Write(&block, binary.LittleEndian, uint32(len(frames)))
	block.WriteByte(0) // reserved
	binary.Write(&block, binary.LittleEndian, initTStates)
	binary.Write(&block, binary.LittleEndian, uint32(2)) // flags: compressed
	block.Write(zbuf.Bytes())

	w.WriteByte(blockInputRecording)
	binary.Write(w, binary.LittleEndian, uint32(5+block.Len()))
	w.Write(block.Bytes())
}
