// mzrun - MinZ Remote Runner
// Compiles and runs MinZ programs on any DZRP-compatible emulator
//
// Supported emulators:
//   - ZEsarUX (https://github.com/chernandezba/zesarux)
//   - ZXSpeculator (https://github.com/deanthecoder/ZXSpeculator)
//   - CSpect (with DeZog plugin)
//   - Any emulator implementing DeZog Remote Protocol
//
// Usage:
//   mzrun program.minz              # Compile and run
//   mzrun --host 192.168.1.5 prog.minz  # Remote emulator
//   mzrun --step program.minz       # Run with stepping

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

// DZRP Commands (per ZXSpeculator/DeZog protocol)
const (
	CMD_INIT              = 1
	CMD_CLOSE             = 2
	CMD_GET_REGISTERS     = 3
	CMD_SET_REGISTER      = 4
	CMD_CONTINUE          = 6
	CMD_PAUSE             = 7
	CMD_READ_MEM          = 8
	CMD_WRITE_MEM         = 9
	CMD_STEP_INTO         = 17
	CMD_ADD_BREAKPOINT    = 40
	CMD_REMOVE_BREAKPOINT = 41
)

var seqNum byte = 1

var (
	host      string
	port      int
	loadAddr  uint16
	startAddr uint16
	timeout   int
	verbose   bool
	step      bool
	reset     bool
	dump      uint16
	debug     bool
)

