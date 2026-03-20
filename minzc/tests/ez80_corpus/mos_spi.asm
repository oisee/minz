; Source: https://github.com/breakintoprogram/agon-mos/blob/main/src/spi.asm
; License: MIT (https://github.com/breakintoprogram/agon-mos)
; Author: Leigh Brown
; Description: AGON MOS SPI low-level driver - production eZ80 firmware.
;              High-performance SPI using IN0/OUT0 with tight polling loops.
;              Demonstrates SCOPE directive, IX-based stack frames, DJNZ polling.

		.ASSUME ADL = 1

SPI_ENA_DELAY	.equ	50
SD_CS		.equ	4
SPI_MOSI	.equ	7
SPI_MISO	.equ	6
SPI_CLK		.equ	3

		XDEF	_init_spi
		XDEF	_spi_transfer
		XDEF	_spi_read_one
		XDEF	_spi_read
		XDEF	_spi_write

; void spi_init(void)
;
		SCOPE
_init_spi:
		; SS must remain high for SPI
		IN0		A,(PB_DR)
		SET		2,A
		OUT0		(PB_DR),A

		IN0		A,(PB_ALT1)
		RES		2,A
		OUT0		(PB_ALT1),A

		IN0		A,(PB_ALT2)
		RES		2,A
		OUT0		(PB_ALT2),A

		IN0		A,(PB_DDR)
		RES		2,A
		OUT0		(PB_DDR),A

		; Enable chip select, deselect
		IN0		A,(PB_DR)
		SET		4,A
		OUT0		(PB_DR),A

		; Set SPI pins to alternate function
		IN0		A,(PB_ALT1)
		AND		A,~((1<<SPI_MOSI)|(1<<SPI_MISO)|(1<<SPI_CLK))
		OUT0		(PB_ALT1),A

		IN0		A,(PB_ALT2)
		OR		A,((1<<SPI_MOSI)|(1<<SPI_MISO)|(1<<SPI_CLK))
		OUT0		(PB_ALT2),A

		; Disable SPI, set baud rate, enable as master
		XOR		A,A
		OUT0		(SPI_CTL),A
		LD		BC,3
		OUT0		(SPI_BRG_H),B
		OUT0		(SPI_BRG_L),C
		LD		A,%30
		OUT0		(SPI_CTL),A
		RET

; unsigned char spi_read_one(void)
;
		SCOPE
_spi_read_one:
		LD		C,%FF
		OUT0		(SPI_TSR),C
		POP		HL
		JR		spi_wait

; unsigned char spi_transfer(unsigned char d)
;
_spi_transfer:
		POP		HL
		POP		DE
		OUT0		(SPI_TSR),E
		PUSH		DE

spi_wait:	LD		B,0
$loop:		IN0		A,(SPI_SR)
		RLA
		JR		C,$done
		DJNZ		$loop
$done:		IN0		A,(SPI_RBR)
		JP		(HL)		; Faster than RET if we have HL

; void spi_read(char *buf, unsigned int len)
;
		SCOPE
_spi_read:
		LD		C,%FF
		OUT0		(SPI_TSR),C	; Request first byte
		PUSH		IX
		LD		IX,0
		ADD		IX,SP
		LD		DE,(IX+6)	; destination address
		LD		HL,(IX+9)	; byte count

$mainloop:	LD		BC,1
		OR		A,A
		SBC		HL,BC
		JR		Z,$waitlast

$waitnext:	LD		B,0
		LD		C,%FF
$loopnext:	IN0		A,(SPI_SR)
		RLA
		JR		C,$gotnext
		DJNZ		$loopnext

$gotnext:	IN0		A,(SPI_RBR)
		OUT0		(SPI_TSR),C	; Request next immediately
		LD		(DE),A
		INC		DE
		JR		$mainloop

$waitlast:	LD		SP,IX
		POP		IX
		LD		B,0
$looplast:	IN0		A,(SPI_SR)
		RLA
		JR		C,$gotlast
		DJNZ		$looplast
$gotlast:	IN0		A,(SPI_RBR)
		LD		(DE),A
		RET

; void spi_write(char *buf, unsigned int len)
;
		SCOPE
_spi_write:
		PUSH		IX
		LD		IX,0
		ADD		IX,SP
		LD		DE,(IX+6)	; source address
		LD		A,(DE)
		OUT0		(SPI_TSR),A	; Send first byte
		INC		DE
		LD		HL,(IX+9)	; byte count

$mainloop:	LD		BC,1
		OR		A,A
		SBC		HL,BC
		JR		Z,$waitlast

$waitnext:	LD		B,0
		LD		A,(DE)
		LD		C,A
		INC		DE
$loopnext:	IN0		A,(SPI_SR)
		RLA
		JR		C,$sentnext
		DJNZ		$loopnext

$sentnext:	OUT0		(SPI_TSR),C
		JR		$mainloop

$waitlast:	LD		SP,IX
		POP		IX
		LD		B,0
$looplast:	IN0		A,(SPI_SR)
		RLA
		JR		C,$sentlast
		DJNZ		$looplast
$sentlast:	RET
