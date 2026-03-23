; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/adl_extensions.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: ADL mode switching and suffix testing.
;              Tests .LIL/.SIS/.LIS/.SIL suffixes on LD and JP instructions.

	assume adl=0
	ld a,(aabbh)
	ld.lil a,(aabbh)

	assume adl=1
	ld a,(aabbh)
	ld.sis a,(aabbh)

	assume adl=0
	jp (hl)
	jp.l (hl)
	jp.lis (hl)

	assume adl=1
	jp (hl)
	jp.s (hl)
	jp.sil (hl)
