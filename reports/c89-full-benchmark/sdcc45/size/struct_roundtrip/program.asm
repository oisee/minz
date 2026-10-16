;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module struct_roundtrip
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _caller
	.globl _consume
	.globl _make_pair
	.globl _result
	.globl _b
	.globl _a
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_a::
	.ds 1
_b::
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
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:4: struct Pair make_pair(unsigned char x) {
;	---------------------------------
; Function make_pair
; ---------------------------------
_make_pair::
	call	___sdcc_enter_ix
	push	af
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:5: struct Pair p = {0, 0}; p.hi = x; p.lo = x + 1; return p;
	ld	hl, #0
	add	hl, sp
	ld	e, l
	ld	d, h
	inc	hl
	xor	a, a
	ld	(de), a
	ld	(hl), #0x00
	ld	a, c
	ld	(de), a
	inc	c
	ld	(hl), c
	ld	hl, #6
	add	hl, sp
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	hl, #0
	add	hl, sp
	ld	bc, #2
	ldir
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:6: }
	pop	af
	pop	ix
	pop	hl
	pop	af
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:7: unsigned short consume(struct Pair p) { return (unsigned short)p.hi * 256u + p.lo; }
;	---------------------------------
; Function consume
; ---------------------------------
_consume::
	call	___sdcc_enter_ix
	ld	b, 4 (ix)
	ld	c, #0x00
	ld	e, 5 (ix)
	ld	d, b
	pop	ix
	pop	hl
	pop	af
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:8: unsigned short caller(unsigned char x, unsigned char y) {
;	---------------------------------
; Function caller
; ---------------------------------
_caller::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	d, a
	ld	e, l
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:9: struct Pair p = make_pair(x);
	push	de
	ld	a, d
	ld	hl, #0x0004
	add	hl, sp
	push	hl
	call	_make_pair
	pop	de
	push	de
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	hl, #4
	add	hl, sp
	ldi
	ld	a, (hl)
	ld	(de), a
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:10: return (consume(p) + x) ^ y;
	ld	hl,#0x3
	add	hl,sp
	ld	b, (hl)
	dec	hl
	ld	c, (hl)
	push	bc
	call	_consume
	ex	de, hl
	pop	de
	ld	c, d
	ld	b, #0x00
	add	hl, bc
	ld	d, #0x00
	ld	a, l
	xor	a, e
	ld	l, a
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:11: }
	ld	sp, ix
	pop	ix
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/struct_roundtrip.c:12: void execute(void) { result = caller(a, b); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_b)
	ld	l, a
	ld	a, (_a)
	call	_caller
	ld	(_result), de
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
