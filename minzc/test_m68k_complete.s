| MinZ 68000 generated code
| Generated: 2025-08-05 16:20:46
| Target: Motorola 68000/68010/68020/68030/68040/68060
| Assembler: vasm/gas compatible


	.text
	.global _start


| Function: test_m68k_complete.add
| SMC enabled - parameters can be patched
test_m68k_complete.add:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
test_m68k_complete.add$param_a:
	move.l #0,d0		| SMC anchor for a
	move.l d0,d0
	move.l d0,d1
	move.l d0,d2
	add.l d1,d2
	move.l d2,d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Function: test_m68k_complete.multiply
| SMC enabled - parameters can be patched
test_m68k_complete.multiply:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
test_m68k_complete.multiply$param_a:
	move.l #0,d0		| SMC anchor for a
	move.l d0,d0
	move.l d0,d1
	move.w d0,d0
	muls.w d1,d0
	move.l d0,d2
	move.l d2,d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Function: test_m68k_complete.is_even
| SMC enabled - parameters can be patched
test_m68k_complete.is_even:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
test_m68k_complete.is_even$param_n:
	move.l #0,d0		| SMC anchor for n
	move.l d0,d0
	moveq #1,d1
	move.l d0,d2
	and.l d1,d2
	moveq #0,d3
	cmp.l d3,d2
	beq .true_L1
	moveq #0,d4
	bra .end_L1
.true_L1:
	moveq #1,d4
.end_L1:
	move.l d4,d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Function: test_m68k_complete.factorial
| SMC enabled - parameters can be patched
test_m68k_complete.factorial:
	link a6,#0
	movem.l d2-d7/a2-a5,-(sp)
test_m68k_complete.factorial$param_n:
	move.l #0,d0		| SMC anchor for n
	move.l d0,d0
	moveq #1,d1
	cmp.l d1,d0
	ble .true_L2
	moveq #0,d2
	bra .end_L2
.true_L2:
	moveq #1,d2
.end_L2:
	tst.l d2
	beq else_1
	moveq #1,d3
	move.l d3,d0
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts
	bra end_if_2
else_1:
end_if_2:
	move.l d0,d4
	move.l d0,d5
	moveq #1,d6
	move.l d5,d7
	sub.l d6,d7
	move.l d7,d0
	jsr factorial
	move.l d0,-40(a6)
	move.w d4,d0
	muls.w -40(a6),d0
	move.l d0,-44(a6)
	move.l -44(a6),d0
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
