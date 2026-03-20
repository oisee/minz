; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src/interrupts.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Dean Belfield
; Description: AGON MOS interrupt handlers - production eZ80 firmware.
;              Key eZ80 features demonstrated:
;              - RETI.L (24-bit return from interrupt)
;              - IN0/OUT0 for direct I/O port access
;              - 24-bit memory addressing in ADL mode
;              - Jump table dispatch via computed addresses
;              - I2C peripheral register manipulation

			.ASSUME	ADL = 1

			DEFINE .STARTUP, SPACE = ROM
			SEGMENT .STARTUP

			XDEF	_vblank_handler
			XDEF	_uart0_handler
			XDEF	_i2c_handler

			XREF	_clock
			XREF	_vdp_protocol_data
			XREF	UART0_serial_RX
			XREF	vdp_protocol

; AGON Vertical Blank Interrupt handler
;
_vblank_handler:	DI
			PUSH		AF
			PUSH		BC
			PUSH		DE
			PUSH		HL
			LD 		HL, (_clock)
			LD		BC, 2
			ADD		HL, BC
			LD		(_clock), HL
			LD		A, (_clock + 3)
			ADC		A, 0
			LD		(_clock + 3), A
			POP		HL
			POP		DE
			POP		BC
			POP		AF
			EI
			RETI.L			; eZ80: 24-bit return from interrupt

; AGON UART0 Interrupt Handler
;
_uart0_handler:		DI
			PUSH		AF
			PUSH		BC
			PUSH		DE
			PUSH		HL
			CALL		UART0_serial_RX
			LD		C, A
			LD		HL, _vdp_protocol_data
			CALL		vdp_protocol
			POP		HL
			POP		DE
			POP		BC
			POP		AF
			EI
			RETI.L			; eZ80: 24-bit return from interrupt

; AGON I2C Interrupt handler - jump table dispatch
;
_i2c_handler:
			DI
			PUSH	AF
			PUSH	HL
			PUSH	DE
			IN0	A, (I2C_SR)		; eZ80: IN0 reads direct port
			AND	11111000b
			RRA
			RRA
			RRA
			CALL	i2c_handle_sr_vector
			; Jump table follows the CALL
			DW	i2c_case_buserror	; 00h
			DW	i2c_case_master_start	; 08h
			DW	i2c_case_invalid	; 10h
			DW	i2c_case_aw_acked	; 18h
			DW	i2c_case_aw_nacked	; 20h
			DW	i2c_case_db_acked	; 28h
			DW	i2c_case_db_nacked	; 30h
			DW	i2c_case_arblost	; 38h

i2c_handle_sr_vector:
			EX	(SP), HL
			ADD	A, A
			ADD	A, L
			LD	L, A
			ADC	A, H
			SUB	L
			LD	H, A
			LD	A, (HL)
			INC	HL
			LD	H, (HL)
			LD	L, A
			EX	(SP), HL
			RET

i2c_case_buserror:
			; Software reset of I2C bus
			XOR	A
			OUT0	(I2C_SRR),A		; eZ80: OUT0 writes direct port
			POP	DE
			POP	HL
			POP	AF
			EI
			RETI.L

i2c_case_master_start:
			LD	A, (_i2c_slave_rw)
			OUT0	(I2C_DR), A
			LD	A, I2C_CTL_IEN | I2C_CTL_ENAB | I2C_CTL_AAK
			OUT0	(I2C_CTL),A
			POP	DE
			POP	HL
			POP	AF
			EI
			RETI.L

i2c_case_aw_acked:
i2c_case_db_acked:
			LD	A, (_i2c_msg_size)
			OR	A
			JR	Z, i2c_sendstop
			DEC	A
			LD	(_i2c_msg_size), A
			LD	HL, _i2c_msg_ptr
			LD	DE, HL
			LD	HL, (HL)
			LD	A, (HL)
			OUT0	(I2C_DR), A
			LD	A, I2C_CTL_IEN | I2C_CTL_ENAB | I2C_CTL_AAK
			OUT0	(I2C_CTL),A
			INC	HL
			EX	DE, HL
			LD	(HL), DE
			POP	DE
			POP	HL
			POP	AF
			EI
			RETI.L

i2c_case_aw_nacked:
i2c_case_invalid:
i2c_sendstop:
			LD	A, I2C_CTL_ENAB | I2C_CTL_STP
			OUT0	(I2C_CTL),A
$$:			IN0	A,(I2C_CTL)
			AND	I2C_CTL_STP
			JR	NZ, $B
			XOR	A
			OUT0	(I2C_CTL),A
			POP	DE
			POP	HL
			POP	AF
			EI
			RETI.L
