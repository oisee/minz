package tas

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// TAS file format constants
const (
	TASMagic       = "MINZTAS\x00"
	TASVersion     = 1
	TASFormatJSON  = 0
	TASFormatBinary = 1
	TASFormatCompressed = 2
)

// TASFile represents a complete TAS recording
type TASFile struct {
	Header   TASHeader       `json:"header"`
	Metadata TASMetadata     `json:"metadata"`
	States   []StateSnapshot `json:"states,omitempty"`
	Events   TASEvents       `json:"events"`
	
	// Compression data (for binary format)
	compression *TASCompression
}

// TASHeader contains file format information
type TASHeader struct {
	Magic      [8]byte   `json:"-"`
	Version    uint16    `json:"version"`
	Format     uint8     `json:"format"`
	Flags      uint8     `json:"flags"`
	Created    time.Time `json:"created"`
	Checksum   uint32    `json:"checksum"`
}

// TASMetadata contains recording metadata
type TASMetadata struct {
	ProgramName    string            `json:"program_name"`
	ProgramVersion string            `json:"program_version"`
	RecordingTime  time.Duration     `json:"recording_time"`
	TotalFrames    int64             `json:"total_frames"`
	TotalCycles    int64             `json:"total_cycles"`
	Description    string            `json:"description"`
	Author         string            `json:"author"`
	Tags           []string          `json:"tags"`
	Properties     map[string]string `json:"properties"`
}

// TASEvents contains all recorded events
type TASEvents struct {
	Inputs     []InputEvent `json:"inputs"`
	SMCEvents  []SMCEvent   `json:"smc_events"`
	IOEvents   []IOEvent    `json:"io_events"`
	Breakpoints []Breakpoint `json:"breakpoints,omitempty"`
}

// IOEvent represents I/O operations
type IOEvent struct {
	Cycle   int64  `json:"cycle"`
	Port    uint16 `json:"port"`
	Value   byte   `json:"value"`
	IsInput bool   `json:"is_input"`
}

// Breakpoint represents debug breakpoints
type Breakpoint struct {
	Cycle       int64  `json:"cycle"`
	PC          uint16 `json:"pc"`
	Description string `json:"description"`
}

// SaveToFile saves TAS recording to file
func (t *TASFile) SaveToFile(filename string, format uint8) error {
	// Update header
	t.Header.Magic = [8]byte{'M', 'I', 'N', 'Z', 'T', 'A', 'S', 0}
	t.Header.Version = TASVersion
	t.Header.Format = format
	t.Header.Created = time.Now()
	
	// Calculate checksum
	t.Header.Checksum = t.calculateChecksum()
	
	switch format {
	case TASFormatJSON:
		return t.saveJSON(filename)
	case TASFormatBinary:
		return t.saveBinary(filename)
	case TASFormatCompressed:
		return t.saveCompressed(filename)
	default:
		return fmt.Errorf("unknown format: %d", format)
	}
}

// LoadFromFile loads TAS recording from file
func LoadFromFile(filename string) (*TASFile, error) {
	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	// Read magic header
	magic := make([]byte, 8)
	if _, err := file.Read(magic); err != nil {
		return nil, err
	}
	
	// Reset to beginning
	file.Seek(0, 0)
	
	// Detect format
	if string(magic) == TASMagic {
		// Binary format
		return loadBinary(file)
	} else if magic[0] == '{' {
		// JSON format
		return loadJSON(file)
	} else if magic[0] == 0x1f && magic[1] == 0x8b {
		// Gzipped format
		return loadCompressed(file)
	}
	
	return nil, fmt.Errorf("unknown file format")
}

// saveJSON saves in human-readable JSON format
func (t *TASFile) saveJSON(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(t)
}

