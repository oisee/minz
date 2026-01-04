// mztap - TAP file loader for ZX Spectrum emulators via DZRP
// Loads TAP files directly to emulator memory and executes CODE blocks
//
// TAP format:
//   - Series of blocks, each prefixed with 2-byte length (LE)
//   - Header blocks: flag=0x00, type, filename[10], datalen[2], param1[2], param2[2], checksum
//   - Data blocks: flag=0xFF, data[n], checksum
//
// Environment variables:
//   DZRP_HOST   - Default emulator host (default: localhost)
//   DZRP_PORT   - Default emulator port (default: 11000)
//   DZRP_SOCKET - Socket type: tcp (default) or ws (WebSocket)
//
// Usage:
//   mztap program.tap                   # Load and run CODE blocks
//   mztap --list program.tap            # Just list blocks
//   mztap --host 192.168.1.5 program.tap # Remote emulator
//   mztap --load 0x8000 --start 0x8000 program.tap  # Override addresses
//   mztap --set "AF=0xFF00,HL=0x4000" program.tap   # Set registers

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// DZRP Commands
const (
	CMD_INIT         = 1
	CMD_CLOSE        = 2
	CMD_GET_REGISTERS = 3
	CMD_SET_REGISTER = 4
	CMD_CONTINUE     = 6
	CMD_PAUSE        = 7
	CMD_READ_MEM     = 8
	CMD_WRITE_MEM    = 9
)

// TAP block types
const (
	TAP_TYPE_PROGRAM  = 0 // BASIC program
	TAP_TYPE_NUMARRAY = 1 // Number array
	TAP_TYPE_CHARARRAY = 2 // Character array
	TAP_TYPE_CODE     = 3 // CODE/bytes
)

// TAPBlock represents a block in a TAP file
type TAPBlock struct {
	IsHeader   bool
	BlockType  byte   // 0=Program, 1=NumArray, 2=CharArray, 3=Code
	Filename   string
	DataLength uint16
	Param1     uint16 // Start address for CODE, LINE for BASIC
	Param2     uint16 // 32768 for CODE, variable offset for others
	Data       []byte // Actual data bytes
	RawData    []byte // Original block data including flag+checksum
}

var seqNum byte = 1

var (
	host       string
	port       int
	socketType string // tcp or ws (WebSocket)
	list       bool
	verbose    bool
	run        bool
	timeout    int
	loadAddr   string // Override load address (hex/dec/oct)
	startAddr  string // Override start address
	setRegs    string // Register values: "AF=0x1234,BC=0x5678"
)

// Register indices for DZRP CMD_SET_REGISTER
var regIndex = map[string]byte{
	"PC": 0, "SP": 1, "AF": 2, "BC": 3, "DE": 4, "HL": 5,
	"IX": 6, "IY": 7, "AF'": 8, "BC'": 9, "DE'": 10, "HL'": 11,
	"I": 12, "R": 13,
}

var rootCmd = &cobra.Command{
	Use:   "mztap [tap file]",
	Short: "TAP file loader - Load ZX Spectrum TAP files via DZRP",
	Long: `mztap loads ZX Spectrum TAP files directly to any DZRP-compatible emulator.

It parses the TAP file, extracts CODE blocks with their load addresses,
and uploads them to the emulator's memory. CODE blocks are then executed.

Supported emulators:
  - ZXSpeculator (https://github.com/deanthecoder/ZXSpeculator)
  - ZEsarUX     (built-in DZRP support)
  - CSpect      (with DeZog plugin)

Environment variables:
  DZRP_HOST   - Default emulator host (overridden by --host)
  DZRP_PORT   - Default emulator port (overridden by --port)
  DZRP_SOCKET - Socket type: tcp (default) or ws (WebSocket)

Address formats (for --load, --start):
  0x8000  - Hexadecimal (0x prefix)
  $8000   - Hexadecimal ($ prefix)
  32768   - Decimal
  0100000 - Octal (leading 0)

Examples:
  mztap program.tap                    # Load and run CODE blocks
  mztap --list program.tap             # List TAP contents
  mztap --host 192.168.1.5 program.tap # Load to remote emulator
  mztap --load 0x9000 program.tap      # Override load address
  mztap --start $8100 program.tap      # Override start address
  mztap --set "AF=0xFF,BC=$1234" prog.tap  # Set registers before run
  mztap --verbose program.tap          # Show detailed output`,
	Args: cobra.ExactArgs(1),
	RunE: runLoader,
}

