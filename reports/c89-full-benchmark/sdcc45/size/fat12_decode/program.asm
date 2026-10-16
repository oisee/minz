;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module fat12_decode
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _fat12_decode
	.globl _result
	.globl _n
	.globl _values
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_values::
	.ds 64
_n::
	.ds 1
_result::
	.ds 2
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _INITIALIZED
;--------------------------------------------------------
; absolute external ram data
;--------------------------------------------------------
	.area _DABS (ABS)
;--------------------------------------------------------
; global & static initialisations
;--------------------------------------------------------
	.area _HOME
	.area _GSINIT
	.area _GSFINAL
	.area _GSINIT
;--------------------------------------------------------
; Home
;--------------------------------------------------------
	.area _HOME
	.area _HOME
;--------------------------------------------------------
; code
;--------------------------------------------------------
	.area _CODE
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:3: unsigned short fat12_decode(unsigned char *fat, unsigned short cluster) {
;	---------------------------------
; Function fat12_decode
; ---------------------------------
_fat12_decode::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	ld	c, l
	ld	b, h
	inc	sp
	inc	sp
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:4: unsigned short offset = cluster + cluster / 2;
	push	de
	ld	l, e
	ld	h, d
	srl	h
	rr	l
	add	hl, de
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:5: unsigned short word = fat[offset] | ((unsigned short)fat[offset + 1] << 8);
	push	hl
	pop	iy
	add	iy, bc
	ld	e, 0 (iy)
	inc	hl
	add	hl, bc
	ld	d, (hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:6: if (cluster & 1) return word >> 4;
	bit	0, -2 (ix)
	jr	Z, 00102$
	ld	b, #0x04
00113$:
	srl	d
	rr	e
	djnz	00113$
	jr	00103$
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:7: return word & 4095;
	ld	a, d
	and	a, #0x0f
	ld	d, a
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:8: }
	pop	af
	pop	ix
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/fat12_decode.c:9: void execute(void) { result = fat12_decode(values, n); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_n)
	ld	e, a
	ld	d, #0x00
	ld	hl, #_values
	call	_fat12_decode
	ld	(_result), de
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
