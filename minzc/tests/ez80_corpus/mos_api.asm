; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src/mos_api.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Dean Belfield
; Description: AGON MOS API code - production eZ80 firmware.
;              Demonstrates real-world ADL mode programming patterns:
;              - .ASSUME ADL=1 directive
;              - 24-bit addressing with HLU, DEU, BCU registers
;              - MBASE register manipulation (LD A, MB)
;              - Mixed ADL/Z80 mode support via MB check
;              - XDEF/XREF linker symbols
;              - DEFINE SEGMENT directives
;              - C calling convention (push params, CALL, pop)
;              - Jump tables with DW (SWITCH_A pattern)

			.ASSUME	ADL = 1

			DEFINE .STARTUP, SPACE = ROM
			SEGMENT .STARTUP

			XDEF	mos_api

			XREF	SWITCH_A
			XREF	SET_AHL24
			XREF	GET_AHL24
			XREF	SET_ADE24

			XREF	_mos_LOAD
			XREF	_mos_SAVE
			XREF	_mos_CD
			XREF	_mos_DIR
			XREF	_mos_DEL
			XREF	_mos_REN
			XREF	_mos_FOPEN
			XREF	_mos_FCLOSE
			XREF	_mos_FGETC
			XREF	_mos_FPUTC
			XREF	_mos_FEOF

			XREF	_keyascii
			XREF	_keycount
			XREF	_keydown
			XREF	_sysvars
			XREF	_keymap

; Call a MOS API function
;  A: function to call
;
mos_api:		CP	80h
			JR	NC, $F
			CP	mos_api_block1_size
			RET	NC
			CALL	SWITCH_A
;
mos_api_block1_start:	DW	mos_api_getkey		; 0x00
			DW	mos_api_load		; 0x01
			DW	mos_api_save		; 0x02
			DW	mos_api_cd		; 0x03
			DW	mos_api_dir		; 0x04
			DW	mos_api_del		; 0x05
			DW	mos_api_ren		; 0x06

mos_api_block1_size:	EQU 	($ - mos_api_block1_start) / 2

; Get keycode
;
mos_api_getkey:		PUSH	HL
			LD	HL, _keycount
mos_api_getkey_1:	LD	A, (HL)
$$:			CP	(HL)
			JR	Z, $B
			LD	A, (_keydown)
			OR	A
			JR	Z, mos_api_getkey_1
			POP	HL
			LD	A, (_keyascii)
			RET

; Load file - demonstrates MBASE handling for mixed-mode support
;
mos_api_load:		LD	A, MB		; Check if MBASE is 0
			OR	A, A
			JR	Z, $F		; If 0, addresses are already 24-bit
			CALL	SET_AHL24	; Convert HL to 24-bit using MB
			CALL	SET_ADE24	; Convert DE to 24-bit using MB
$$:			PUSH	BC		; size
			PUSH	DE		; address
			PUSH	HL		; filename
			CALL	_mos_LOAD
			LD	A, L		; Return value in HLU -> A
			POP	HL
			POP	DE
			POP	BC
			SCF
			RET

; Save file
;
mos_api_save:		LD	A, MB
			OR	A, A
			JR	Z, $F
			CALL	SET_AHL24
			CALL	SET_ADE24
$$:			PUSH	BC
			PUSH	DE
			PUSH	HL
			CALL	_mos_SAVE
			LD	A, L
			POP	HL
			POP	DE
			POP	BC
			SCF
			RET

; Change directory
;
mos_api_cd:		LD	A, MB
			OR	A, A
			CALL	NZ, SET_AHL24
			PUSH	HL
			CALL	_mos_CD
			LD	A, L
			POP	HL
			RET

; Directory listing
;
mos_api_dir:		LD	A, MB
			OR	A, A
			CALL	NZ, SET_AHL24
			PUSH	HL
			CALL	_mos_DIR
			LD	A, L
			POP	HL
			RET

; Delete file
;
mos_api_del:		LD	A, MB
			OR	A, A
			CALL	NZ, SET_AHL24
			PUSH	HL
			CALL	_mos_DEL
			LD	A, L
			POP	HL
			RET

; Rename file
;
mos_api_ren:		LD	A, MB
			OR	A, A
			JR	Z, $F
			CALL	SET_AHL24
			CALL	SET_ADE24
$$:			PUSH	DE
			PUSH	HL
			CALL	_mos_REN
			LD	A, L
			POP	HL
			POP	DE
			RET

; Get sysvars pointer
;
mos_api_sysvars:	LD	IX, _sysvars
			RET

; Open file
;
mos_api_fopen:		PUSH	BC
			PUSH	DE
			PUSH	HL
			PUSH	IX
			PUSH	IY
			LD	A, MB
			OR	A, A
			CALL	NZ, SET_AHL24
			LD	A, C
			LD	BC, 0
			LD	C, A
			PUSH	BC		; mode
			PUSH	HL		; filename
			CALL	_mos_FOPEN
			LD	A, L
			POP	HL
			POP	BC
			POP	IY
			POP	IX
			POP	HL
			POP	DE
			POP	BC
			RET

; Get keyboard map
;
mos_api_getkbmap:	LD	IX, _keymap
			RET

; UART open - demonstrates LEA instruction
;
mos_api_uopen:		LEA	HL, IX + 0
			LD	A, MB
			OR	A, A
			CALL	NZ, SET_AHL24
			PUSH	HL
			; ... (calls _open_UART1)
			LD	A, L
			POP	HL
			RET