func init() {
	// Default from environment, fallback to sensible defaults
	defaultHost := os.Getenv("DZRP_HOST")
	if defaultHost == "" {
		defaultHost = "localhost"
	}
	defaultPort := 11000
	if envPort := os.Getenv("DZRP_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			defaultPort = p
		}
	}
	defaultSocket := os.Getenv("DZRP_SOCKET")
	if defaultSocket == "" {
		defaultSocket = "tcp"
	}

	rootCmd.Flags().StringVar(&host, "host", defaultHost, "DZRP emulator host/IP (env: DZRP_HOST)")
	rootCmd.Flags().IntVar(&port, "port", defaultPort, "DZRP port (env: DZRP_PORT)")
	rootCmd.Flags().StringVar(&socketType, "socket", defaultSocket, "Socket type: tcp or ws (env: DZRP_SOCKET)")
	rootCmd.Flags().BoolVar(&list, "list", false, "List TAP contents without loading")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVar(&run, "run", true, "Run CODE blocks after loading (default true)")
	rootCmd.Flags().IntVar(&timeout, "timeout", 0, "Execution timeout in seconds (0 = run forever)")
	rootCmd.Flags().StringVar(&loadAddr, "load", "", "Override load address (hex: 0x8000/$8000, dec: 32768)")
	rootCmd.Flags().StringVar(&startAddr, "start", "", "Override start/run address")
	rootCmd.Flags().StringVar(&setRegs, "set", "", "Set registers before run: \"AF=0xFF,BC=$1234\"")
}

// parseAddress parses hex (0x, $), octal (0), or decimal
func parseAddress(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty address")
	}

	// Handle $ prefix for hex (common in Z80 world)
	if strings.HasPrefix(s, "$") {
		val, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(val), err
	}

	// Handle 0x prefix for hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		val, err := strconv.ParseUint(s[2:], 16, 16)
		return uint16(val), err
	}

	// Handle 0 prefix for octal (but not just "0")
	if strings.HasPrefix(s, "0") && len(s) > 1 && s[1] >= '0' && s[1] <= '7' {
		val, err := strconv.ParseUint(s[1:], 8, 16)
		return uint16(val), err
	}

	// Decimal
	val, err := strconv.ParseUint(s, 10, 16)
	return uint16(val), err
}

