; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/opcodes_a.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: ADC, ADD, AND instructions in ADL mode.
;              Tests all addressing modes: register, (HL), (IX+d), (IY+d), immediate.

	.assume adl=1
	adc a, (hl)
	adc a, (ix+5)
	adc a, (iy+5)
	adc a, 5
	adc a, a
	adc a, b
	adc a, c
	adc a, d
	adc a, e
	adc a, h
	adc a, l
	adc hl, bc
	adc hl, de
	adc hl, hl
	adc hl, sp
	add a, (hl)
	add a, (ix+5)
	add a, (iy+5)
	add a, 5
	add a, a
	add a, b
	add a, c
	add a, d
	add a, e
	add a, h
	add a, l
	add hl, bc
	add hl, de
	add hl, hl
	add hl, sp
	add ix, bc
	add ix, de
	add ix, ix
	add ix, sp
	add iy, bc
	add iy, de
	add iy, iy
	add iy, sp
	and a, (hl)
	and a, (ix+5)
	and a, (iy+5)
	and a, 5
	and a, a
	and a, b
	and a, c
	and a, d
	and a, e
	and a, h
	and a, l
