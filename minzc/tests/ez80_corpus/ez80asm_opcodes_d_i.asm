; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/opcodes_d-i.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: DAA through INIRX instructions in ADL mode.
;              Includes eZ80-specific: IN0, IND2, IND2R, INDM, INDMR, INDRX, INI2, INI2R, INIM, INIMR, INIRX

	;.assume adl=1
	daa
	dec (hl)
	dec ixh
	dec ixl
	dec iyh
	dec iyl
	dec ix
	dec iy
	dec (ix+5)
	dec (iy+5)
	dec a
	dec b
	dec c
	dec d
	dec e
	dec h
	dec l
	dec bc
	dec de
	dec hl
	dec sp
	di
label:
	djnz label
	ei
	ex af, af'
	ex de, hl
	ex (sp), hl
	ex (sp), ix
	ex (sp), iy
	exx
	halt
	im 0
	im 1
	im 2
	in a, (bc)
	in b, (bc)
	in c, (bc)
	in d, (bc)
	in e, (bc)
	in h, (bc)
	in l, (bc)
	in a, (c)
	in b, (c)
	in c, (c)
	in d, (c)
	in e, (c)
	in h, (c)
	in l, (c)
	; eZ80-specific: IN0 reads from direct port address
	in0 a, (5)
	in0 b, (5)
	in0 c, (5)
	in0 d, (5)
	in0 e, (5)
	in0 h, (5)
	in0 l, (5)
	inc (hl)
	inc ixh
	inc ixl
	inc iyh
	inc iyl
	inc ix
	inc iy
	inc (ix+5)
	inc (iy+5)
	inc a
	inc b
	inc c
	inc d
	inc e
	inc h
	inc l
	inc bc
	inc de
	inc hl
	inc sp
	; Block I/O instructions (Z80 + eZ80 extensions)
	ind
	ind2      ; eZ80: block input with 2-byte port
	ind2r     ; eZ80: block input with 2-byte port, repeat
	indm      ; eZ80: block input decrement with port in C, B=count
	indmr     ; eZ80: block input decrement repeat
	indr
	indrx     ; eZ80: block input decrement with 24-bit addressing
	ini
	ini2      ; eZ80: block input increment with 2-byte port
	ini2r     ; eZ80: block input increment with 2-byte port, repeat
	inim      ; eZ80: block input increment with port in C, B=count
	inimr     ; eZ80: block input increment repeat
	inir
	inirx     ; eZ80: block input increment with 24-bit addressing