// parseRegisters parses "AF=0x1234,BC=$5678" format
func parseRegisters(s string) (map[string]uint16, error) {
	result := make(map[string]uint16)
	if s == "" {
		return result, nil
	}

	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid register format: %s (expected REG=VALUE)", pair)
		}

		regName := strings.ToUpper(strings.TrimSpace(parts[0]))
		if _, ok := regIndex[regName]; !ok {
			return nil, fmt.Errorf("unknown register: %s", regName)
		}

		val, err := parseAddress(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", regName, err)
		}

		result[regName] = val
	}

	return result, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runLoader(cmd *cobra.Command, args []string) error {
	tapFile := args[0]

	// Parse TAP file
	blocks, err := parseTAP(tapFile)
	if err != nil {
		return fmt.Errorf("failed to parse TAP: %w", err)
	}

	// List mode - just show contents
	if list {
		fmt.Printf("TAP file: %s\n", tapFile)
		fmt.Println("=========================================")
		for i, block := range blocks {
			if block.IsHeader {
				typeName := getTypeName(block.BlockType)
				fmt.Printf("[%d] HEADER: %-10s Type=%-8s Len=%5d",
					i, block.Filename, typeName, block.DataLength)
				if block.BlockType == TAP_TYPE_CODE {
					fmt.Printf(" Load=$%04X", block.Param1)
				} else if block.BlockType == TAP_TYPE_PROGRAM {
					fmt.Printf(" LINE=%d", block.Param1)
				}
				fmt.Println()
			} else {
				fmt.Printf("[%d] DATA:   %d bytes\n", i, len(block.Data))
			}
		}
		return nil
	}

	// Find CODE blocks to load
	var codeBlocks []struct {
		name string
		addr uint16
		data []byte
	}

	for i := 0; i < len(blocks)-1; i++ {
		if blocks[i].IsHeader && blocks[i].BlockType == TAP_TYPE_CODE {
			// Next block should be the data
			if !blocks[i+1].IsHeader {
				codeBlocks = append(codeBlocks, struct {
					name string
					addr uint16
					data []byte
				}{
					name: blocks[i].Filename,
					addr: blocks[i].Param1,
					data: blocks[i+1].Data,
				})
			}
		}
	}

	if len(codeBlocks) == 0 {
		return fmt.Errorf("no CODE blocks found in TAP file")
	}

	// Apply load address override if specified
	if loadAddr != "" {
		overrideAddr, err := parseAddress(loadAddr)
		if err != nil {
			return fmt.Errorf("invalid --load address: %w", err)
		}
		for i := range codeBlocks {
			codeBlocks[i].addr = overrideAddr
		}
		if verbose {
			fmt.Printf("Load address overridden to $%04X\n", overrideAddr)
		}
	}

	fmt.Printf("Found %d CODE block(s) to load:\n", len(codeBlocks))
	for _, cb := range codeBlocks {
		fmt.Printf("  - %s: %d bytes at $%04X\n", strings.TrimSpace(cb.name), len(cb.data), cb.addr)
	}

	// Parse register settings
	regSettings, err := parseRegisters(setRegs)
	if err != nil {
		return fmt.Errorf("invalid --set: %w", err)
	}

	// Connect to emulator
	if verbose {
		fmt.Printf("\nConnecting to %s:%d...\n", host, port)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to DZRP emulator at %s: %w", addr, err)
	}
	defer conn.Close()

	if verbose {
		fmt.Println("Connected! Initializing DZRP session...")
	}

	// Initialize DZRP session
	if err := dzrpInit(conn); err != nil {
		return fmt.Errorf("DZRP init failed: %w", err)
	}

	// Pause emulation
	if err := dzrpPause(conn); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	// Load each CODE block
	var firstAddr uint16
	for i, cb := range codeBlocks {
		if verbose {
			fmt.Printf("Loading %s (%d bytes) at $%04X...\n",
				strings.TrimSpace(cb.name), len(cb.data), cb.addr)
		}
		if err := dzrpWriteMem(conn, cb.addr, cb.data); err != nil {
			return fmt.Errorf("failed to write %s: %w", cb.name, err)
		}
		if i == 0 {
			firstAddr = cb.addr
		}
	}

	fmt.Printf("Loaded %d CODE block(s) successfully!\n", len(codeBlocks))

	// Apply start address override
	runAddr := firstAddr
	if startAddr != "" {
		overrideStart, err := parseAddress(startAddr)
		if err != nil {
			return fmt.Errorf("invalid --start address: %w", err)
		}
		runAddr = overrideStart
		if verbose {
			fmt.Printf("Start address overridden to $%04X\n", runAddr)
		}
	}

	// Run if requested
	if run {
		// Set custom registers before running
		for regName, regVal := range regSettings {
			idx := regIndex[regName]
			if err := dzrpSetRegister(conn, idx, regVal); err != nil {
				return fmt.Errorf("failed to set %s: %w", regName, err)
			}
			if verbose {
				fmt.Printf("Set %s=$%04X\n", regName, regVal)
			}
		}

		fmt.Printf("Starting execution at $%04X...\n", runAddr)

		if err := dzrpSetPC(conn, runAddr); err != nil {
			return fmt.Errorf("failed to set PC: %w", err)
		}

		if err := dzrpContinue(conn); err != nil {
			return fmt.Errorf("failed to continue: %w", err)
		}

		if timeout > 0 {
			time.Sleep(time.Duration(timeout) * time.Second)
			if err := dzrpPause(conn); err != nil {
				return fmt.Errorf("failed to pause: %w", err)
			}
			regs, err := dzrpGetRegisters(conn)
			if err != nil {
				return fmt.Errorf("failed to get registers: %w", err)
			}
			fmt.Printf("\n=== Execution Complete ===\n")
			fmt.Printf("PC=$%04X SP=$%04X AF=$%04X BC=$%04X DE=$%04X HL=$%04X\n",
				regs["PC"], regs["SP"], regs["AF"], regs["BC"], regs["DE"], regs["HL"])
		} else {
			fmt.Println("Program running (use emulator to stop)")
		}
	}

	return nil
}

