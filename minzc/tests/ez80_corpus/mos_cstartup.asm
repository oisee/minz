; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src_startup/cstartup.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Copyright (C) 2005 by ZiLOG, Inc. Modified by Dean Belfield
; Description: eZ80 C runtime startup code.
;              Clears BSS, copies initialized data from ROM to RAM.
;              Demonstrates 24-bit addressing, LDIR for block copy.

			XDEF __c_startup
			XDEF __cstartup
			XREF _main
			XREF __low_bss
			XREF __len_bss
			XREF __low_data
			XREF __low_romdata
			XREF __len_data
			XREF __copy_code_to_ram
			XREF __len_code
			XREF __low_code
			XREF __low_romcode

__cstartup		EQU %1

			DEFINE .STARTUP, SPACE = ROM
			SEGMENT .STARTUP
			.ASSUME ADL=1

__c_startup:
;
; Clear BSS section
;
			LD	BC, __len_bss
			LD	a, __len_bss >> 16	; Check 24-bit length
			OR	A, C
			OR	A, B
			JR	Z, _c_bss_done
			XOR	A, A
			LD 	(__low_bss), A
			SBC	HL, HL
			DEC	BC
			SBC	HL, BC
			JR	Z, _c_bss_done
			LD	HL, __low_bss
			LD	DE, __low_bss + 1
			LDIR

_c_bss_done:
;
; Copy initialized data from ROM to RAM
;
			LD	BC, __len_data
			LD	A, __len_data >> 16
			OR	A, C
			OR	A, B
			JR	Z, _c_data_done
			LD	HL, __low_romdata
			LD	DE, __low_data
			LDIR

_c_data_done:
;
; Copy code from FLASH to RAM if needed
;
			LD	A, __copy_code_to_ram
			OR	A, A
			JR	Z, _copy_code_to_ram_done
			LD	BC, __len_code
			LD	A, __len_code >> 16
			OR	A, C
			OR	A, B
			JR	Z, _copy_code_to_ram_done
			LD	HL, __low_romcode
			LD	DE, __low_code
			LDIR

_copy_code_to_ram_done:
			RET

			END
