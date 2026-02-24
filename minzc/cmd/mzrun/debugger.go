package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/disasm"
)

func runDebugger(conn net.Conn, startPC uint16) error {
	reader := bufio.NewReader(os.Stdin)
	pc := startPC

	fmt.Println("\n=== MinZ Step Debugger ===")
	fmt.Println("Commands: s=step, o=step-over, c=continue, r=regs, m=mem, q=quit")
	fmt.Println()

	for {
		// Read memory at PC for disassembly
		mem, err := dzrpReadMem(conn, pc, 4)
		if err != nil {
			return fmt.Errorf("failed to read memory: %w", err)
		}

		instr, _ := disasm.Disasm(mem, pc)
		fmt.Printf("$%04X: %-20s > ", pc, instr)

		// Read command
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "", "s", "step":
			// Single step using CMD_STEP_INTO (properly follows JP/JR/CALL/RET)
			if err := dzrpStepInto(conn); err != nil {
				return err
			}
			// Read new PC
			regs, err := dzrpGetRegisters(conn)
			if err != nil {
				return err
			}
			pc = regs["PC"]

		case "o", "over":
			// Step over - if CALL, set breakpoint after it; otherwise single step
			if mem[0] == 0xCD || // CALL nn
				(mem[0] == 0xC4) || (mem[0] == 0xCC) || (mem[0] == 0xD4) || (mem[0] == 0xDC) || // CALL cc,nn
				(mem[0] == 0xE4) || (mem[0] == 0xEC) || (mem[0] == 0xF4) || (mem[0] == 0xFC) {
				nextPC := pc + 3
				fmt.Printf("  [step over CALL, break at $%04X]\n", nextPC)
				if err := dzrpStepTo(conn, nextPC); err != nil {
					// CALL might not return, check PC
					regs, _ := dzrpGetRegisters(conn)
					if regs != nil {
						pc = regs["PC"]
						fmt.Printf("  [stopped at $%04X]\n", pc)
					}
					continue
				}
			} else {
				// Normal step - use CMD_STEP_INTO for proper JP/JR handling
				if err := dzrpStepInto(conn); err != nil {
					return err
				}
			}
			regs, err := dzrpGetRegisters(conn)
			if err != nil {
				return err
			}
			pc = regs["PC"]

		case "c", "continue":
			fmt.Println("Continuing...")
			if err := dzrpContinue(conn); err != nil {
				return err
			}
			return nil

		case "r", "regs":
			regs, err := dzrpGetRegisters(conn)
			if err != nil {
				return err
			}
			fmt.Printf("PC=$%04X SP=$%04X\n", regs["PC"], regs["SP"])
			fmt.Printf("AF=$%04X BC=$%04X DE=$%04X HL=$%04X\n",
				regs["AF"], regs["BC"], regs["DE"], regs["HL"])
			fmt.Printf("IX=$%04X IY=$%04X\n", regs["IX"], regs["IY"])
			pc = regs["PC"]

		case "m", "mem":
			fmt.Printf("Memory at $%04X:\n", pc)
			mem, _ := dzrpReadMem(conn, pc, 32)
			for i := 0; i < len(mem); i += 16 {
				fmt.Printf("%04X: ", pc+uint16(i))
				end := i + 16
				if end > len(mem) {
					end = len(mem)
				}
				for j := i; j < end; j++ {
					fmt.Printf("%02X ", mem[j])
				}
				fmt.Println()
			}

		case "q", "quit":
			fmt.Println("Quitting debugger...")
			return nil

		default:
			fmt.Println("Unknown command. s=step, o=over, c=continue, r=regs, m=mem, q=quit")
		}
	}
}

func dzrpStepTo(conn net.Conn, targetPC uint16) error {
	// Use CMD_CONTINUE with temporary breakpoint
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], 1) // 1 temp breakpoint
	binary.LittleEndian.PutUint16(payload[2:4], targetPC)

	if err := dzrpSend(conn, CMD_CONTINUE, payload); err != nil {
		return err
	}

	// Wait for pause notification - use raw recv that doesn't skip notifications
	for {
		seqNum, cmd, _, err := dzrpRecvRaw(conn)
		if err != nil {
			return err
		}

		// Check if it's a pause notification (seq=0, cmd=1)
		if seqNum == 0 && cmd == 1 { // NTF_PAUSE
			return nil
		}

		// CMD_CONTINUE response (seq != 0), keep waiting for notification
		if cmd == CMD_CONTINUE {
			continue
		}
	}
}

// dzrpStepInto executes exactly one instruction using CMD_STEP_INTO
func dzrpStepInto(conn net.Conn) error {
	if err := dzrpSend(conn, CMD_STEP_INTO, nil); err != nil {
		return err
	}

	// Wait for pause notification
	for {
		seqNum, cmd, _, err := dzrpRecvRaw(conn)
		if err != nil {
			return err
		}

		// Check if it's a pause notification (seq=0, cmd=1)
		if seqNum == 0 && cmd == 1 { // NTF_PAUSE
			return nil
		}

		// CMD_STEP_INTO response (seq != 0), keep waiting for notification
		if cmd == CMD_STEP_INTO {
			continue
		}
	}
}

// dzrpRecvRaw reads a message without skipping notifications
func dzrpRecvRaw(conn net.Conn) (byte, byte, []byte, error) {
	// Read 4-byte length (little-endian)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return 0, 0, nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf)

	if length < 2 {
		return 0, 0, nil, fmt.Errorf("invalid message length: %d", length)
	}

	// Read seqNum + cmdId + payload
	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return 0, 0, nil, err
	}

	return msgBuf[0], msgBuf[1], msgBuf[2:], nil
}
