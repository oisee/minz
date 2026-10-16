;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module gcd
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _gcd
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
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:3: unsigned char gcd(unsigned char x, unsigned char y) {
;	---------------------------------
; Function gcd
; ---------------------------------
_gcd::
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:4: if (x == 0) return y;
	ld	c, a
	or	a, a
	jr	NZ, 00102$
	ld	a, l
	ret
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:5: if (y == 0) return x;
	ld	a, l
	or	a, a
	jr	NZ, 00108$
	ld	a, c
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:6: while (x != y) {
00108$:
	ld	a, c
	sub	a, l
	jr	Z, 00110$
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:7: if (x > y) x -= y;
	ld	a, l
	sub	a, c
	jr	NC, 00106$
	ld	a, c
	sub	a, l
	ld	c, a
	jr	00108$
00106$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:8: else y -= x;
	ld	a, l
	sub	a, c
	ld	l, a
	jr	00108$
00110$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:10: return x;
	ld	a, c
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:11: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-main-audit/research/c89-benchmark/cases/gcd.c:12: void execute(void) { result = gcd(a, b); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_b)
	ld	l, a
	ld	a, (_a)
	call	_gcd
	ld	(_result+0), a
	xor	a, a
	ld	(_result+1), a
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
