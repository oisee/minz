package mir2

import (
	"fmt"

	"github.com/minz/minzc/pkg/z80asm"
)

// CPMApplicationStackBytes is the initial resident application stack budget.
// It is a workload budget, not guarded-runtime stack certification. Deployment
// still needs a TPA covering the image and a valid caller return path.
const CPMApplicationStackBytes = 1024

// The static application budget ends below MZE's 0xFE00 TPA ceiling. At entry,
// the actual CP/M vector at 6 must cover the complete image, and the caller's
// two-byte return word must fit within that TPA without overlapping the image.
// Reject before any application write or stack switch; RET preserves the caller.
const cpmApplicationEntry = `ORG 0x100
__cpm_app_entry:
    LD HL, (6)
    LD DE, __cpm_app_image_end
    OR A
    SBC HL, DE
    JR C, __cpm_app_reject
    LD HL, 2
    ADD HL, SP
    JR C, __cpm_app_reject
    LD DE, (6)
    OR A
    SBC HL, DE
    JR C, __cpm_app_check_return
    JR NZ, __cpm_app_reject
__cpm_app_check_return:
    LD HL, 0
    ADD HL, SP
    LD DE, __cpm_app_image_end
    OR A
    SBC HL, DE
    JR NC, __cpm_app_admit
    LD HL, 0
    ADD HL, SP
    LD DE, 0xFF
    OR A
    SBC HL, DE
    JR NC, __cpm_app_reject
__cpm_app_admit:
    LD (__cpm_app_saved_sp), SP
    LD SP, __cpm_app_stack_top
    CALL main
__cpm_app_exit:
    LD SP, (__cpm_app_saved_sp)
    RET
__cpm_app_reject:
    RET
`

func cpmApplicationTail() string {
	return fmt.Sprintf(`
; Ordinary CP/M application storage; no guarded-runtime certification.
__cpm_app_saved_sp:
    DW 0
__cpm_app_stack_canary:
    DB 0xA5
__cpm_app_stack:
    DS %d
__cpm_app_stack_top:
__cpm_app_image_end:
`, CPMApplicationStackBytes)
}

func validateCPMApplicationEntry(m *Module) error {
	main := m.FuncByName("main")
	if m.Target != 0 || main == nil || len(main.Contract.Params) != 0 || len(main.Contract.Returns) != 0 {
		return fmt.Errorf("CP/M application entry requires target cpm and void main without arguments")
	}
	return nil
}

func validateCPMApplicationImage(assembly string) error {
	a, err := z80asm.NewAssembler().AssembleString(assembly)
	if err != nil {
		return fmt.Errorf("CP/M application assembly: %w", err)
	}
	if len(a.Errors) > 0 {
		return fmt.Errorf("CP/M application assembly: %v", a.Errors)
	}
	// Check both the binary extent and the end symbol so address wrap or later
	// storage allocation cannot hide overflow.
	end := 0x100 + len(a.Binary)
	if a.Origin != 0x100 || end >= 0xFE00 || a.Symbols["__cpm_app_image_end"] != end {
		return fmt.Errorf("CP/M application image including %d-byte stack must fit below 0xFE00 (end 0x%x)", CPMApplicationStackBytes, end)
	}
	return nil
}

// Terminal halt uses the existing native A=status convention. Print a bounded
// diagnostic before returning to CP/M, whose RET convention has no process
// error status. Preserve A/flags and the program's stored ABAP fault byte.
const cpmApplicationFaultRuntime = `
__cpm_app_fault:
    PUSH AF
    LD HL, __cpm_app_fault_text
    LD B, __cpm_app_fault_text_end-__cpm_app_fault_text
__cpm_app_fault_text_loop:
    LD A, (HL)
    CALL __cpm_app_fault_putchar
    INC HL
    DJNZ __cpm_app_fault_text_loop
    POP AF
    PUSH AF
    RRCA
    RRCA
    RRCA
    RRCA
    AND 0x0F
    CALL __cpm_app_fault_hex
    POP AF
    PUSH AF
    AND 0x0F
    CALL __cpm_app_fault_hex
    LD A, 13
    CALL __cpm_app_fault_putchar
    LD A, 10
    CALL __cpm_app_fault_putchar
    POP AF
    JP __cpm_app_exit
__cpm_app_fault_hex:
    ADD A, 48
    CP 58
    JR C, __cpm_app_fault_putchar
    ADD A, 7
__cpm_app_fault_putchar:
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH IX
    PUSH IY
    LD E, A
    LD C, 2
    CALL 5
    POP IY
    POP IX
    POP HL
    POP DE
    POP BC
    POP AF
    RET
__cpm_app_fault_text:
    DB "ABAP runtime error 0x"
__cpm_app_fault_text_end:
`