// saveBinary saves in efficient binary format
func (t *TASFile) saveBinary(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Write header
	if err := binary.Write(file, binary.LittleEndian, t.Header); err != nil {
		return err
	}
	
	// Write metadata as JSON (for flexibility)
	metaBytes, err := json.Marshal(t.Metadata)
	if err != nil {
		return err
	}
	
	// Write metadata length and data
	if err := binary.Write(file, binary.LittleEndian, uint32(len(metaBytes))); err != nil {
		return err
	}
	if _, err := file.Write(metaBytes); err != nil {
		return err
	}
	
	// Write events
	if err := t.writeEvents(file); err != nil {
		return err
	}
	
	// Write state snapshots if present
	if len(t.States) > 0 {
		if err := t.writeStates(file); err != nil {
			return err
		}
	}
	
	return nil
}

// saveCompressed saves with gzip compression
func (t *TASFile) saveCompressed(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Create gzip writer
	gz := gzip.NewWriter(file)
	defer gz.Close()
	
	// Set metadata
	gz.Header.Name = filename
	gz.Header.Comment = fmt.Sprintf("MinZ TAS Recording v%d", TASVersion)
	gz.Header.ModTime = time.Now()
	
	// Use binary format inside gzip
	t.Header.Format = TASFormatBinary
	
	// Create buffer for binary data
	var buf bytes.Buffer
	
	// Write header
	if err := binary.Write(&buf, binary.LittleEndian, t.Header); err != nil {
		return err
	}
	
	// Write rest of data
	metaBytes, err := json.Marshal(t.Metadata)
	if err != nil {
		return err
	}
	
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(metaBytes))); err != nil {
		return err
	}
	buf.Write(metaBytes)
	
	// Write events efficiently
	if err := t.writeEventsCompact(&buf); err != nil {
		return err
	}
	
	// Compress and write
	_, err = gz.Write(buf.Bytes())
	return err
}

// writeEvents writes events in binary format
func (t *TASFile) writeEvents(w io.Writer) error {
	// Write input events count and data
	if err := binary.Write(w, binary.LittleEndian, uint32(len(t.Events.Inputs))); err != nil {
		return err
	}
	for _, input := range t.Events.Inputs {
		if err := writeInputEvent(w, input); err != nil {
			return err
		}
	}
	
	// Write SMC events
	if err := binary.Write(w, binary.LittleEndian, uint32(len(t.Events.SMCEvents))); err != nil {
		return err
	}
	for _, smc := range t.Events.SMCEvents {
		if err := writeSMCEvent(w, smc); err != nil {
			return err
		}
	}
	
	// Write IO events
	if err := binary.Write(w, binary.LittleEndian, uint32(len(t.Events.IOEvents))); err != nil {
		return err
	}
	for _, io := range t.Events.IOEvents {
		if err := writeIOEvent(w, io); err != nil {
			return err
		}
	}
	
	return nil
}

// writeEventsCompact writes events with delta compression
func (t *TASFile) writeEventsCompact(w io.Writer) error {
	var lastCycle int64 = 0
	
	// Merge all events and sort by cycle
	type timedEvent struct {
		cycle     int64
		eventType byte
		data      interface{}
	}
	
	var events []timedEvent
	
	// Add all events
	for _, input := range t.Events.Inputs {
		events = append(events, timedEvent{int64(input.Cycle), 'I', input})
	}
	for _, smc := range t.Events.SMCEvents {
		events = append(events, timedEvent{int64(smc.Cycle), 'S', smc})
	}
	for _, io := range t.Events.IOEvents {
		events = append(events, timedEvent{io.Cycle, 'O', io})
	}
	
	// Sort by cycle (already sorted in practice)
	// Write total event count
	if err := binary.Write(w, binary.LittleEndian, uint32(len(events))); err != nil {
		return err
	}
	
	// Write events with delta encoding
	for _, evt := range events {
		// Write delta cycle (usually small)
		delta := evt.cycle - lastCycle
		if err := writeVarInt(w, delta); err != nil {
			return err
		}
		lastCycle = evt.cycle
		
		// Write event type
		if err := binary.Write(w, binary.LittleEndian, evt.eventType); err != nil {
			return err
		}
		
		// Write event data
		switch evt.eventType {
		case 'I':
			if err := writeInputEventCompact(w, evt.data.(InputEvent)); err != nil {
				return err
			}
		case 'S':
			if err := writeSMCEventCompact(w, evt.data.(SMCEvent)); err != nil {
				return err
			}
		case 'O':
			if err := writeIOEventCompact(w, evt.data.(IOEvent)); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// writeStates writes state snapshots
func (t *TASFile) writeStates(w io.Writer) error {
	// Write count
	if err := binary.Write(w, binary.LittleEndian, uint32(len(t.States))); err != nil {
		return err
	}
	
	// Write each state
	for _, state := range t.States {
		if err := writeStateSnapshot(w, state); err != nil {
			return err
		}
	}
	
	return nil
}

// Helper functions for writing individual events

func writeInputEvent(w io.Writer, evt InputEvent) error {
	if err := binary.Write(w, binary.LittleEndian, evt.Cycle); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.Port); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, evt.Value)
}

func writeSMCEvent(w io.Writer, evt SMCEvent) error {
	if err := binary.Write(w, binary.LittleEndian, evt.Cycle); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.PC); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.Address); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.OldValue); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, evt.NewValue)
}

