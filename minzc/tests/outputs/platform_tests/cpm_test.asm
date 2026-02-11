; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: cpm_test.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; Allocate struct FileInfo
    LD HL, -3
    ADD HL, SP
    LD SP, HL
    EX DE, HL
    LD HL, SP
    LD ($F004), HL    ; Virtual register 2 to memory
    ; r3 = 1
    LD A, 1
    ; Register 3 already in A
    ; r4 = 1024
    LD HL, 1024
    LD D, H
    LD E, L
    ; store fi, r2
    LD HL, ($F004)    ; Virtual register 2 from memory
    LD ($F002), HL
    ; Store to FileInfo.handle
    LD HL, ($F004)    ; Virtual register 2 from memory
    PUSH HL
    LD HL, 1      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; Store to FileInfo.size
    LD HL, ($F004)    ; Virtual register 2 from memory
    INC HL       ; Optimized: ADD HL,1 -> INC HL
    PUSH HL
    LD HL, 1024      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; Inlined from cpm_test.increment
    LD HL, ($F000)
    LD B, H
    LD C, L
    ; Inlined from cpm_test.increment
    LD A, 1
    LD ($F004), A     ; Virtual register 2 to memory
    ; Inlined from cpm_test.increment
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    LD H, B
    LD L, C
    ; Inlined from cpm_test.increment
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    LD H, B
    LD L, C
    LD D, H
    LD E, L
    ; Inlined from cpm_test.increment
    XOR A    ; Optimized: was LD A, 0
    EXX               ; Switch to shadow registers
    LD L, A         ; Store to shadow HL' (now active)
    EXX               ; Switch back to main registers
    ; CTIE: Computed at compile-time (was CALL cpm_test.add$u16$u16)
    LD HL, 300
    ; r15 = 5
    LD A, 5
    EXX               ; Switch to shadow registers
    LD C, A         ; Store to shadow C' (now active)
    EXX               ; Switch back to main registers
    ; Inlined from cpm_test.increment
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    LD H, B
    LD L, C
    LD ($F000), HL
    ; store sum, r12
    LD HL, 300      ; Constant
    LD ($F00E), HL
    ; Inlined from cpm_test.increment
    LD HL, 0      ; Constant
    LD ($F000), HL
    ; r16 = call cpm_test.count_up$u8(r15)

    ; *** SMC ANNOTATED CALL to cpm_test.count_up$u8 ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (count_up_u8_return_patch.op), A
    LD HL, temp_result
    LD (count_up_u8_store_addr), HL
    LD A, 5       ; Constant
    LD (count_up_u8_param_max+1), A   ; Patch max
    CALL count_up_u8
    ; *** END SMC CALL ***

    ; r19 = 10
    LD A, 10
    LD ($F026), A     ; Virtual register 19 to memory
    ; store count, r16
    EXX               ; Switch to shadow registers
    LD A, E         ; From shadow E' (now active)
    EXX               ; Switch back to main registers
    LD ($F01A), A
    ; r20 = call cpm_test.test_while$u8(r19)

    ; *** SMC ANNOTATED CALL to cpm_test.test_while$u8 ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (test_while_u8_return_patch.op), A
    LD HL, temp_result
    LD (test_while_u8_store_addr), HL
    LD A, 10       ; Constant
    LD (test_while_u8_param_limit+1), A   ; Patch limit
    CALL test_while_u8
    ; *** END SMC CALL ***

    ; store wcount, r20
    LD A, L
    LD ($F022), A
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: cpm_test.increment
increment:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r1 = load cpm_test.counter
    LD HL, ($F000)
    ; store counter, r1
    LD ($F000), HL
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: cpm_test.add$u16$u16
cpm_test.add$u16$u16:
; TRUE SMC function with immediate anchors
add_u16_u16_param_a$immOP:
    LD HL, 0       ; a anchor (will be patched)
add_u16_u16_param_a$imm0 EQU add_u16_u16_param_a$immOP+1
    ; Register 3 already in HL
add_u16_u16_param_b$immOP:
    LD HL, 0       ; b anchor (will be patched)
add_u16_u16_param_b$imm0 EQU add_u16_u16_param_b$immOP+1
    ; Register 4 already in HL
    ; r5 = r3 + r4
    ; Register 3 already in HL
    LD D, H
    LD E, L
    ; Register 4 already in HL
    ADD HL, DE
    ; Register 5 already in HL
    ; return r5
    ; Register 5 already in HL
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: cpm_test.count_up$u8
cpm_test.count_up$u8:
; TRUE SMC function with immediate anchors
    ; r3 = 0
    XOR A        ; MIR hint: zero via XOR
    LD L, A         ; Store to physical register L
    ; r4 = 0
    LD H, A         ; Store to physical register H
