package mir2

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

const Z80RuntimeAgonZ80 Z80Runtime = "agon-z80"

// This is native ABI admission, not CP/M's guarded memory certification.
// The application owns a short stack; the pinned MOS owns the long stack.
func ValidateAgonZ80Stack(bytes int) error {
	if bytes != 0 && (bytes < 128 || bytes > 16384 || bytes%2 != 0) {
		return fmt.Errorf("agon-z80 runtime stack must be even and between 128 and 16384 bytes")
	}
	return nil
}

func ValidateRuntimeStack(runtime Z80Runtime, bytes int) error {
	if runtime == Z80RuntimeAgonZ80 {
		return ValidateAgonZ80Stack(bytes)
	}
	return ValidateCPMRuntimeStack(runtime, bytes)
}

func validateAgonZ80Runtime(m *Module) error {
	main := m.FuncByName("main")
	if m.Target != 2 || main == nil || len(main.Contract.Params) != 0 || len(main.Contract.Returns) != 0 {
		return fmt.Errorf("agon-z80 requires platform Agon and void main without parameters")
	}
	for _, g := range m.Globals {
		if strings.HasPrefix(g.Name, "__agon_") || g.Storage != StorageNormal || g.At != nil || len(g.VtableSyms) != 0 {
			return fmt.Errorf("agon-z80 rejects reserved/fixed-address/SMC global %s", g.Name)
		}
	}
	for _, f := range m.Funcs {
		if strings.HasPrefix(f.Name, "__agon_") || f.Attrs.HasAsm || f.Attrs.IsExtern || f.Attrs.ExternAddr != 0 || f.Attrs.IsRecursive || f.Attrs.FallibleCY || f.Attrs.FixedIndirectABI {
			return fmt.Errorf("agon-z80 rejects opaque ASM, externs, recursion or reserved function %s", f.Name)
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case OpAsm, OpAlloca, OpCallIndirect, OpIn8, OpIn16, OpOut8, OpOut16, OpPatchSlot, OpLoadPatched, OpPatch, OpPush, OpPop:
					return fmt.Errorf("agon-z80 rejects unsupported native operation %s in %s", i.Op, f.Name)
				case OpCall, OpCallCond:
					switch i.Sym {
					case "@mir.io.halt", "@mir.io.print.char", "@mir.io.print.str", "@mir.io.print.sstr", "@mir.io.print.lstr", "@mir.io.print.nl", "@mir.io.print.u8", "@mir.io.print.dec", "console_log":
					default:
						if m.FuncByName(i.Sym) == nil {
							return fmt.Errorf("agon-z80 rejects unresolved call %s", i.Sym)
						}
					}
				}
			}
		}
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("agon-z80 rejects recursive native call graph at %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		if f := m.FuncByName(name); f != nil {
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if (i.Op == OpCall || i.Op == OpCallCond) && m.FuncByName(i.Sym) != nil {
						if err := visit(i.Sym); err != nil {
							return err
						}
					}
				}
			}
		}
		state[name] = 2
		return nil
	}
	for _, f := range m.Funcs {
		if err := visit(f.Name); err != nil {
			return err
		}
	}
	return nil
}

const agonZ80Entry = `
    .ASSUME ADL=0
    ORG $0045
__agon_entry:
    LD A, MB
    CP 4
    JP NZ, __agon_bad_entry
    LD (__agon_saved_sps), SP
    LD.LIL ($040000+__agon_saved_spl), SP
    LD HL, (__agon_saved_sps)
    LD A, H
    CP $F0
    JP C, __agon_bad_entry
    LD A, H
    AND L
    CP $FF
    JP Z, __agon_bad_entry
    LD A, (__agon_saved_spl+2)
    CP $0B
    JP NZ, __agon_bad_entry
    LD A, (__agon_saved_spl+1)
    CP $F2
    JP C, __agon_bad_entry
    PUSH.LIL IX
    PUSH.LIL IY
    LD.LIL ($040000+__agon_frame_spl), SP
    LD SP, __agon_stack_top
    XOR A
    LD (__agon_status), A
    CALL __agon_stack_check
    CALL main
__agon_completed:
    JP __agon_cleanup
__agon_bad_entry:
    LD.LIL HL, 3
    RET.LIS
`

func agonZ80Tail(stack int) string {
	if stack == 0 {
		stack = 1024
	}
	return `
; Pinned MOS rst10/UART0 changes AF; BC is saved by firmware, other
; primary/alternate registers are not used. Preserve AF for inline print ABI.
__agon_putchar:
    PUSH AF
    RST.LIS $10
    JP NC, __agon_console_failure
    POP AF
    RET
__agon_console_failure:
    LD A, 5
    JP __agon_fail
; Reserve 64 short bytes for this check, next growth, MOS mixed entry and
; asynchronous interrupt frames. Firmware long-stack allowance is separate.
__agon_stack_check:
    PUSH AF
    PUSH HL
    PUSH DE
    LD HL, 0
    ADD HL, SP
    LD DE, __agon_stack_bottom+64
    OR A
    SBC HL, DE
    JP C, __agon_stack_failure
    POP DE
    POP HL
    POP AF
    RET
__agon_stack_failure:
    LD A, 2
__agon_fail:
    LD (__agon_status), A
    JP __agon_cleanup
__agon_cleanup:
    LD SP, (__agon_saved_sps)
    LD.LIL SP, ($040000+__agon_frame_spl)
    LD HL, __agon_stack_guard
    LD B, 64
__agon_guard_loop:
    LD A, (HL)
    CP $A5
    JR NZ, __agon_guard_failure
    INC HL
    DJNZ __agon_guard_loop
    JR __agon_restore
__agon_guard_failure:
    LD A, 4
    LD (__agon_status), A
__agon_restore:
    POP.LIL IY
    POP.LIL IX
    LD.LIL HL, 0
    LD A, (__agon_status)
    LD L, A
__agon_return:
    RET.LIS
__agon_status: DB 0
__agon_saved_sps: DW 0
__agon_saved_spl: DS 3
__agon_frame_spl: DS 3
__agon_stack_guard: DS 64, $A5
__agon_stack_bottom:
    DS ` + fmt.Sprint(stack) + `, 0
__agon_stack_top:
__agon_image_end:
`
}

func validateAgonZ80Image(assembly string) error {
	a := z80asm.NewAssembler()
	if err := a.SetTarget(z80asm.TargetAgonZ80); err != nil {
		return err
	}
	r, err := a.AssembleString(assembly)
	if err != nil {
		return fmt.Errorf("agon-z80 runtime assembly: %w", err)
	}
	if len(r.Errors) != 0 {
		return fmt.Errorf("agon-z80 runtime assembly: %v", r.Errors)
	}
	if _, err := z80asm.GetTargetConfig(z80asm.TargetAgonZ80).OutputFormat.Generator(r); err != nil {
		return err
	}
	return nil
}
