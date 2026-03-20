; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/z180_new.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: Z180/eZ80 specific instructions: IN0, OUT0, MLT, TST, TSTIO, SLP, OTIM, OTDM.
;              These are shared between Z180 and eZ80 instruction sets.
;              MLT is particularly important - 8x8 multiply using register pairs.

; Test if all Z180 opcodes pass the CPU filter
.cpu Z180
	IN0 B,(0)
	IN0 D,(0)
	IN0 H,(0)
	OUT0 (0),B
	OUT0 (0),D
	OUT0 (0),H
	OTIM
	OTIMR
	TST A,B
	TST A,D
	TST A,H
	TST A,(HL)
	TST A,0
	TSTIO 0
	SLP
	IN0 C,(0)
	IN0 E,(0)
	IN0 L,(0)
	IN0 A,(0)
	OUT0 (0),C
	OUT0 (0),E
	OUT0 (0),L
	OUT0 (0),A
	OTDM
	OTDMR
	TST A,C
	TST A,E
	TST A,L
	TST A,A
	; MLT: 8-bit multiply. Multiplies high and low bytes of register pair.
	; Result stored in register pair. E.g. MLT BC: B*C -> BC
	MLT BC
	MLT DE
	MLT HL
	MLT SP
