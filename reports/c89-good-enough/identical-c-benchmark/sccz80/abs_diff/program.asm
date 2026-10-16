;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 12:16:23 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_measured_benchmark_research_c89_benchmark_cases_abs_diff_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	1,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c"
	C_LINE	2,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c"
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c"
	SECTION	code_compiler

; Function abs_diff flags 0x00000200 __smallc 
; unsigned char abs_diff(unsigned char x, unsigned char y)
; parameter 'unsigned char y' at sp+2 size(1)
; parameter 'unsigned char x' at sp+4 size(1)
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c::abs_diff::0::0"
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
	jp	nc,i_2	;
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


.i_2
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


	C_LINE	7,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c::abs_diff::0::1"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	7,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-measured-benchmark/research/c89-benchmark/cases/abs_diff.c::execute::0::1"
._execute
	ld	hl,(_a)
	ld	h,0
	push	hl
	ld	hl,(_b)
	ld	h,0
	push	hl
	call	_abs_diff
	pop	bc
	pop	bc
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---


; --- Start of Static Variables ---

	SECTION	bss_compiler
._a	defs	1
._b	defs	1
._result	defs	2
	SECTION	code_compiler


; --- Start of Scope Defns ---

	GLOBAL	_a
	GLOBAL	_b
	GLOBAL	_result
	GLOBAL	_abs_diff
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
