;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module abs_diff
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _abs_diff
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
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/abs_diff.c:3: unsigned char abs_diff(unsigned char x, unsigned char y) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/abs_diff.c:4: if (x > y) return x - y;
	ld	a, l
	sub	a, c
	jr	NC, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/abs_diff.c:5: return y - x;
	ld	a, l
	sub	a, c
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/abs_diff.c:6: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/abs_diff.c:7: void execute(void) { result = abs_diff(a, b); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_b)
	ld	l, a
	ld	a, (_a)
	call	_abs_diff
	ld	(_result+0), a
	xor	a, a
	ld	(_result+1), a
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