var rootCmd = &cobra.Command{
	Use:   "mzrun [minz file or binary]",
	Short: "MinZ Remote Runner - Execute on any DZRP-compatible emulator",
	Long: `mzrun compiles MinZ programs and runs them on any emulator supporting
the DeZog Remote Protocol (DZRP).

Supported emulators:
  - ZEsarUX      (built-in DZRP support)
  - ZXSpeculator (with DZRP fork)
  - CSpect       (with DeZog plugin)
  - Any DZRP-compatible emulator

This enables testing MinZ programs on accurate ZX Spectrum emulators
without needing a local display - perfect for CI/CD and headless testing.

Examples:
  mzrun program.minz                    # Compile and run (localhost:11000)
  mzrun --host 192.168.1.5 program.minz # Run on remote emulator
  mzrun --verbose program.minz          # Show register state
  mzrun program.bin --load 0x8000       # Run pre-compiled binary
  mzrun --reset                         # Reset emulator (PC=0, run ROM)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProgram,
}

func init() {
	rootCmd.Flags().StringVar(&host, "host", "localhost", "DZRP emulator host/IP")
	rootCmd.Flags().IntVar(&port, "port", 11000, "DZRP port")
	rootCmd.Flags().Uint16Var(&loadAddr, "load", 0x8000, "Load address")
	rootCmd.Flags().Uint16Var(&startAddr, "start", 0, "Start address (default: same as load)")
	rootCmd.Flags().IntVar(&timeout, "timeout", 10, "Execution timeout in seconds (0 = run forever)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVar(&step, "step", false, "Single-step execution")
	rootCmd.Flags().BoolVar(&reset, "reset", false, "Reset emulator (set PC=0x0000 and run)")
	rootCmd.Flags().Uint16Var(&dump, "dump", 0, "Dump N bytes from load address after execution")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Interactive step debugger")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runProgram(cmd *cobra.Command, args []string) error {
	var binaryData []byte
	var err error

	// Handle reset mode
	if reset {
		if len(args) > 0 {
			return fmt.Errorf("--reset does not take a file argument")
		}
		return doReset()
	}

	// Normal mode requires a file
	if len(args) == 0 {
		return fmt.Errorf("requires a minz file or binary (or use --reset)")
	}

	inputFile := args[0]

	// Check if it's a .minz file that needs compilation
	if len(inputFile) > 5 && inputFile[len(inputFile)-5:] == ".minz" {
		if verbose {
			fmt.Printf("Compiling %s...\n", inputFile)
		}
		binaryData, err = compileMinz(inputFile)
		if err != nil {
			return fmt.Errorf("compilation failed: %w", err)
		}
	} else {
		// Assume it's a binary file
		binaryData, err = os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read binary: %w", err)
		}
	}

	if startAddr == 0 {
		startAddr = loadAddr
	}

	if verbose {
		fmt.Printf("Binary size: %d bytes\n", len(binaryData))
		fmt.Printf("Load address: $%04X\n", loadAddr)
		fmt.Printf("Start address: $%04X\n", startAddr)
		fmt.Printf("Connecting to %s:%d...\n", host, port)
	}

	// Connect to DZRP emulator
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to DZRP emulator at %s: %w", addr, err)
	}
	defer conn.Close()

	if verbose {
		fmt.Println("Connected! Initializing...")
	}

	// Initialize DZRP session
	if err := dzrpInit(conn); err != nil {
		return fmt.Errorf("DZRP init failed: %w", err)
	}

	// Pause emulation
	if err := dzrpPause(conn); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	// Write program to memory
	if verbose {
		fmt.Printf("Loading program at $%04X...\n", loadAddr)
	}
	if err := dzrpWriteMem(conn, loadAddr, binaryData); err != nil {
		return fmt.Errorf("failed to write memory: %w", err)
	}

	// Set PC to start address
	if err := dzrpSetPC(conn, startAddr); err != nil {
		return fmt.Errorf("failed to set PC: %w", err)
	}

	if debug {
		// Ensure emulator is paused at start address before debugger
		if err := dzrpPause(conn); err != nil {
			return fmt.Errorf("failed to pause at start: %w", err)
		}
		// Interactive debugger mode
		return runDebugger(conn, startAddr)
	}

	if verbose {
		fmt.Println("Starting execution...")
	}

	// Continue execution
	if err := dzrpContinue(conn); err != nil {
		return fmt.Errorf("failed to continue: %w", err)
	}

	// timeout 0 means "run forever" - just exit and leave it running
	if timeout == 0 {
		fmt.Println("Program running (use emulator to stop)")
		return nil
	}

	// Wait for program to complete (or timeout)
	time.Sleep(time.Duration(timeout) * time.Second)

	// Pause and read final state
	if err := dzrpPause(conn); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	// Get final registers
	regs, err := dzrpGetRegisters(conn)
	if err != nil {
		return fmt.Errorf("failed to get registers: %w", err)
	}

	fmt.Println("\n=== Execution Complete ===")
	fmt.Printf("PC=$%04X SP=$%04X  I=$%02X R=$%02X IM=%d\n",
		regs["PC"], regs["SP"], regs["I"], regs["R"], regs["IM"])
	fmt.Printf("AF=$%04X BC=$%04X DE=$%04X HL=$%04X\n",
		regs["AF"], regs["BC"], regs["DE"], regs["HL"])
	fmt.Printf("AF'=%04X BC'=%04X DE'=%04X HL'=%04X\n",
		regs["AF'"], regs["BC'"], regs["DE'"], regs["HL'"])
	fmt.Printf("IX=$%04X IY=$%04X\n", regs["IX"], regs["IY"])

	// Dump memory if requested
	if dump > 0 {
		fmt.Printf("\n=== Memory at $%04X (%d bytes) ===\n", loadAddr, dump)
		mem, err := dzrpReadMem(conn, loadAddr, dump)
		if err != nil {
			return fmt.Errorf("failed to read memory: %w", err)
		}
		// Print hex dump
		for i := 0; i < len(mem); i += 16 {
			fmt.Printf("%04X: ", loadAddr+uint16(i))
			end := i + 16
			if end > len(mem) {
				end = len(mem)
			}
			for j := i; j < end; j++ {
				fmt.Printf("%02X ", mem[j])
			}
			fmt.Println()
		}
	}

	return nil
}

func doReset() error {
	if verbose {
		fmt.Printf("Connecting to %s:%d...\n", host, port)
	}

	// Connect to DZRP emulator
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to DZRP emulator at %s: %w", addr, err)
	}
	defer conn.Close()

	if verbose {
		fmt.Println("Connected! Initializing...")
	}

	// Initialize DZRP session
	if err := dzrpInit(conn); err != nil {
		return fmt.Errorf("DZRP init failed: %w", err)
	}

	// Pause emulation
	if err := dzrpPause(conn); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	// Set PC to 0x0000
	if verbose {
		fmt.Println("Setting PC to $0000...")
	}
	if err := dzrpSetPC(conn, 0x0000); err != nil {
		return fmt.Errorf("failed to set PC: %w", err)
	}

	// Continue execution
	if verbose {
		fmt.Println("Starting execution from ROM...")
	}
	if err := dzrpContinue(conn); err != nil {
		return fmt.Errorf("failed to continue: %w", err)
	}

	fmt.Println("Emulator reset - running from $0000")
	return nil
}

func compileMinz(inputFile string) ([]byte, error) {
	// Create temp output files
	tmpAsm := "/tmp/mzrun_output.a80"
	tmpBin := "/tmp/mzrun_output.bin"

	// Run minzc compiler to generate assembly
	cmd := exec.Command("./minzc/main", inputFile, "-o", tmpAsm)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("compiler error: %s\n%s", err, string(output))
	}

	// Run mza assembler to generate binary
	cmd = exec.Command("./mza", tmpAsm, "-o", tmpBin)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("assembler error: %s\n%s", err, string(output))
	}

	// Read the compiled binary
	data, err := os.ReadFile(tmpBin)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	return data, nil
}

// DZRP Protocol Functions

func dzrpSend(conn net.Conn, cmd byte, data []byte) error {
	// Format: [4-byte length LE][seqNum][cmdId][payload]
	length := uint32(2 + len(data)) // seqNum + cmdId + payload
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
		// Read 4-byte length (little-endian)
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return 0, nil, err
		}
		length := binary.LittleEndian.Uint32(lenBuf)

		if length < 2 {
			return 0, nil, fmt.Errorf("invalid message length: %d", length)
		}

		// Read seqNum + cmdId + payload
		msgBuf := make([]byte, length)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			return 0, nil, err
		}

		seqNum := msgBuf[0]
		cmdId := msgBuf[1]

		if verbose {
			if seqNum == 0 {
				fmt.Printf("  <- notification: cmd=%d len=%d (skipping)\n", cmdId, len(msgBuf)-2)
			} else {
				fmt.Printf("  <- response: seq=%d cmd=%d len=%d\n", seqNum, cmdId, len(msgBuf)-2)
			}
		}

		// Skip notifications (seq=0), wait for actual response
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

func dzrpReadMem(conn net.Conn, addr uint16, length uint16) ([]byte, error) {
	// DZRP READ_MEM format: [addr:2][length:2]
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], addr)
	binary.LittleEndian.PutUint16(payload[2:4], length)

	if err := dzrpSend(conn, CMD_READ_MEM, payload); err != nil {
		return nil, err
	}

	_, data, err := dzrpRecv(conn)
	if err != nil {
		return nil, err
	}

	// Response format: [error:1][data...]
	if len(data) < 1 {
		return nil, fmt.Errorf("empty response")
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("error code: %d", data[0])
	}

	return data[1:], nil
}

func dzrpWriteMem(conn net.Conn, addr uint16, data []byte) error {
	// Split into chunks (max 256 bytes per write for safety)
	for len(data) > 0 {
		chunkSize := 256
		if len(data) < chunkSize {
			chunkSize = len(data)
		}

		// DZRP WRITE_MEM format: [addr:2][length:2][data:N]
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

func dzrpGetRegisters(conn net.Conn) (map[string]uint16, error) {
	if err := dzrpSend(conn, CMD_GET_REGISTERS, nil); err != nil {
		return nil, err
	}

	_, data, err := dzrpRecv(conn)
	if err != nil {
		return nil, err
	}

	// Response format: [error:1][registers...]
	if len(data) < 1 {
		return nil, fmt.Errorf("empty response")
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("error code: %d", data[0])
	}

	regs := make(map[string]uint16)
	data = data[1:] // Skip error byte
	if len(data) >= 28 {
		// Main registers (16-bit)
		regs["PC"] = binary.LittleEndian.Uint16(data[0:2])
		regs["SP"] = binary.LittleEndian.Uint16(data[2:4])
		regs["AF"] = binary.LittleEndian.Uint16(data[4:6])
		regs["BC"] = binary.LittleEndian.Uint16(data[6:8])
		regs["DE"] = binary.LittleEndian.Uint16(data[8:10])
		regs["HL"] = binary.LittleEndian.Uint16(data[10:12])
		regs["IX"] = binary.LittleEndian.Uint16(data[12:14])
		regs["IY"] = binary.LittleEndian.Uint16(data[14:16])
		// Shadow registers (16-bit)
		regs["AF'"] = binary.LittleEndian.Uint16(data[16:18])
		regs["BC'"] = binary.LittleEndian.Uint16(data[18:20])
		regs["DE'"] = binary.LittleEndian.Uint16(data[20:22])
		regs["HL'"] = binary.LittleEndian.Uint16(data[22:24])
		// 8-bit registers
		regs["I"] = uint16(data[24])
		regs["R"] = uint16(data[25])
		regs["IM"] = uint16(data[26])
	}
	return regs, nil
}
