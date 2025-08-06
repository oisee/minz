| MinZ 68000 generated code
| Generated: 2025-08-06 16:51:44
| Target: Motorola 68000/68010/68020/68030/68040/68060
| Assembler: vasm/gas compatible


	.text
	.global _start


| Function: tests.backend_e2e.sources.control_flow.main
| SMC enabled - parameters can be patched
tests.backend_e2e.sources.control_flow.main:
	link a6,#-12
	movem.l d2-d7/a2-a5,-(sp)
	moveq #0,d0
	move.l d0,i
loop_1:
	move.l i,d2
	moveq #10,d3
	cmp.l d3,d2
	blt .true_L1
	moveq #0,d4
	bra .end_L1
.true_L1:
	moveq #1,d4
.end_L1:
	tst.l d4
	beq end_loop_2
	move.l i,d5
	moveq #1,d6
	move.l d5,d7
	add.l d6,d7
	move.l d7,i
	bra loop_1
end_loop_2:
	move.l i,-40(a6)
	move.l #10,-44(a6)
	cmp.l -44(a6),-40(a6)
	beq .true_L2
	moveq #0,-48(a6)
	bra .end_L2
.true_L2:
	moveq #1,-48(a6)
.end_L2:
	tst.l -48(a6)
	beq else_3
	move.l #1,-52(a6)
	move.l -52(a6),result
	bra end_if_4
else_3:
	move.l #0,-60(a6)
	move.l -60(a6),result
end_if_4:
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
