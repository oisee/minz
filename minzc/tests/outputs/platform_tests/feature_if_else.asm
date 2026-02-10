; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.omvu5Wb2P8.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 5
    LD A, 5
    ; Register 2 already in A
    ; store x, r2
    LD ($F002), A
    ; r3 = load x
    LD A, ($F002)
    LD C, A         ; Store to physical register C
    ; r4 = 3
    LD A, 3
    LD D, A         ; Store to physical register D
    ; r5 = r3 > r4
    ; Optimized: x > 3
    LD A, C
    CP 4          ; Compare with 3+1
    JR C, tmp_omvu5Wb2P8_main_gt_false_0       ; If carry, x < 4+1, so x <= 3
    LD HL, 1       ; True: x > 3
    JR tmp_omvu5Wb2P8_main_gt_done_0
tmp_omvu5Wb2P8_main_gt_false_0:
    LD HL, 0       ; False: x <= 3
tmp_omvu5Wb2P8_main_gt_done_0:
    ; jump_if_not r5, else_1
    LD A, E
    OR A
    JP Z, tmp_omvu5Wb2P8_main_else_1_i1   ; Test for zero
    ; r6 = 1
    LD A, 1
    LD H, A         ; Store to physical register H
    ; store x, r6
    LD ($F002), A
    ; jump end_if_2
    JP tmp_omvu5Wb2P8_main_end_if_2_i1
    ; else_1:
tmp_omvu5Wb2P8_main_else_1_i1:
    ; r7 = 0
    XOR A        ; MIR hint: zero via XOR
    LD L, A         ; Store to physical register L
    ; store x, r7
    LD ($F002), A
    ; end_if_2:
tmp_omvu5Wb2P8_main_end_if_2_i1:
    ; return
    RET

    END


; Assembly peephole optimization: 4 patterns applied