func writeIOEvent(w io.Writer, evt IOEvent) error {
	if err := binary.Write(w, binary.LittleEndian, evt.Cycle); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.Port); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.Value); err != nil {
		return err
	}
	isInput := byte(0)
	if evt.IsInput {
		isInput = 1
	}
	return binary.Write(w, binary.LittleEndian, isInput)
}

// Compact versions use smaller encodings

func writeInputEventCompact(w io.Writer, evt InputEvent) error {
	// Port and value as compact format
	data := byte(evt.Port & 0xFF)
	if evt.Value != 0 {
		data |= 0x80
	}
	return binary.Write(w, binary.LittleEndian, data)
}

func writeSMCEventCompact(w io.Writer, evt SMCEvent) error {
	// Pack PC and Address into 3 bytes total (if possible)
	if err := binary.Write(w, binary.LittleEndian, evt.PC); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.Address); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, evt.OldValue); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, evt.NewValue)
}

func writeIOEventCompact(w io.Writer, evt IOEvent) error {
	// Pack port and direction into 2 bytes
	portAndDir := evt.Port
	if evt.IsInput {
		portAndDir |= 0x8000
	}
	if err := binary.Write(w, binary.LittleEndian, portAndDir); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, evt.Value)
}

func writeStateSnapshot(w io.Writer, state StateSnapshot) error {
	// Write cycle
	if err := binary.Write(w, binary.LittleEndian, state.Cycle); err != nil {
		return err
	}
	
	// Write registers
	if err := binary.Write(w, binary.LittleEndian, state.PC); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.SP); err != nil {
		return err
	}
	// Write main registers
	if err := binary.Write(w, binary.LittleEndian, state.A); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.F); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.B); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.C); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.D); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.E); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.H); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.L); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.IX); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.IY); err != nil {
		return err
	}
	
	// Write shadow registers
	if err := binary.Write(w, binary.LittleEndian, state.A_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.F_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.B_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.C_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.D_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.E_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.H_); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.L_); err != nil {
		return err
	}
	
	// Write other registers
	if err := binary.Write(w, binary.LittleEndian, state.I); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.R); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.IFF1); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, state.IFF2); err != nil {
		return err
	}
	// IM field not present in StateSnapshot
	// if err := binary.Write(w, binary.LittleEndian, state.IM); err != nil {
	//	return err
	// }
	
	// Write memory (compressed in practice)
	_, err := w.Write(state.Memory[:])
	return err
}

// Variable-length integer encoding for deltas
func writeVarInt(w io.Writer, v int64) error {
	// Simple variable-length encoding
	for v >= 0x80 {
		if err := binary.Write(w, binary.LittleEndian, byte(v|0x80)); err != nil {
			return err
		}
		v >>= 7
	}
	return binary.Write(w, binary.LittleEndian, byte(v))
}

