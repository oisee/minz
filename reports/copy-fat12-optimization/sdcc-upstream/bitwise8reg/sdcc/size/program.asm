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
	.globl _test8
	.globl _or8
	.globl _and8
	.globl _c3
	.globl _c2
	.globl _c1
	.globl _c0
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
_c0::
	.ds 1
_c1::
	.ds 1
_c2::
	.ds 1
_c3::
	.ds 1
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
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:10: unsigned char and8(void)
;	---------------------------------
; Function and8
; ---------------------------------
_and8::
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:14: t0 = c0 + 1;
	ld	a, (_c0)
	ld	c, a
	inc	c
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:15: t1 = c1 + 1;
	ld	a, (_c1)
	ld	l, a
	inc	l
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:16: t2 = c2 + 1;
	ld	a, (_c2)
	ld	b, a
	inc	b
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:17: t3 = c3 + 1;
	ld	a, (_c3)
	ld	e, a
	inc	e
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:19: t4 = t0 & t1;
	ld	a, c
	and	a, l
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:20: t5 = t2 & t3;
	ld	a, b
	and	a, e
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:22: return(t4 + t5);
	add	a, c
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:23: }
	ret
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:25: unsigned char or8(void)
;	---------------------------------
; Function or8
; ---------------------------------
_or8::
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:29: t0 = c0 + 1;
	ld	a, (_c0)
	ld	c, a
	inc	c
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:30: t1 = c1 + 1;
	ld	a, (_c1)
	ld	l, a
	inc	l
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:31: t2 = c2 + 1;
	ld	a, (_c2)
	ld	b, a
	inc	b
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:32: t3 = c3 + 1;
	ld	a, (_c3)
	ld	e, a
	inc	e
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:34: t4 = t0 | t1;
	ld	a, c
	or	a, l
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:35: t5 = t2 | t3;
	ld	a, b
	or	a, e
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:37: return(t4 + t5);
	add	a, c
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:38: }
	ret
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:41: test8(void)
;	---------------------------------
; Function test8
; ---------------------------------
_test8::
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:43: c0 = 2;
	ld	hl, #_c0
	ld	(hl), #0x02
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:44: c1 = 2;
	ld	hl, #_c1
	ld	(hl), #0x02
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:45: c2 = 4;
	ld	hl, #_c2
	ld	(hl), #0x04
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:46: c3 = 4;
	ld	hl, #_c3
	ld	(hl), #0x04
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:48: ASSERT(and8() == 8);
	ld	hl, (_regression_checks)
	inc	hl
	ld	(_regression_checks), hl
	call	_and8
	sub	a, #0x08
	jr	Z, 00104$
	ld	hl, (_regression_failures)
	inc	hl
	ld	(_regression_failures), hl
00104$:
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:49: ASSERT(or8() == 8);
	ld	hl, (_regression_checks)
	inc	hl
	ld	(_regression_checks), hl
	call	_or8
	sub	a, #0x08
	ret	Z
	ld	hl, (_regression_failures)
	inc	hl
	ld	(_regression_failures), hl
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c:50: }
	ret
;/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/wrapper.c:3: void execute(void) { regression_failures=0; regression_checks=0; test8(); result=regression_failures ? 65535 : regression_checks; }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	hl, #0x0000
	ld	(_regression_failures), hl
	ld	(_regression_checks), hl
	call	_test8
	ld	a, (_regression_failures+1)
	ld	hl, #_regression_failures
	or	a, (hl)
	jr	Z, 00103$
	ld	hl, #0xffff
	jr	00104$
00103$:
	ld	hl, (_regression_checks)
00104$:
	ld	(_result), hl
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
