package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/spf13/cobra"
)

// Banked RAM flags live here, not in main.go: main.go is pinned by the frozen
// Autosiril file gate (research/autosiril-native/files/evidence/receipt.json).
var (
	modelName string
	bankLoads []string
)

const pagingHelp = `

BANKED RAM (--model 128k | pentagon | scorpion-memory):
  128k/pentagon    8 x 16K banks, port $7FFD bits 0-2 select the bank at $C000,
                   bit 5 locks the map; bank 5 sits at $4000, bank 2 at $8000
  scorpion-memory  16 banks: port $1FFD bit 4 adds 8 to the bank number
  --load-bank FILE@BANK[:OFFSET]  preload a bank before the run (repeatable)
  RAM map only: no ROM switching, no ULA contention; T-states stay uncontended.`

func init() {
	rootCmd.Flags().StringVar(&modelName, "model", "48k", "memory model: 48k (flat 64K), 128k, pentagon, scorpion-memory (256K)")
	rootCmd.Flags().StringArrayVar(&bankLoads, "load-bank", nil, "preload a RAM bank: FILE@BANK or FILE@BANK:OFFSET (hex offset, repeatable)")
	rootCmd.Long += pagingHelp

	// The machine is created inside the root command; the hook maps and
	// preloads the banks right after creation, before the binary is loaded.
	run := rootCmd.Run
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		emulator.NewMachineHook = func(cpu *emulator.RemogattoZ80) {
			if err := setupPaging(cpu, modelName, bankLoads); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
		run(cmd, args)
	}
}

// setupPaging enables the banked memory model and applies --load-bank specs.
func setupPaging(cpu *emulator.RemogattoZ80, model string, loads []string) error {
	pm, err := emulator.ParsePagingModel(model)
	if err != nil {
		return err
	}
	cpu.EnablePaging(pm)
	for _, spec := range loads {
		file, bank, offset, err := parseBankLoad(spec)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("--load-bank: %w", err)
		}
		if err := cpu.LoadBank(bank, offset, data); err != nil {
			return fmt.Errorf("--load-bank %s: %w", spec, err)
		}
	}
	return nil
}

// parseBankLoad parses FILE@BANK or FILE@BANK:OFFSET (decimal bank, hex offset
// within the 16K bank; a full address such as C000 is reduced to its offset).
func parseBankLoad(spec string) (file string, bank int, offset uint16, err error) {
	at := strings.LastIndex(spec, "@")
	if at <= 0 || at == len(spec)-1 {
		return "", 0, 0, fmt.Errorf("--load-bank: expected FILE@BANK[:OFFSET], got %q", spec)
	}
	file, rest := spec[:at], spec[at+1:]
	bankStr, offStr, hasOff := strings.Cut(rest, ":")
	b, perr := strconv.ParseUint(bankStr, 10, 8)
	if perr != nil {
		return "", 0, 0, fmt.Errorf("--load-bank: bad bank %q in %q", bankStr, spec)
	}
	if hasOff {
		offStr = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(offStr, "$"), "0x"), "0X")
		o, perr := strconv.ParseUint(offStr, 16, 16)
		if perr != nil {
			return "", 0, 0, fmt.Errorf("--load-bank: bad offset %q in %q", offStr, spec)
		}
		offset = uint16(o) & 0x3FFF
	}
	return file, int(b), offset, nil
}
