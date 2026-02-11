; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.NmO8eMcZEA.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 0
    XOR A        ; MIR hint: zero via XOR
    ; Register 2 already in A
    ; DJNZ OPTIMIZED: for i in 0..5
    NOP
    ; store s, r2
    XOR A          ; Constant 0
    LD ($F002), A
    ; djnz_loop_1:
tmp_NmO8eMcZEA_main_djnz_loop_1_i1:
    ; r5 = load s
    LD A, ($F002)
    LD C, A         ; Store to physical register C
    ; Increment i
    LD A, D
    INC A
    LD D, A         ; Store to physical register D
    ; DJNZ - decrement B and loop
    LD A, E
    LD B, A
    DJNZ tmp_NmO8eMcZEA_main_djnz_loop_1_i1
    LD A, B
    LD E, A         ; Store to physical register E
    ; store s, r5
    LD A, C
    LD ($F002), A
    ; return
    RET

    END


; Assembly peephole optimization: no patterns matched