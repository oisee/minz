;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 14:19:13 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_old_claims_replay_research_c89_benchmark_old_claims_cases_old_min8_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	4,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c"
	SECTION	code_compiler

; Function fibonacci flags 0x00000200 __smallc 
; int fibonacci(int n)
; parameter 'int n' at sp+2 size(2)
	C_LINE	4,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fibonacci::0::0"
._fibonacci
	ld	hl,0	;const
	push	hl
	inc	hl
	push	hl
.i_2
	call	l_gint6sp	;
	ld	de,0
	ex	de,hl
	call	l_gt
	jp	nc,i_3	;
	ld	hl,2	;const
	call	l_gintspsp	;
	pop	bc
	pop	hl
	push	hl
	push	bc
	pop	de
	add	hl,de
	push	hl
	ld	hl,4	;const
	add	hl,sp
	push	hl
	dec	hl
	dec	hl
	call	l_gint	;
	call	l_pint_pop
	pop	hl
	ld	d,h
	ld	e,l
	pop	bc
	push	hl
	push	de
	ld	hl,8	;const
	add	hl,sp
	dec	(hl)
	ld	a,(hl)
	inc	hl
	cp	255
	jr	nz,ASMPC+3
	dec	(hl)
	ld	h,(hl)
	ld	l,a
	pop	bc
	jp	i_2	;EOS
.i_3
	pop	bc
	pop	hl
	ret


	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fibonacci::0::2"

; Function fib10 flags 0x00000200 __smallc 
; int fib10()
	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fib10::0::2"
._fib10
	ld	hl,10	;const
	push	hl
	call	_fibonacci
	pop	bc
	ret


	C_LINE	11,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fib10::0::3"

; Function sum_to flags 0x00000200 __smallc 
; int sum_to(int n)
; parameter 'int n' at sp+2 size(2)
	C_LINE	11,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::sum_to::0::3"
._sum_to
	ld	hl,0	;const
	push	hl
	push	hl
.i_4
	ld	hl,0	;const
	call	l_gintspsp	;
	call	l_gint8sp	;
	pop	de
	call	l_lt
	jp	nc,i_5	;
	ld	hl,2	;const
	call	l_gintspsp	;
	pop	bc
	pop	hl
	push	hl
	push	bc
	pop	de
	add	hl,de
	pop	de
	pop	bc
	push	hl
	push	de
	ld	l,e
	ld	h,d
	inc	hl
	pop	bc
	push	hl
	jp	i_4	;EOS
.i_5
	pop	bc
	pop	hl
	ret


	C_LINE	16,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::sum_to::0::5"

; Function sum10 flags 0x00000200 __smallc 
; int sum10()
	C_LINE	16,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::sum10::0::5"
._sum10
	ld	hl,10	;const
	push	hl
	call	_sum_to
	pop	bc
	ret


	C_LINE	18,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::sum10::0::6"

; Function factorial flags 0x00000200 __smallc 
; int factorial(int n)
; parameter 'int n' at sp+2 size(2)
	C_LINE	18,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::factorial::0::6"
._factorial
	ld	hl,1	;const
	push	hl
.i_6
	call	l_gint4sp	;
	ld	de,1
	ex	de,hl
	call	l_gt
	jp	nc,i_7	;
	ld	hl,0	;const
	call	l_gintspsp	;
	call	l_gint6sp	;
	pop	de
	call	l_mult
	pop	bc
	push	hl
	ld	hl,4	;const
	add	hl,sp
	dec	(hl)
	ld	a,(hl)
	inc	hl
	cp	255
	jr	nz,ASMPC+3
	dec	(hl)
	jp	i_6	;EOS
.i_7
	pop	hl
	ret


	C_LINE	23,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::factorial::0::8"

; Function fact6 flags 0x00000200 __smallc 
; int fact6()
	C_LINE	23,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fact6::0::8"
._fact6
	ld	hl,6	;const
	push	hl
	call	_factorial
	pop	bc
	ret


	C_LINE	26,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::fact6::0::9"

; Function twice flags 0x00000200 __smallc 
; int twice(int x)
; parameter 'int x' at sp+2 size(2)
	C_LINE	26,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::twice::0::9"
._twice
	pop	bc
	pop	hl
	push	hl
	push	bc
	ld	d,h
	ld	e,l
	add	hl,de
	ret


	C_LINE	27,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::twice::0::10"

; Function add flags 0x00000200 __smallc 
; int add(int a, int b)
; parameter 'int b' at sp+2 size(2)
; parameter 'int a' at sp+4 size(2)
	C_LINE	27,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::add::0::10"
