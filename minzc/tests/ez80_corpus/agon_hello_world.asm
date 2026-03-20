; Source: https://github.com/schur/Agon-Light-Assembly/blob/main/hello_world/hello_world.asm
; License: MIT (https://github.com/schur/Agon-Light-Assembly)
; Author: Dean Belfield, Reinhard Schu
; Description: Agon Light hello world with hex printing.
;              Demonstrates LD.L (24-bit load in Z80 mode compatibility).

MAIN:
	LD	IX,HELLO_WORLD
	CALL	PRSTR

;       Test printing Hex numbers
	LD	A,$A7
	CALL	Print_Hex8
	LD	HL,$E39F
	CALL	Print_Hex16
	LD.L	HL,$6A9BF4		; eZ80: 24-bit load
	CALL	Print_Hex24
	LD	HL,0
	RET

HELLO_WORLD:	.DB	"Hello World\n\r",0
