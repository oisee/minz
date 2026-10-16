;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module case
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _testTortureExecute
	.globl _result
	.globl _regression_checks
	.globl _regression_failures
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_regression_failures::
	.ds 2
_regression_checks::
	.ds 2
_result::
	.ds 2
_a:
	.ds 398
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
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:15: foo ()
;	---------------------------------
; Function foo
; ---------------------------------
_foo:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:18: for (i = 198; i >= 0; i--)
	ld	bc, #_a+0
	ld	de, #0x00c6
00102$:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:19: a[i] = i;
	ld	l, e
	ld	h, d
	add	hl, hl
	add	hl, bc
	ld	(hl), e
	inc	hl
	ld	(hl), d
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:18: for (i = 198; i >= 0; i--)
	dec	de
	bit	7, d
	jr	Z, 00102$
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:20: }
	ret
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:24: testTortureExecute (void)
;	---------------------------------
; Function testTortureExecute
; ---------------------------------
_testTortureExecute::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:28: foo ();
	call	_foo
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:29: for (i = 0; i < 199; i++)
	ld	hl, #0x0000
	ex	(sp), hl
00109$:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:30: if (a[i] != i)
	pop	hl
	push	hl
	add	hl, hl
	ld	de, #_a
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	e, (hl)
	ld	a, -2 (ix)
	sub	a, c
	jr	NZ, 00135$
	xor	a, a
	sub	a, e
	jr	Z, 00110$
00135$:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:31: ASSERT (0);
	ld	hl, (_regression_checks)
	inc	hl
	ld	(_regression_checks), hl
	ld	hl, (_regression_failures)
	inc	hl
	ld	(_regression_failures), hl
00110$:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:29: for (i = 0; i < 199; i++)
	inc	-2 (ix)
	ld	a, -2 (ix)
	sub	a, #0xc7
	jr	C, 00109$
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:33: return;
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c:34: }
	ld	sp, ix
	pop	ix
	ret
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/wrapper.c:3: void execute(void) { regression_failures=0; regression_checks=0; testTortureExecute(); result=regression_failures ? 65535 : regression_checks; }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	hl, #0x0000
	ld	(_regression_failures), hl
	ld	(_regression_checks), hl
	call	_testTortureExecute
	ld	a, (_regression_failures+1)
	ld	hl, #_regression_failures
	or	a, (hl)
	jr	Z, 00103$
	ld	hl, #0xffff
	jp	00104$
00103$:
	ld	hl, (_regression_checks)
00104$:
	ld	(_result), hl
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