._add
	ld	hl,4	;const
	call	l_gintspsp	;
	call	l_gint4sp	;
	pop	de
	add	hl,de
	ret


	C_LINE	30,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::add::0::11"

; Function abs_diff flags 0x00000200 __smallc 
; unsigned char abs_diff(unsigned char a, unsigned char b)
; parameter 'unsigned char b' at sp+2 size(1)
; parameter 'unsigned char a' at sp+4 size(1)
	C_LINE	30,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::abs_diff::0::11"
._abs_diff
	ld	hl,4	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	and	a
	sbc	hl,de
	jp	nc,i_8	;
	ld	hl,4	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	ld	h,0
	ret


.i_8
	ld	hl,2	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	ld	h,0
	ret


	C_LINE	35,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::abs_diff::0::12"

; Function clamp flags 0x00000200 __smallc 
; unsigned char clamp(unsigned char val, unsigned char lo, unsigned char hi)
; parameter 'unsigned char hi' at sp+2 size(1)
; parameter 'unsigned char lo' at sp+4 size(1)
; parameter 'unsigned char val' at sp+6 size(1)
	C_LINE	35,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::clamp::0::12"
._clamp
	ld	hl,6	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	jp	nc,i_9	;
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


.i_9
	ld	hl,6	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	and	a
	sbc	hl,de
	jp	nc,i_10	;
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


.i_10
	ld	hl,6	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


	C_LINE	41,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::clamp::0::13"

; Function min8 flags 0x00000200 __smallc 
; unsigned char min8(unsigned char a, unsigned char b)
; parameter 'unsigned char b' at sp+2 size(1)
; parameter 'unsigned char a' at sp+4 size(1)
	C_LINE	41,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::min8::0::13"
._min8
	ld	hl,4	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	jp	nc,i_11	;
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


.i_11
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


	C_LINE	47,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::min8::0::14"

; Function square flags 0x00000200 __smallc 
; int square(int x)
; parameter 'int x' at sp+2 size(2)
	C_LINE	47,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::square::0::14"
._square
	pop	bc
	pop	hl
	push	hl
	push	bc
	ld	d,h
	ld	e,l
	call	l_mult
	ret


	C_LINE	48,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::square::0::15"

; Function demo flags 0x00000200 __smallc 
; int demo()
	C_LINE	48,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::demo::0::15"
._demo
	ld	hl,4	;const
	push	hl
	call	_square
	pop	bc
	push	hl
	ld	hl,5	;const
	push	hl
	call	_twice
	pop	bc
	pop	de
	add	hl,de
	ret


	C_LINE	51,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::demo::0::16"

; Function wrap_min flags 0x00000200 __smallc 
; unsigned char wrap_min(unsigned char a, unsigned char b)
; parameter 'unsigned char b' at sp+2 size(1)
; parameter 'unsigned char a' at sp+4 size(1)
	C_LINE	51,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_min::0::16"
._wrap_min
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	push	hl
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	push	hl
	call	_min8
	pop	bc
	pop	bc
	ld	h,0
	ret


	C_LINE	52,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_min::0::17"

; Function wrap_twice flags 0x00000200 __smallc 
; int wrap_twice(int x)
; parameter 'int x' at sp+2 size(2)
	C_LINE	52,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_twice::0::17"
._wrap_twice
	pop	bc
	pop	hl
	push	hl
	push	bc
	push	hl
	call	_twice
	pop	bc
	ret


	C_LINE	55,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_twice::0::18"
	C_LINE	55,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_twice::0::18"
	C_LINE	56,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::wrap_twice::0::18"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	56,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_min8.c::execute::0::18"
._execute
	ld	hl,(_a)
	ld	h,0
	push	hl
	ld	hl,(_b)
	ld	h,0
	push	hl
	call	_min8
	pop	bc
	pop	bc
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---


; --- Start of Static Variables ---

	SECTION	bss_compiler
._result	defs	2
._a	defs	1
._b	defs	1
	SECTION	code_compiler


; --- Start of Scope Defns ---

	GLOBAL	_fibonacci
	GLOBAL	_fib10
	GLOBAL	_sum_to
	GLOBAL	_sum10
	GLOBAL	_factorial
	GLOBAL	_fact6
	GLOBAL	_twice
	GLOBAL	_add
	GLOBAL	_abs_diff
	GLOBAL	_clamp
	GLOBAL	_min8
	GLOBAL	_square
	GLOBAL	_demo
	GLOBAL	_wrap_min
	GLOBAL	_wrap_twice
	GLOBAL	_result
	GLOBAL	_a
	GLOBAL	_b
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
