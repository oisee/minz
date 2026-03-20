; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src/misc.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Dean Belfield
; Description: AGON MOS miscellaneous helper functions.
;              Key eZ80 features:
;              - SET_AHL24/SET_ADE24: Manipulate U byte (bits 16-23) of registers
;              - Self-modifying code for exec16/exec24 (JP/CALL.IS patching)
;              - Mixed ADL/Z80 mode execution
;              - IN0/OUT0 for timer access
;              - MBASE register save/restore across mode switches

			.ASSUME	ADL = 1

			DEFINE .STARTUP, SPACE = ROM
			SEGMENT .STARTUP

			XDEF	SWITCH_A
			XDEF	SET_AHL24
			XDEF	GET_AHL24
			XDEF	SET_ADE24
			XDEF	_exec16
			XDEF	_exec24

			XREF	_callSM

; Switch on A - lookup table immediately after call
;  A: Index into lookup table
;
SWITCH_A:		EX	(SP), HL
			ADD	A, A
			ADD8U_HL
			LD	A, (HL)
			INC	HL
			LD	H, (HL)
			LD	L, A
			EX	(SP), HL
			RET

; Set the MSB of HL (U byte) to A
; This is the key eZ80 idiom for constructing 24-bit addresses
;
SET_AHL24:		PUSH	HL
			LD	HL, 2
			ADD	HL, SP
			LD	(HL), A
			POP	HL
			RET

; Get the MSB of HL (U byte) in A
;
GET_AHL24:		PUSH	HL
			LD	HL, 2
			ADD	HL, SP
			LD	A, (HL)
			POP	HL
			RET

; Set the MSB of DE (U byte) to A
;
SET_ADE24:		EX	DE, HL
			PUSH	HL
			LD	HL, 2
			ADD	HL, SP
			LD	(HL), A
			POP	HL
			EX	DE, HL
			RET

; Execute a 24-bit program (stays in ADL mode)
; Self-modifying code: writes "JP addr" to RAM and calls it
;
_exec24:		PUSH 	IY
			LD	IY, 0
			ADD	IY, SP
			PUSH 	AF
			PUSH	DE
			PUSH	IX
			LD	A, MB		; Save MBASE
			PUSH	AF
			LD	DE, (IY+6)	; 24-bit address
			LD	A, (IY+8)	; High byte for code segment
			LD	HL, (IY+9)	; Params pointer
			; Write "JP (DE)" to RAM
			LD	IX, _callSM
			LD	(IX + 0), 0C3h	; JP llhhuu
			LD	(IX + 1), E
			LD	(IX + 2), D
			LD	(IX + 3), A
			JR	_execSM

; Execute a 16-bit program (switches to Z80 mode ADL=0)
; Self-modifying code: writes "CALL.IS addr; RET" to RAM
;
_exec16:		PUSH 	IY
			LD	IY, 0
			ADD	IY, SP
			PUSH 	AF
			PUSH	DE
			PUSH	IX
			LD	A, MB		; Save MBASE
			PUSH	AF
			LD	DE, (IY+6)
			LD	A, (IY+8)
			LD	MB, A		; Set MBASE for Z80 mode segment
			LD	HL, (IY+9)
			; Write "CALL.IS addr; RET" to RAM
			LD	IX, _callSM
			LD	(IX + 0), 49h	; CALL.IS prefix
			LD	(IX + 1), 0CDh	; CALL
			LD	(IX + 2), E
			LD	(IX + 3), D
			LD	(IX + 4), 0C9h	; RET

_execSM:		CALL	_callSM
			POP	AF
			LD	MB, A		; Restore MBASE
			POP	IX
			POP	DE
			POP 	AF
			LD	SP, IY
			POP	IY
			RET

; Wait for timer0 to hit 0 - demonstrates IN0 (eZ80 direct port read)
;
_wait_timer0:		PUSH	AF
			PUSH	BC
			IN0	A, (TMR0_CTL)	; eZ80: read timer control register
			OR	3
			OUT0	(TMR0_CTL), A	; eZ80: write timer control register
$$:			IN0	B, (TMR0_DR_L)
			IN0 	A, (TMR0_DR_H)
			OR	B
			JR	NZ, $B
			POP	BC
			POP	AF
			RET