func parseTAP(filename string) ([]TAPBlock, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var blocks []TAPBlock
	offset := 0

	for offset < len(data)-2 {
		// Read block length (2 bytes LE)
		if offset+2 > len(data) {
			break
		}
		blockLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if blockLen == 0 || offset+blockLen > len(data) {
			break
		}

		// Read block data
		blockData := data[offset : offset+blockLen]
		offset += blockLen

		block := TAPBlock{
			RawData: blockData,
		}

		if len(blockData) < 2 {
			continue
		}

		flag := blockData[0]

		if flag == 0x00 && len(blockData) >= 18 {
			// Header block
			block.IsHeader = true
			block.BlockType = blockData[1]

			// Extract filename (bytes 2-11)
			filename := string(blockData[2:12])
			block.Filename = filename

			// Data length (bytes 12-13)
			block.DataLength = binary.LittleEndian.Uint16(blockData[12:14])

			// Param1 (bytes 14-15) - start address for CODE, LINE for BASIC
			block.Param1 = binary.LittleEndian.Uint16(blockData[14:16])

			// Param2 (bytes 16-17)
			block.Param2 = binary.LittleEndian.Uint16(blockData[16:18])

		} else if flag == 0xFF {
			// Data block
			block.IsHeader = false
			// Data is everything except flag byte and checksum
			if len(blockData) > 2 {
				block.Data = blockData[1 : len(blockData)-1]
			}
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

func getTypeName(t byte) string {
	switch t {
	case TAP_TYPE_PROGRAM:
		return "PROGRAM"
	case TAP_TYPE_NUMARRAY:
		return "NUMARRAY"
	case TAP_TYPE_CHARARRAY:
		return "CHARARRAY"
	case TAP_TYPE_CODE:
		return "CODE"
	default:
		return fmt.Sprintf("TYPE_%d", t)
	}
}

// DZRP Protocol Functions

func dzrpSend(conn net.Conn, cmd byte, data []byte) error {
	length := uint32(2 + len(data))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{seqNum, cmd}); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := conn.Write(data); err != nil {
			return err
		}
	}
	seqNum++
	return nil
}

func dzrpRecv(conn net.Conn) (byte, []byte, error) {
	for {
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return 0, nil, err
		}
		length := binary.LittleEndian.Uint32(lenBuf)

		if length < 2 {
			return 0, nil, fmt.Errorf("invalid message length: %d", length)
		}

		msgBuf := make([]byte, length)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			return 0, nil, err
		}

		seqNum := msgBuf[0]
		cmdId := msgBuf[1]

		if verbose {
			if seqNum == 0 {
				fmt.Printf("  <- notification: cmd=%d\n", cmdId)
			}
		}

		// Skip notifications (seq=0)
		if seqNum == 0 {
			continue
		}

		return cmdId, msgBuf[2:], nil
	}
}

func dzrpInit(conn net.Conn) error {
	if err := dzrpSend(conn, CMD_INIT, nil); err != nil {
		return err
	}
	_, _, err := dzrpRecv(conn)
	return err
}

func dzrpPause(conn net.Conn) error {
	if err := dzrpSend(conn, CMD_PAUSE, nil); err != nil {
		return err
	}
	_, _, err := dzrpRecv(conn)
	return err
}

func dzrpContinue(conn net.Conn) error {
	if err := dzrpSend(conn, CMD_CONTINUE, nil); err != nil {
		return err
	}
	_, _, err := dzrpRecv(conn)
	return err
}

func dzrpWriteMem(conn net.Conn, addr uint16, data []byte) error {
	for len(data) > 0 {
		chunkSize := 256
		if len(data) < chunkSize {
			chunkSize = len(data)
		}

		payload := make([]byte, 4+chunkSize)
		binary.LittleEndian.PutUint16(payload[0:2], addr)
		binary.LittleEndian.PutUint16(payload[2:4], uint16(chunkSize))
		copy(payload[4:], data[:chunkSize])

		if err := dzrpSend(conn, CMD_WRITE_MEM, payload); err != nil {
			return err
		}
		if _, _, err := dzrpRecv(conn); err != nil {
			return err
		}

		addr += uint16(chunkSize)
		data = data[chunkSize:]
	}
	return nil
}

func dzrpSetPC(conn net.Conn, pc uint16) error {
	data := make([]byte, 3)
	data[0] = 0 // Register index for PC
	binary.LittleEndian.PutUint16(data[1:3], pc)
	if err := dzrpSend(conn, CMD_SET_REGISTER, data); err != nil {
		return err
	}
	_, _, err := dzrpRecv(conn)
	return err
}

func dzrpSetRegister(conn net.Conn, regIdx byte, value uint16) error {
	data := make([]byte, 3)
	data[0] = regIdx
	binary.LittleEndian.PutUint16(data[1:3], value)
	if err := dzrpSend(conn, CMD_SET_REGISTER, data); err != nil {
		return err
	}
	_, _, err := dzrpRecv(conn)
	return err
}

func dzrpGetRegisters(conn net.Conn) (map[string]uint16, error) {
	if err := dzrpSend(conn, CMD_GET_REGISTERS, nil); err != nil {
		return nil, err
	}

	_, data, err := dzrpRecv(conn)
	if err != nil {
		return nil, err
	}

	if len(data) < 1 {
		return nil, fmt.Errorf("empty response")
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("error code: %d", data[0])
	}

	regs := make(map[string]uint16)
	data = data[1:]
	if len(data) >= 28 {
		regs["PC"] = binary.LittleEndian.Uint16(data[0:2])
		regs["SP"] = binary.LittleEndian.Uint16(data[2:4])
		regs["AF"] = binary.LittleEndian.Uint16(data[4:6])
		regs["BC"] = binary.LittleEndian.Uint16(data[6:8])
		regs["DE"] = binary.LittleEndian.Uint16(data[8:10])
		regs["HL"] = binary.LittleEndian.Uint16(data[10:12])
		regs["IX"] = binary.LittleEndian.Uint16(data[12:14])
		regs["IY"] = binary.LittleEndian.Uint16(data[14:16])
	}
	return regs, nil
}