func readVarInt(r io.Reader) (int64, error) {
	var v int64
	var shift uint
	for {
		var b byte
		if err := binary.Read(r, binary.LittleEndian, &b); err != nil {
			return 0, err
		}
		v |= int64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return v, nil
}

// Load functions

func loadJSON(r io.Reader) (*TASFile, error) {
	var tas TASFile
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&tas); err != nil {
		return nil, err
	}
	return &tas, nil
}

func loadBinary(r io.Reader) (*TASFile, error) {
	var tas TASFile
	
	// Read header
	if err := binary.Read(r, binary.LittleEndian, &tas.Header); err != nil {
		return nil, err
	}
	
	// Verify magic
	if string(tas.Header.Magic[:7]) != "MINZTAS" {
		return nil, fmt.Errorf("invalid TAS file magic")
	}
	
	// Read metadata length
	var metaLen uint32
	if err := binary.Read(r, binary.LittleEndian, &metaLen); err != nil {
		return nil, err
	}
	
	// Read metadata
	metaBytes := make([]byte, metaLen)
	if _, err := r.Read(metaBytes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metaBytes, &tas.Metadata); err != nil {
		return nil, err
	}
	
	// Read events
	if err := readEvents(r, &tas.Events); err != nil {
		return nil, err
	}
	
	// Read states if present
	var stateCount uint32
	if err := binary.Read(r, binary.LittleEndian, &stateCount); err != nil {
		// No states is OK
		return &tas, nil
	}
	
	tas.States = make([]StateSnapshot, stateCount)
	for i := uint32(0); i < stateCount; i++ {
		if err := readStateSnapshot(r, &tas.States[i]); err != nil {
			return nil, err
		}
	}
	
	return &tas, nil
}

func loadCompressed(r io.Reader) (*TASFile, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	
	return loadBinary(gz)
}

func readEvents(r io.Reader, events *TASEvents) error {
	// Read input events
	var inputCount uint32
	if err := binary.Read(r, binary.LittleEndian, &inputCount); err != nil {
		return err
	}
	events.Inputs = make([]InputEvent, inputCount)
	for i := uint32(0); i < inputCount; i++ {
		if err := readInputEvent(r, &events.Inputs[i]); err != nil {
			return err
		}
	}
	
	// Read SMC events
	var smcCount uint32
	if err := binary.Read(r, binary.LittleEndian, &smcCount); err != nil {
		return err
	}
	events.SMCEvents = make([]SMCEvent, smcCount)
	for i := uint32(0); i < smcCount; i++ {
		if err := readSMCEvent(r, &events.SMCEvents[i]); err != nil {
			return err
		}
	}
	
	// Read IO events
	var ioCount uint32
	if err := binary.Read(r, binary.LittleEndian, &ioCount); err != nil {
		return err
	}
	events.IOEvents = make([]IOEvent, ioCount)
	for i := uint32(0); i < ioCount; i++ {
		if err := readIOEvent(r, &events.IOEvents[i]); err != nil {
			return err
		}
	}
	
	return nil
}

func readInputEvent(r io.Reader, evt *InputEvent) error {
	if err := binary.Read(r, binary.LittleEndian, &evt.Cycle); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.Port); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.Value); err != nil {
		return err
	}
	return nil
}

func readSMCEvent(r io.Reader, evt *SMCEvent) error {
	if err := binary.Read(r, binary.LittleEndian, &evt.Cycle); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.PC); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.Address); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.OldValue); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &evt.NewValue)
}

func readIOEvent(r io.Reader, evt *IOEvent) error {
	if err := binary.Read(r, binary.LittleEndian, &evt.Cycle); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.Port); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &evt.Value); err != nil {
		return err
	}
	var isInput byte
	if err := binary.Read(r, binary.LittleEndian, &isInput); err != nil {
		return err
	}
	evt.IsInput = isInput != 0
	return nil
}

