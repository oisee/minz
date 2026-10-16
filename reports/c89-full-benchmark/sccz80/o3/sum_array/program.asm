;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 13:55:15 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_absdiff_cfg_research_c89_benchmark_cases_sum_array_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	1,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c"
	C_LINE	2,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c"
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c"
	SECTION	code_compiler

; Function sum_array flags 0x00000200 __smallc 
; unsigned int sum_array(unsigned char * p, unsigned char count)
; parameter 'unsigned char count' at sp+2 size(1)
; parameter 'unsigned char * p' at sp+4 size(2)
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c::sum_array::0::0"
._sum_array
	ld	hl,0	;const
	push	hl
	dec	sp
	ld	hl,0	;const
	add	hl,sp
	ld	(hl),0
	jp	i_4	;EOS
.i_2
	ld	hl,0	;const
	add	hl,sp
	inc	(hl)
.i_4
	pop	hl
	push	hl
	ld	h,0
	ex	de,hl
	ld	hl,5	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	jp	nc,i_3	;
	ld	hl,1	;const
	add	hl,sp
	push	hl
	ld	e,(hl)
	inc	hl
	ld	d,(hl)
	push	de
	ld	hl,11	;const
	add	hl,sp
	ld	e,(hl)
	inc	hl
	ld	d,(hl)
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	add	hl,de
	ld	l,(hl)
	ld	h,0
	pop	de
	add	hl,de
	call	l_pint_pop
	jp	i_2	;EOS
.i_3
	call	l_gint1sp	;
	inc	sp
	pop	bc
	ret


	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c::sum_array::0::1"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/sum_array.c::execute::0::1"
._execute
	ld	hl,_values
	push	hl
	ld	hl,(_n)
	ld	h,0
	push	hl
	call	_sum_array
	pop	bc
	pop	bc
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---


; --- Start of Static Variables ---

	SECTION	bss_compiler
._values	defs	256
._n	defs	1
._result	defs	2
	SECTION	code_compiler


; --- Start of Scope Defns ---

	GLOBAL	_values
	GLOBAL	_n
	GLOBAL	_result
	GLOBAL	_sum_array
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
