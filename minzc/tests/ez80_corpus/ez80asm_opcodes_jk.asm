; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/opcodes_j-k.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: JP and JR instructions in ADL mode.
;              In ADL mode, JP targets are 24-bit addresses (3 bytes).

	.assume adl=1
	jp nz, aabbcch
	jp z, aabbcch
	jp nc, aabbcch
	jp c, aabbcch
	jp po, aabbcch
	jp pe, aabbcch
	jp p, aabbcch
	jp m, aabbcch
labela:
	jp (hl)
	jp (ix)
	jp (iy)
	jp aabbcch
	jr nz, labela
	jr z, labela
	jr nc,labela
	jr c, labela
	jr labela