count_up_u8_param_max$immOP:
    LD A, 0        ; max anchor (will be patched)
count_up_u8_param_max$imm0 EQU count_up_u8_param_max$immOP+1
    LD L, A         ; Store to physical register L
    ; Initialize loop variable i
    LD HL, 0      ; Constant
    ; store count, r3
    XOR A          ; Constant 0
    LD ($F004), A
    ; for_loop_1:
cpm_test_count_up_u8_for_loop_1_i4:
    ; Check i < end
    ; 8-bit less-than comparison
    XOR A          ; Constant 0
    LD B, A
    LD A, L
    LD C, A
    LD A, B       ; A = Src1
    CP C          ; Compare Src1 with Src2
    JR C, cpm_test_count_up_u8_lt_true_0      ; Carry = Src1 < Src2
    LD HL, 0       ; False
    JR cpm_test_count_up_u8_lt_done_0
cpm_test_count_up_u8_lt_true_0:
    LD HL, 1       ; True
cpm_test_count_up_u8_lt_done_0:
    LD ($F00E), HL    ; Virtual register 7 to memory
    ; jump_if_not r7, for_end_2
    LD A, ($F00E)     ; Virtual register 7 from memory
    OR A
    JP Z, cpm_test_count_up_u8_for_end_2_i4   ; Test for zero
    ; r8 = load count
    LD A, ($F004)
    LD H, A         ; Store to physical register H
    ; Increment i
    LD A, L
    INC A
    LD L, A         ; Store to physical register L
    ; store count, r8
    LD A, H
    LD ($F004), A
    ; jump for_loop_1
    JP cpm_test_count_up_u8_for_loop_1_i4
    ; for_end_2:
cpm_test_count_up_u8_for_end_2_i4:
    ; r11 = load count
    LD A, ($F004)
    LD L, A         ; Store to HL (low byte)
    ; return r11
    ; Register 11 already in HL
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: cpm_test.test_while$u8
cpm_test.test_while$u8:
; TRUE SMC function with immediate anchors
    ; r3 = 0
    XOR A        ; MIR hint: zero via XOR
    LD H, A         ; Store to physical register H
    ; store i, r3
    LD ($F004), A
    ; loop_3:
cpm_test_test_while_u8_loop_3_i5:
    ; r4 = load i
    LD A, ($F004)
    LD ($F008), A     ; Virtual register 4 to memory
test_while_u8_param_limit$immOP:
    LD A, 0        ; limit anchor (will be patched)
test_while_u8_param_limit$imm0 EQU test_while_u8_param_limit$immOP+1
    LD H, A         ; Store to physical register H
    ; r6 = r4 < r5
    ; 8-bit less-than comparison
    XOR A          ; Constant 0
    LD B, A
    LD A, H
    LD C, A
    LD A, B       ; A = Src1
    CP C          ; Compare Src1 with Src2
    JR C, cpm_test_test_while_u8_lt_true_1      ; Carry = Src1 < Src2
    LD HL, 0       ; False
    JR cpm_test_test_while_u8_lt_done_1
cpm_test_test_while_u8_lt_true_1:
    LD HL, 1       ; True
cpm_test_test_while_u8_lt_done_1:
    ; jump_if_not r6, end_loop_4
    LD A, L
    OR A
    JP Z, cpm_test_test_while_u8_end_loop_4_i5   ; Test for zero
    ; r7 = load i
    LD A, ($F004)
    LD L, A         ; Store to HL (low byte)
    ; store i, r7
    LD A, L
    LD ($F004), A
    ; jump loop_3
    JP cpm_test_test_while_u8_loop_3_i5
    ; end_loop_4:
cpm_test_test_while_u8_end_loop_4_i5:
    ; r10 = load i
    LD A, ($F004)
    LD H, A         ; Store to physical register H
    ; return r10
    RET

; TRUE SMC PATCH-TABLE
; Format: DW anchor_addr, DB size, DB param_tag
PATCH_TABLE:
    DW add_u16_u16_param_a$imm0           ; cpm_test.add$u16$u16.a
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_u16_u16_param_b$imm0           ; cpm_test.add$u16$u16.b
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW count_up_u8_param_max$imm0           ; cpm_test.count_up$u8.max
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW test_while_u8_param_limit$imm0           ; cpm_test.test_while$u8.limit
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW 0              ; End of table
PATCH_TABLE_END:

; Standard library routines
temp_result:
    DW 0           ; Temporary storage for function results


; Data section (follows code contiguously)

cpm_test.counter:
    DW 0

    END


; Assembly peephole optimization: 10 patterns applied