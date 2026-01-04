// tap.go - TAP file parser for DZRP loading
// Enables instant loading of ZX Spectrum TAP files via DZRP protocol

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// TapBlockType represents the type of data in a TAP header
type TapBlockType byte

const (
	TapTypeProgram   TapBlockType = 0 // BASIC program
	TapTypeNumArray  TapBlockType = 1 // Numeric array
	TapTypeCharArray TapBlockType = 2 // Character array
	TapTypeCode      TapBlockType = 3 // CODE/Bytes
)

// TapHeader represents a parsed TAP header block
type TapHeader struct {
	Type       TapBlockType
	Name       string
	DataLength uint16
	Param1     uint16 // For CODE: start address; for BASIC: autostart line
	Param2     uint16 // For CODE: 32768; for BASIC: start of vars offset
}

// TapBlock represents a header + data pair from a TAP file
type TapBlock struct {
	Header *TapHeader // nil for orphan data blocks
	Data   []byte     // actual data (without flag and checksum)
}

// TapFile represents a parsed TAP file
type TapFile struct {
	Blocks []TapBlock
}

// ParseTapFile reads and parses a TAP file
func ParseTapFile(filename string) (*TapFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read TAP file: %w", err)
	}

	tap := &TapFile{}
	var pendingHeader *TapHeader

	offset := 0
	for offset < len(data) {
		// Read block length (2 bytes, little-endian)
		if offset+2 > len(data) {
			break
		}
		blockLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+blockLen > len(data) {
			return nil, fmt.Errorf("truncated block at offset %d", offset-2)
		}

		blockData := data[offset : offset+blockLen]
		offset += blockLen

		if blockLen < 2 {
			continue // Invalid block
		}

		flag := blockData[0]
		// checksum := blockData[blockLen-1] // Last byte is checksum

		// Flag 0x00 = header, 0xFF = data
		if flag == 0x00 && blockLen == 19 {
			// Header block
			header := parseHeader(blockData[1 : blockLen-1])
			pendingHeader = header
		} else if flag == 0xFF {
			// Data block
			actualData := blockData[1 : blockLen-1] // Remove flag and checksum
			block := TapBlock{
				Header: pendingHeader,
				Data:   actualData,
			}
			tap.Blocks = append(tap.Blocks, block)
			pendingHeader = nil
		}
	}

	return tap, nil
}

func parseHeader(data []byte) *TapHeader {
	if len(data) < 17 {
		return nil
	}

	// Extract name (10 bytes, space-padded)
	name := strings.TrimRight(string(data[1:11]), " ")

	return &TapHeader{
		Type:       TapBlockType(data[0]),
		Name:       name,
		DataLength: binary.LittleEndian.Uint16(data[11:13]),
		Param1:     binary.LittleEndian.Uint16(data[13:15]),
		Param2:     binary.LittleEndian.Uint16(data[15:17]),
	}
}

// GetCodeBlocks returns all CODE blocks from the TAP file
func (t *TapFile) GetCodeBlocks() []TapBlock {
	var result []TapBlock
	for _, block := range t.Blocks {
		if block.Header != nil && block.Header.Type == TapTypeCode {
			result = append(result, block)
		}
	}
	return result
}

// GetFirstCodeBlock returns the first CODE block, or nil if none
func (t *TapFile) GetFirstCodeBlock() *TapBlock {
	for _, block := range t.Blocks {
		if block.Header != nil && block.Header.Type == TapTypeCode {
			return &block
		}
	}
	return nil
}

// GetAllLoadableBlocks returns all blocks that can be loaded to memory
// (CODE blocks with known addresses)
func (t *TapFile) GetAllLoadableBlocks() []struct {
	Address uint16
	Data    []byte
	Name    string
} {
	var result []struct {
		Address uint16
		Data    []byte
		Name    string
	}

	for _, block := range t.Blocks {
		if block.Header != nil && block.Header.Type == TapTypeCode {
			result = append(result, struct {
				Address uint16
				Data    []byte
				Name    string
			}{
				Address: block.Header.Param1,
				Data:    block.Data,
				Name:    block.Header.Name,
			})
		}
	}

	return result
}

// PrintInfo displays information about the TAP file
func (t *TapFile) PrintInfo() {
	fmt.Printf("TAP file contains %d block(s):\n", len(t.Blocks))
	for i, block := range t.Blocks {
		if block.Header != nil {
			switch block.Header.Type {
			case TapTypeProgram:
				fmt.Printf("  %d. BASIC Program: \"%s\" (%d bytes, autostart: %d)\n",
					i+1, block.Header.Name, len(block.Data), block.Header.Param1)
			case TapTypeCode:
				fmt.Printf("  %d. CODE: \"%s\" (%d bytes at $%04X)\n",
					i+1, block.Header.Name, len(block.Data), block.Header.Param1)
			case TapTypeNumArray:
				fmt.Printf("  %d. Numeric Array: \"%s\" (%d bytes)\n",
					i+1, block.Header.Name, len(block.Data))
			case TapTypeCharArray:
				fmt.Printf("  %d. Character Array: \"%s\" (%d bytes)\n",
					i+1, block.Header.Name, len(block.Data))
			}
		} else {
			fmt.Printf("  %d. Data block: %d bytes (no header)\n", i+1, len(block.Data))
		}
	}
}

// String returns a brief description of the TAP file
func (t *TapFile) String() string {
	codeBlocks := t.GetCodeBlocks()
	if len(codeBlocks) > 0 {
		first := codeBlocks[0]
		return fmt.Sprintf("TAP: \"%s\" at $%04X (%d bytes)",
			first.Header.Name, first.Header.Param1, len(first.Data))
	}
	return fmt.Sprintf("TAP: %d blocks", len(t.Blocks))
}
