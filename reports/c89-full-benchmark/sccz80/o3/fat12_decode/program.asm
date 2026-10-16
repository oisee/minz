;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 13:55:15 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_absdiff_cfg_research_c89_benchmark_cases_fat12_decode_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	1,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c"
	C_LINE	2,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c"
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c"
	SECTION	code_compiler

; Function fat12_decode flags 0x00000200 __smallc 
; unsigned int fat12_decode(unsigned char * fat, unsigned int cluster)
; parameter 'unsigned int cluster' at sp+2 size(2)
; parameter 'unsigned char * fat' at sp+4 size(2)
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c::fat12_decode::0::0"
._fat12_decode
	pop	bc
	pop	hl
	push	hl
	push	bc
	push	hl
	srl	h
	rr	l
	pop	de
	add	hl,de
	push	hl
	ld	hl,6	;const
	call	l_gintspsp	;
	pop	bc
	pop	hl
	push	hl
	push	bc
	pop	de
	add	hl,de
	ld	l,(hl)
	ld	h,0
	push	hl
	ld	hl,8	;const
	call	l_gintspsp	;
	call	l_gint4sp	;
	inc	hl
	pop	de
	add	hl,de
	pop	de
	ld	a,d
	or	(hl)
	ld	d,a
	ex	de,hl
	push	hl
	call	l_gint6sp	;
	ld	a,l
	and	1
	jp	z,i_2	;
	pop	hl
	push	hl
	ld	de,4
	call	l_asr_u_hl_by_e
	pop	bc
	pop	bc
	ret


.i_2
	pop	hl
	push	hl
	ld	de,4095
	call	l_and
	pop	bc
	pop	bc
	ret


	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c::fat12_decode::0::1"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	9,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/fat12_decode.c::execute::0::1"
._execute
	ld	hl,_values
	push	hl
	ld	hl,(_n)
	ld	h,0
	push	hl
	call	_fat12_decode
	pop	bc
	pop	bc
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---


; --- Start of Static Variables ---

	SECTION	bss_compiler
._values	defs	64
._n	defs	1
._result	defs	2
	SECTION	code_compiler


; --- Start of Scope Defns ---

	GLOBAL	_values
	GLOBAL	_n
	GLOBAL	_result
	GLOBAL	_fat12_decode
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
