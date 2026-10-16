;* * * * *  Small-C/Plus z88dk * * * * *
;  Version: 1-83383009-20261016
;
;	Reconstructed for z80 Module Assembler
;
;	Module compile time: Fri Oct 16 13:55:15 2026


	C_LINE	0,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c"

	MODULE	X_home_alice_dev_minz_artifacts_grace_workspaces_c89_absdiff_cfg_research_c89_benchmark_cases_gcd_c


	INCLUDE "z80_crt0.hdr"


	C_LINE	1,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c"
	C_LINE	2,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c"
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c"
	SECTION	code_compiler

; Function gcd flags 0x00000200 __smallc 
; unsigned char gcd(unsigned char x, unsigned char y)
; parameter 'unsigned char y' at sp+2 size(1)
; parameter 'unsigned char x' at sp+4 size(1)
	C_LINE	3,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c::gcd::0::0"
._gcd
	ld	hl,4	;const
	add	hl,sp
	ld	a,(hl)
	and	a
	jp	nz,i_2	;
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


.i_2
	ld	hl,2	;const
	add	hl,sp
	ld	a,(hl)
	and	a
	jp	nz,i_3	;
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


.i_3
	ld	hl,4	;const
	add	hl,sp
	ld	e,(hl)
	ld	d,0
	ld	hl,2	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	call	l_ne
	jp	nc,i_5	;
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
	jp	nc,i_6	;
	ld	hl,4	;const
	add	hl,sp
	push	hl
	ld	e,(hl)
	ld	d,0
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	pop	de
	ld	a,l
	ld	(de),a
	jp	i_7	;EOS
.i_6
	ld	hl,2	;const
	add	hl,sp
	push	hl
	ld	e,(hl)
	ld	d,0
	ld	hl,6	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ex	de,hl
	and	a
	sbc	hl,de
	pop	de
	ld	a,l
	ld	(de),a
	jp	i_4	;EOS
.i_5
	ld	hl,4	;const
	add	hl,sp
	ld	l,(hl)
	ld	h,0
	ret


	C_LINE	12,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c::gcd::0::2"

; Function execute flags 0x00000200 __smallc 
; void execute()
	C_LINE	12,"/home/alice/dev/minz-artifacts/grace-workspaces/c89-absdiff-cfg/research/c89-benchmark/cases/gcd.c::execute::0::2"
._execute
	ld	hl,(_a)
	ld	h,0
	push	hl
	ld	hl,(_b)
	ld	h,0
	push	hl
	call	_gcd
	pop	bc
	pop	bc
	ld	(_result),hl
	ret


	SECTION	bss_compiler
	SECTION	code_compiler
; --- Start of Optimiser additions ---
	defc	i_7 = i_4
	defc	i_4 = i_3


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
	GLOBAL	_gcd
	GLOBAL	_execute


; --- End of Scope Defns ---


; --- End of Compilation ---
