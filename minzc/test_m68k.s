| MinZ 68000 generated code
| Generated: 2025-08-05 16:20:19
| Target: Motorola 68000/68010/68020/68030/68040/68060
| Assembler: vasm/gas compatible


	.text
	.global _start


| Function: test_m68k.fibonacci
| SMC enabled - parameters can be patched
test_m68k.fibonacci:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
test_m68k.fibonacci$param_n:
	move.l #0,d0		| SMC anchor for n
	move.l d0,d0
	moveq #1,d1
	cmp.l d1,d0
	ble .true_L1
	moveq #0,d2
	bra .end_L1
.true_L1:
	moveq #1,d2
.end_L1:
	tst.l d2
	beq else_1
	move.l d0,d3
	move.l d3,d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts
	bra end_if_2
else_1:
end_if_2:
	move.l d0,d4
	moveq #1,d5
	move.l d4,d6
	sub.l d5,d6
	move.l d6,d0
	jsr fibonacci
	move.l d0,d7
	move.l d0,-40(a6)
	move.l #2,-44(a6)
	move.l -40(a6),-48(a6)
	sub.l -44(a6),-48(a6)
	move.l -48(a6),d0
	jsr fibonacci
	move.l d0,-52(a6)
	move.l d7,-56(a6)
	add.l -52(a6),-56(a6)
	move.l -56(a6),d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Function: test_m68k.main
| SMC enabled - parameters can be patched
test_m68k.main:
	link a6,#-4
	movem.l d2-d7/a2-a5,-(sp)
	moveq #10,d0
	jsr fibonacci
	move.l d0,d1
	move.l d1,-40(a6)
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
