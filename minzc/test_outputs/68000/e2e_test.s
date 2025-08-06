| MinZ 68000 generated code
| Generated: 2025-08-06 12:38:56
| Target: Motorola 68000/68010/68020/68030/68040/68060
| Assembler: vasm/gas compatible


	.text
	.global _start


| Function: tests.minz.e2e_test.main
| SMC enabled - parameters can be patched
tests.minz.e2e_test.main:
	link a6,#-12
	movem.l d2-d7/a2-a5,-(sp)
	moveq #42,d0
	move.l d0,x
	moveq #10,d2
	move.l d2,y
	move.l x,d4
	move.l y,d5
	move.l d4,d6
	add.l d5,d6
	move.l d6,sum
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Print helpers
print_char:
	| Character in d0
	| Platform-specific implementation needed
	| Amiga: dos.library/Write
	| Atari ST: GEMDOS Cconout
	| Mac: _PBWrite trap
	rts

print_hex:
	move.b d0,d1
	lsr.b #4,d0
	bsr print_nibble
	move.b d1,d0
	bsr print_nibble
	rts

print_nibble:
	and.b #$0F,d0
	cmp.b #10,d0
	blt .digit
	add.b #'A'-10,d0
	bra print_char
.digit:
	add.b #'0',d0
	bra print_char

_start:
	jsr main
	move.l #0,d0
	trap #0		| Exit
