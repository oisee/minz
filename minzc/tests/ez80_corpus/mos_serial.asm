; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src/serial.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Dean Belfield
; Description: AGON MOS UART serial code - production eZ80 firmware.
;              Demonstrates IN0/OUT0, TST instruction, hardware flow control.

			.ASSUME	ADL = 1

			DEFINE .STARTUP, SPACE = ROM
			SEGMENT .STARTUP

			XDEF	UART0_serial_TX
			XDEF	UART0_serial_RX
			XDEF	UART0_serial_GETCH
			XDEF	UART0_serial_PUTCH
			XDEF	UART1_serial_TX
			XDEF	UART1_serial_RX
			XDEF	_putch
			XDEF	_getch

			XREF	_serialFlags

UART0_PORT		EQU	%C0
UART1_PORT		EQU	%D0

UART0_REG_RBR:		EQU	UART0_PORT+0
UART0_REG_THR:		EQU	UART0_PORT+0
UART0_REG_LSR:		EQU	UART0_PORT+5

TX_WAIT			EQU	16384
UART_LSR_ETX		EQU 	%40
UART_LSR_RDY		EQU	%01

; Write a character to UART0
;
UART0_serial_TX:	PUSH		BC
			PUSH		AF
			LD		BC,TX_WAIT
UART0_serial_TX1:	IN0		A,(UART0_REG_LSR)	; eZ80: IN0 direct port
			AND 		UART_LSR_ETX
			JR		NZ, UART0_serial_TX2
			DEC		BC
			LD		A, B
			OR		C
			JR		NZ, UART0_serial_TX1
			POP		AF
			POP		BC
			OR		A		; Clear carry
			RET
UART0_serial_TX2:	POP		AF
			OUT0		(UART0_REG_THR),A	; eZ80: OUT0 direct port
			POP		BC
			SCF
			RET

; Read a character from UART0
;
UART0_serial_RX:	IN0		A,(UART0_REG_LSR)
			AND 		UART_LSR_RDY
			RET		Z
			IN0		A,(UART0_REG_RBR)
			SCF
			RET

; Blocking read with TST instruction
;
UART0_serial_GETCH:	PUSH		AF
			LD		A, (_serialFlags)
			TST		01h		; eZ80/Z180: TST instruction
			JR		Z, UART_serial_NE
			POP		AF
$$:			CALL 		UART0_serial_RX
			JR		NC,$B
			RET

; Blocking write with hardware flow control
;
UART0_serial_PUTCH:	PUSH	AF
			LD	A, (_serialFlags)
			TST	01h
			JR	Z, UART_serial_NE
			TST	02h		; Check flow control flag
			CALL	NZ, UART0_wait_CTS
			POP	AF
$$:			CALL	UART0_serial_TX
			JR	NC, $B
			RET

; Not enabled handler
;
UART_serial_NE:		POP	AF
			OR	A
			RET

; C wrapper: INT putch(INT ch)
;
_putch:
putch:			PUSH	IY
			LD	IY, 0
			ADD	IY, SP
			LD	A, (IY+6)	; INT ch (least significant byte)
			LD	HL, 0
			LD	L, A
			CALL	UART0_serial_PUTCH
			LD 	SP, IY
			POP	IY
			RET

; C wrapper: INT getch(VOID)
;
_getch:
getch:			PUSH	IY
			LD	IY, 0
			ADD	IY, SP
			CALL	UART0_serial_GETCH
			LD	HL, 0
			LD	L, A
			LD 	SP, IY
			POP	IY
			RET

; CTS check via GPIO
;
UART0_wait_CTS:		GET_GPIO	PD_DR, 8
			JR		NZ, UART0_wait_CTS
			RET

; UART1 handlers (same pattern, different port)
;
UART1_serial_TX:	PUSH		BC
			PUSH		AF
			LD		BC,TX_WAIT
UART1_serial_TX1:	IN0		A,(UART1_PORT+5)
			AND 		UART_LSR_ETX
			JR		NZ, UART1_serial_TX2
			DEC		BC
			LD		A, B
			OR		C
			JR		NZ, UART1_serial_TX1
			POP		AF
			POP		BC
			OR		A
			RET
UART1_serial_TX2:	POP		AF
			OUT0		(UART1_PORT+0),A
			POP		BC
			SCF
			RET

UART1_serial_RX:	IN0		A,(UART1_PORT+5)
			AND 		UART_LSR_RDY
			RET		Z
			IN0		A,(UART1_PORT+0)
			SCF
			RET
