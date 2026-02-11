; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.7DovKiLdwQ.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 0
    XOR A        ; MIR hint: zero via XOR
    ; Register 2 already in A
    ; store i, r2
    LD ($F002), A
    ; loop_1:
tmp_7DovKiLdwQ_main_loop_1_i1:
    ; r3 = load i
    LD A, ($F002)
    LD C, A         ; Store to physical register C
    ; r4 = 5
    LD A, 5
    LD D, A         ; Store to physical register D
    ; r5 = r3 < r4
    ; 8-bit less-than comparison
    LD A, C
    LD B, A
    LD A, 5       ; Constant
    LD C, A
    LD A, B       ; A = Src1
    CP C          ; Compare Src1 with Src2
    JR C, tmp_7DovKiLdwQ_main_lt_true_0      ; Carry = Src1 < Src2
    LD HL, 0       ; False
    JR tmp_7DovKiLdwQ_main_lt_done_0
tmp_7DovKiLdwQ_main_lt_true_0:
    LD HL, 1       ; True
tmp_7DovKiLdwQ_main_lt_done_0:
    ; jump_if_not r5, end_loop_2
    LD A, E
    OR A
    JP Z, tmp_7DovKiLdwQ_main_end_loop_2_i1   ; Test for zero
    ; r6 = load i
    LD A, ($F002)
    LD H, A         ; Store to physical register H
    ; store i, r6
    LD A, H
    LD ($F002), A
    ; jump loop_1
    JP tmp_7DovKiLdwQ_main_loop_1_i1
    ; end_loop_2:
tmp_7DovKiLdwQ_main_end_loop_2_i1:
    ; return
    RET

    END


; Assembly peephole optimization: 2 patterns applied