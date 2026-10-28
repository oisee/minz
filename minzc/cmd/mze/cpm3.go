package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/spf13/cobra"
)

// CP/M 3 program return codes (BDOS 108, P_CODE) and chaining (BDOS 47,
// P_CHAIN), plus a CCP-like SUBMIT runner. Opt-in through `mze submit`; the
// plain CP/M 2.2 target never sees these functions.
//
// Semantics follow the CP/M 3 BDOS reference (seasip.info/Cpm/bdos.html):
//   - P_CODE: DE=0FFFFh reads the code into HL, any other DE sets it.
//     0000h-0FEFFh: no fatal error; 0FF00h-0FF7Fh: fatal error;
//     0FFFDh: terminated by a hardware error; 0FFFEh: Control-C.
//   - The CCP resets the code to 0 before each program it runs, unless the
//     program was started by P_CHAIN, so a chained program sees its caller's.
//   - A command starting with ':' is skipped when the code is 0FF00h or above.
//   - P_CHAIN runs the command line at 80h and only returns on failure.
const (
	cpm3FatalLow     = 0xFF00
	cpm3HardwareFail = 0xFFFD
)

type cpm3State struct {
	code  uint16
	chain string
}

// cpm3 is non-nil only inside `mze submit`.
var cpm3 *cpm3State

// cpm3BDOS handles the CP/M 3 functions when the SUBMIT runner is active.
func cpm3BDOS(z80 *emulator.RemogattoZ80WithScreen, function byte, de uint16) (byte, uint16, bool) {
	if cpm3 == nil {
		return 0, 0, false
	}
	switch function {
	case 0x6C: // P_CODE
		if de == 0xFFFF {
			return byte(cpm3.code), cpm3.code, true
		}
		cpm3.code = de
		return 0, de, true
	case 0x2F: // P_CHAIN: command line at 80h, terminated by NUL
		var line []byte
		for at := uint16(0x80); at < 0x100; at++ {
			b := z80.ReadMemory(at)
			if b == 0 {
				break
			}
			line = append(line, b)
		}
		cpm3.chain = strings.TrimSpace(string(line))
		if cpm3.chain == "" {
			return 0xFF, 0xFFFF, true // nothing to chain: return failure
		}
		// Terminate the caller: its BDOS return goes to warm boot (0000h, HALT).
		sp := z80.GetRegisters().SP
		z80.WriteMemory(sp, 0x00)
		z80.WriteMemory(sp+1, 0x00)
		return 0, 0, true
	}
	return 0, 0, false
}

// trapCPM3BDOS serves P_CODE and P_CHAIN before the CP/M 2.2 handler in
// main.go sees them. The emulator checks its T-state trap after every
// instruction and its BDOS intercept before the next one, so a trap re-armed
// one T-state ahead sees each CALL 5 first and returns like the intercept.
// Keeping the hook here leaves main.go, which frozen Autosiril evidence pins,
// unchanged.
func trapCPM3BDOS(z80 *emulator.RemogattoZ80WithScreen) {
	cpu := z80.RemogattoZ80
	var trap func(int64)
	trap = func(now int64) {
		defer cpu.SetTStateTrap(now+1, trap)
		r := cpu.GetRegisters()
		if r.PC != 0x0005 {
			return
		}
		a, hl, ok := cpm3BDOS(z80, byte(r.BC), r.DE)
		if !ok {
			return
		}
		r.A, r.HL = a, hl
		ret := uint16(z80.ReadMemory(r.SP+1))<<8 | uint16(z80.ReadMemory(r.SP))
		r.SP += 2
		r.PC = ret
		cpu.SetRegisters(r)
	}
	cpu.SetTStateTrap(1, trap)
}

// runCPM3Command loads NAME.COM from dir and runs it with the given tail,
// like the CCP: the command tail goes to 80h as a length byte plus text.
func runCPM3Command(dir, command string, resetCode bool) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}
	name := strings.ToUpper(fields[0])
	if !strings.Contains(name, ".") {
		name += ".COM"
	}
	canonical, ok := cpmName83(name)
	if !ok {
		return fmt.Errorf("invalid program name %q", fields[0])
	}
	entries, err := cpmDirectoryEntries(dir, canonical)
	if err != nil || len(entries) != 1 {
		return fmt.Errorf("program %s not found in %s", name, dir)
	}
	binary, err := os.ReadFile(entries[0].path)
	if err != nil {
		return err
	}
	if resetCode {
		cpm3.code = 0
	}
	z80 := emulator.NewRemogattoZ80WithScreen()
	setExecutionLimit(z80.RemogattoZ80, timeout)
	setupCPMBDOS(z80)
	trapCPM3BDOS(z80)
	tail := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), fields[0])))
	if tail != "" {
		tail = " " + tail
	}
	if len(tail) > 126 {
		return fmt.Errorf("command tail too long")
	}
	z80.WriteMemory(0x80, byte(len(tail)))
	for i := 0; i < len(tail); i++ {
		z80.WriteMemory(uint16(0x81+i), tail[i])
	}
	z80.WriteMemory(uint16(0x81+len(tail)), 0)
	z80.LoadAt(0x0100, binary)
	z80.SetPC(0x0100)
	sp := uint16(0xFE00) - 2
	z80.WriteMemory(sp, 0x00)
	z80.WriteMemory(sp+1, 0x00)
	z80.SetSP(sp)
	if err := z80.Execute(); err != nil {
		cpm3.code = cpm3HardwareFail
		fmt.Printf("%s: execution error: %v\n", name, err)
	}
	if cycles {
		fmt.Printf("%s: %d T-states\n", name, z80.GetCycles())
	}
	fmt.Printf("%s: return code %04Xh\n", name, cpm3.code)
	return nil
}

// runSubmit executes a SUBMIT job: one command per line, ';' comments,
// ':' conditional lines, and P_CHAIN continuations.
func runSubmit(path string) (code uint16, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }() // read-only
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return 0, err
	}
	// setupCPMBDOS serves files from the current directory when no .COM is
	// named on the command line, so run the job from its own directory.
	cwd, err := os.Getwd()
	if err != nil {
		return 0, err
	}
	if err := os.Chdir(dir); err != nil {
		return 0, err
	}
	defer func() {
		if e := os.Chdir(cwd); e != nil && err == nil {
			err = e
		}
	}()
	cpm3 = &cpm3State{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, ":") {
			line = strings.TrimSpace(line[1:])
			if cpm3.code >= cpm3FatalLow {
				fmt.Printf("skipped (return code %04Xh): %s\n", cpm3.code, line)
				continue
			}
		}
		if err := runCPM3Command(dir, line, true); err != nil {
			return cpm3.code, err
		}
		for cpm3.chain != "" {
			next := cpm3.chain
			cpm3.chain = ""
			if err := runCPM3Command(dir, next, false); err != nil {
				return cpm3.code, err
			}
		}
	}
	return cpm3.code, scanner.Err()
}

var submitCmd = &cobra.Command{
	Use:   "submit JOB.SUB",
	Short: "Run a CP/M 3 SUBMIT job (program return codes, ':' conditionals, P_CHAIN)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		code, err := runSubmit(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "submit: %v\n", err)
			os.Exit(2)
		}
		if code >= cpm3FatalLow {
			os.Exit(1)
		}
	},
}

func init() {
	submitCmd.Flags().UintVar(&timeout, "timeout", 0, "per-program execution limit in T-states (0 = none)")
	submitCmd.Flags().BoolVar(&cycles, "cycles", false, "print T-states per program")
	rootCmd.AddCommand(submitCmd)
}
