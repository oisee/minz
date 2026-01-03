// Package dzrp implements the DeZog Remote Protocol for Z80 debugging
//
// DZRP is a socket-based protocol (TCP port 11000) used by DeZog VS Code
// extension for ZX Spectrum debugging. This implementation provides a
// bridge between MZE (MinZ Emulator) and DeZog.
//
// Protocol Specification:
// - All messages are length-prefixed (4 bytes, big-endian)
// - First byte after length is command/response ID
// - Followed by command-specific data
//
// Reference: https://github.com/maziac/DeZog

package dzrp

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Protocol version
const (
	DZRP_VERSION_MAJOR = 2
	DZRP_VERSION_MINOR = 0
	DZRP_VERSION_PATCH = 0
)

// Command IDs (from DeZog specification)
const (
	// Requests from DeZog
	CMD_INIT                = 1
	CMD_CLOSE               = 2
	CMD_GET_REGISTERS       = 3
	CMD_SET_REGISTER        = 4
	CMD_WRITE_BANK          = 5
	CMD_CONTINUE            = 6
	CMD_PAUSE               = 7
	CMD_ADD_BREAKPOINT      = 8
	CMD_REMOVE_BREAKPOINT   = 9
	CMD_ADD_WATCHPOINT      = 10 // Not implemented
	CMD_REMOVE_WATCHPOINT   = 11 // Not implemented
	CMD_READ_MEM            = 12
	CMD_WRITE_MEM           = 13
	CMD_GET_SLOTS           = 14
	CMD_READ_STATE          = 15
	CMD_WRITE_STATE         = 16
	CMD_GET_TBBLUE_REG      = 17 // ZX Spectrum Next specific
	CMD_GET_SPRITES_PALETTE = 18 // Not implemented
	CMD_GET_SPRITES         = 19 // Not implemented
	CMD_GET_SPRITE_PATTERNS = 20 // Not implemented
	CMD_GET_SPRITES_CLIP    = 21 // Not implemented

	// Notifications (from emulator to DeZog)
	NTF_PAUSE = 1 // Sent when emulator pauses (breakpoint, etc.)
)

// Machine types for CMD_INIT response
const (
	MACHINE_ZX48K      = 1
	MACHINE_ZX128K     = 2
	MACHINE_ZXNEXT     = 5
	MACHINE_ZX48K_LITE = 10 // Simplified, no contention
)

// Breakpoint types
const (
	BP_EXEC  = 0 // Execution breakpoint (at PC)
	BP_READ  = 1 // Memory read watchpoint
	BP_WRITE = 2 // Memory write watchpoint
)

// Register indices for CMD_GET_REGISTERS / CMD_SET_REGISTER
const (
	REG_PC    = 0
	REG_SP    = 1
	REG_AF    = 2
	REG_BC    = 3
	REG_DE    = 4
	REG_HL    = 5
	REG_IX    = 6
	REG_IY    = 7
	REG_AF2   = 8  // Alternate AF
	REG_BC2   = 9  // Alternate BC
	REG_DE2   = 10 // Alternate DE
	REG_HL2   = 11 // Alternate HL
	REG_I     = 12
	REG_R     = 13
	REG_IM    = 14 // Interrupt mode
	REG_IFF1  = 15
	REG_IFF2  = 16
)

// Message represents a DZRP protocol message
type Message struct {
	Command byte
	Data    []byte
}

// ReadMessage reads a DZRP message from a reader
func ReadMessage(r io.Reader) (*Message, error) {
	// Read 4-byte length prefix (big-endian)
	lenBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length < 1 {
		return nil, fmt.Errorf("invalid message length: %d", length)
	}

	// Limit max message size (64KB should be plenty)
	if length > 65536 {
		return nil, fmt.Errorf("message too large: %d", length)
	}

	// Read message body
	body := make([]byte, length)
	_, err = io.ReadFull(r, body)
	if err != nil {
		return nil, err
	}

	return &Message{
		Command: body[0],
		Data:    body[1:],
	}, nil
}

// WriteMessage writes a DZRP message to a writer
func WriteMessage(w io.Writer, cmd byte, data []byte) error {
	length := uint32(1 + len(data)) // command byte + data

	// Write length prefix
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, length)
	_, err := w.Write(lenBuf)
	if err != nil {
		return err
	}

	// Write command byte
	_, err = w.Write([]byte{cmd})
	if err != nil {
		return err
	}

	// Write data
	if len(data) > 0 {
		_, err = w.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteResponse writes a response message (uses same format as WriteMessage)
func WriteResponse(w io.Writer, origCmd byte, data []byte) error {
	// Response uses the original command ID as the "response" ID
	return WriteMessage(w, origCmd, data)
}

// WriteNotification writes a notification message
func WriteNotification(w io.Writer, notifType byte, data []byte) error {
	// Notifications use command ID 0, followed by notification type
	body := append([]byte{notifType}, data...)
	return WriteMessage(w, 0, body)
}

// EncodeU16LE encodes a uint16 in little-endian
func EncodeU16LE(v uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, v)
	return buf
}

// DecodeU16LE decodes a uint16 from little-endian bytes
func DecodeU16LE(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

// EncodeU32LE encodes a uint32 in little-endian
func EncodeU32LE(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

// DecodeU32LE decodes a uint32 from little-endian bytes
func DecodeU32LE(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}
