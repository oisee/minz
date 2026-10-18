package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
)

// Ordinary MZX application entry. This is a measured resident stack budget,
// not per-instruction stack certification and not a physical TR-DOS ROM ABI.
const zxApplicationEntry = `ORG 0x6000
__zx_app_entry:
 PUSH IX
 PUSH IY
 LD (__zx_app_caller_sp),SP
 LD A,I
 DI
 JP PO,__zx_app_di
 LD A,1
 JR __zx_app_save_iff
__zx_app_di:
 XOR A
__zx_app_save_iff:
 LD (__zx_app_iff),A
 LD SP,__zx_app_stack_top
 LD HL,0xa55a
 LD (__zx_app_stack_bottom),HL
 XOR A
 LD (_abap_error_code),A
 CALL main
__zx_app_exit:
 DI
 LD HL,(__zx_app_stack_bottom)
 LD DE,0xa55a
 OR A
 SBC HL,DE
 JR Z,__zx_app_restore
 LD A,9
 LD (_abap_error_code),A
__zx_app_restore:
 LD SP,(__zx_app_caller_sp)
 POP IY
 POP IX
 LD A,(__zx_app_iff)
 OR A
 LD A,(_abap_error_code)
 RET Z
 EI
 RET
`
const zxApplicationTail = `
__zx_app_caller_sp: DW 0
__zx_app_iff: DB 0
__zx_app_stack_bottom: DS 1024
__zx_app_stack_top:
__zx_app_image_end:
`

func validateZXApplicationImage(source string) error {
	a, e := z80asm.NewAssembler().AssembleString(source)
	if e != nil {
		return fmt.Errorf("ZX application assembly: %w", e)
	}
	if len(a.Errors) > 0 {
		return fmt.Errorf("ZX application assembly: %v", a.Errors)
	}
	if a.Origin != 0x6000 || len(a.Binary) > 0x9000 {
		return fmt.Errorf("ZX application exceeds 0x6000..0xF000 image budget (%d bytes)", len(a.Binary))
	}
	return nil
}
