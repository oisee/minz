;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 13:55:15 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_absdiff_cfg_research_c89_benchmark_cases_copy_bytes_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	1,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c"
	C_LINE	2,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c"
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c"
	SECTION	code_compiler

; Function copy_bytes flags 0x00000200 __smallc 
; void copy_bytes(unsigned char * dst, unsigned char * src, unsigned char count)
; parameter 'unsigned char count' at sp+2 size(1)
; parameter 'unsigned char * src' at sp+4 size(2)
; parameter 'unsigned char * dst' at sp+6 size(2)
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c::copy_bytes::0::0"
._copy_bytes
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
	ld	hl,3	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	jp	nc,i_3	;
	ld	hl,7	;const
	add	hl,sp
	ld	e,(hl)
	inc	hl
	ld	d,(hl)
	pop	hl
	push	hl
	ld	h,0
	add	hl,de
	push	hl
	ld	hl,7	;const
	add	hl,sp
	ld	e,(hl)
	inc	hl
	ld	d,(hl)
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	add	hl,de
	ld	a,(hl)
	pop	de
	ld	(de),a
	jp	i_2	;EOS
.i_3
	inc	sp
	ret


	C_LINE	7,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c::copy_bytes::0::1"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	7,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/copy_bytes.c::execute::0::1"
._execute
	ld	hl,_target
	push	hl
	ld	hl,_source
	push	hl
	ld	hl,(_n)
	ld	h,0
	push	hl
	call	_copy_bytes
	pop	bc
	pop	bc
	pop	bc
	ld	hl,(_n)
	ld	h,0
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---


; --- Start of Static Variables ---

	SECTION	bss_compiler
._source	defs	64
._target	defs	64
._n	defs	1
._result	defs	2
	SECTION	code_compiler


; --- Start of Scope Defns ---

	GLOBAL	_source
	GLOBAL	_target
	GLOBAL	_n
	GLOBAL	_result
	GLOBAL	_copy_bytes
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