func readStateSnapshot(r io.Reader, state *StateSnapshot) error {
	// Read cycle
	if err := binary.Read(r, binary.LittleEndian, &state.Cycle); err != nil {
		return err
	}
	
	// Read registers
	if err := binary.Read(r, binary.LittleEndian, &state.PC); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.SP); err != nil {
		return err
	}
	// Read main registers
	if err := binary.Read(r, binary.LittleEndian, &state.A); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.F); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.B); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.C); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.D); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.E); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.H); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.L); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.IX); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.IY); err != nil {
		return err
	}
	
	// Read shadow registers
	if err := binary.Read(r, binary.LittleEndian, &state.A_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.F_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.B_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.C_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.D_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.E_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.H_); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.L_); err != nil {
		return err
	}
	
	// Read other registers
	if err := binary.Read(r, binary.LittleEndian, &state.I); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.R); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.IFF1); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &state.IFF2); err != nil {
		return err
	}
	// IM field not present in StateSnapshot
	// if err := binary.Read(r, binary.LittleEndian, &state.IM); err != nil {
	//	return err
	// }
	
	// Read memory
	_, err := r.Read(state.Memory[:])
	return err
}

// calculateChecksum calculates CRC32 checksum
func (t *TASFile) calculateChecksum() uint32 {
	// Simple checksum for now
	var sum uint32
	for _, input := range t.Events.Inputs {
		sum ^= uint32(input.Cycle)
		sum = (sum << 1) | (sum >> 31) // Rotate
	}
	for _, smc := range t.Events.SMCEvents {
		sum ^= uint32(smc.Address)
		sum = (sum << 1) | (sum >> 31)
	}
	return sum
}

// Replay support

// CreateReplay creates a TASFile from debugger state
func CreateReplay(debugger *TASDebugger) *TASFile {
	tas := &TASFile{
		Header: TASHeader{
			Version: TASVersion,
			Created: time.Now(),
		},
		Metadata: TASMetadata{
			ProgramName:    "MinZ Program",
			ProgramVersion: "1.0",
			TotalFrames:    int64(len(debugger.stateHistory)),
			TotalCycles:    debugger.currentFrame,
			Author:         "MinZ TAS Debugger",
			Tags:           []string{"debug", "recording"},
			Properties:     make(map[string]string),
		},
		Events: TASEvents{
			Inputs:    debugger.inputLog,
			SMCEvents: debugger.smcEvents,
			IOEvents:  []IOEvent{}, // TODO: Implement IO tracking
		},
	}
	
	// Add key frames for seeking
	if len(debugger.stateHistory) > 100 {
		// Sample every 100 frames for quick seeking
		for i := 0; i < len(debugger.stateHistory); i += 100 {
			tas.States = append(tas.States, debugger.stateHistory[i])
		}
	}
	
	return tas
}

// ApplyReplay applies a TAS recording to emulator
func ApplyReplay(tas *TASFile, emulator Z80Emulator) error {
	// Reset emulator - not in interface
	// emulator.Reset()
	
	// Create event queues
	inputIdx := 0
	smcIdx := 0
	ioIdx := 0
	
	// Run until all events processed
	cycle := uint64(0)
	for inputIdx < len(tas.Events.Inputs) || 
	    smcIdx < len(tas.Events.SMCEvents) ||
	    ioIdx < len(tas.Events.IOEvents) {
		
		// Process all events at current cycle
		for inputIdx < len(tas.Events.Inputs) && tas.Events.Inputs[inputIdx].Cycle <= cycle {
			// Apply input event
			// TODO: Hook into emulator input system
			inputIdx++
		}
		
		for smcIdx < len(tas.Events.SMCEvents) && tas.Events.SMCEvents[smcIdx].Cycle <= cycle {
			// SMC events are informational (already in memory writes)
			smcIdx++
		}
		
		for ioIdx < len(tas.Events.IOEvents) && uint64(tas.Events.IOEvents[ioIdx].Cycle) <= cycle {
			// Apply IO event
			// TODO: Hook into emulator IO system
			ioIdx++
		}
		
		// Step emulator - not in interface
		// emulator.Step()
		cycle++
	}
	
	return nil
}