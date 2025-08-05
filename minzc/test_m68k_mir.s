| MinZ 68000 generated code
| Generated: 2025-08-05 16:23:11
| Target: Motorola 68000/68010/68020/68030/68040/68060
| Assembler: vasm/gas compatible


	.text
	.global _start


| Function: test_add
test_add:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
	move.l d0,d0
	move.l d0,d1
	move.l d0,d2
	add.l d0,d2
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Function: main
main:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
	moveq #0,d0
	moveq #0,d1
	jsr 
	move.l d0,d2
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
