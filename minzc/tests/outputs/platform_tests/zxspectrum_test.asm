; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; Allocate struct Point
    LD HL, -2
    ADD HL, SP
    LD SP, HL
    EX DE, HL
    LD HL, SP
    LD ($F004), HL    ; Virtual register 2 to memory
    ; r3 = 10
    LD A, 10
    ; Register 3 already in A
    ; r4 = 20
    LD A, 20
    LD B, A         ; Store to physical register B
    ; store p, r2
    LD HL, ($F004)    ; Virtual register 2 from memory
    LD ($F002), HL
    ; Store to Point.x
    LD HL, ($F004)    ; Virtual register 2 from memory
    PUSH HL
    LD HL, 10      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; Store to Point.y
    LD HL, ($F004)    ; Virtual register 2 from memory
    INC HL       ; Optimized: ADD HL,1 -> INC HL
    PUSH HL
    LD HL, 20      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; r10 = load p
    LD HL, ($F002)
    ; Load field x (offset 0)
    LD E, (HL)
    INC HL
    LD D, (HL)
    EX DE, HL
    ; Load field y (offset 1)
    ; r14 = call zxspectrum_test.add_u8$u8$u8(r11, r13)

    ; *** SMC ANNOTATED CALL to zxspectrum_test.add_u8$u8$u8 ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (add_u8_u8_u8_return_patch.op), A
    LD HL, temp_result
    LD (add_u8_u8_u8_store_addr), HL
    EXX               ; Switch to shadow registers
    LD A, B         ; From shadow B' (now active)
    EXX               ; Switch back to main registers
    LD (add_u8_u8_u8_param_a+1), A   ; Patch a
    EXX               ; Switch to shadow registers
    LD A, C         ; From shadow C' (now active)
    EXX               ; Switch back to main registers
    LD (add_u8_u8_u8_param_b+1), A   ; Patch b
    CALL add_u8_u8_u8
    ; *** END SMC CALL ***

    ; r17 = 15
    LD A, 15
    EXX               ; Switch to shadow registers
    LD E, A         ; Store to shadow E' (now active)
    EXX               ; Switch back to main registers
    ; store sum, r14
    EXX               ; Switch to shadow registers
    LD A, D         ; From shadow D' (now active)
    EXX               ; Switch back to main registers
    LD ($F00A), A
    ; r18 = call zxspectrum_test.test_if$u8(r17)

    ; *** SMC ANNOTATED CALL to zxspectrum_test.test_if$u8 ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (test_if_u8_return_patch.op), A
    LD HL, temp_result
    LD (test_if_u8_store_addr), HL
    LD A, 15       ; Constant
    LD (test_if_u8_param_x+1), A   ; Patch x
    CALL test_if_u8
    ; *** END SMC CALL ***

    ; store result, r18
    LD A, H
    LD ($F01E), A
    ; r20 = call zxspectrum_test.test_while

    ; *** SMC ANNOTATED CALL to zxspectrum_test.test_while ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (test_while_return_patch.op), A
    LD HL, temp_result
    LD (test_while_store_addr), HL
    CALL test_while
    LD ($F028), HL    ; Virtual register 20 to memory
    ; *** END SMC CALL ***

    ; store wresult, r20
    LD A, ($F028)     ; Virtual register 20 from memory
    LD ($F026), A
    ; r22 = call zxspectrum_test.test_for

    ; *** SMC ANNOTATED CALL to zxspectrum_test.test_for ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (test_for_return_patch.op), A
    LD HL, temp_result
    LD (test_for_store_addr), HL
    CALL test_for
    ; *** END SMC CALL ***

    ; store fresult, r22
    LD A, L
    LD ($F02A), A
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.add_u8$u8$u8
zxspectrum_test.add_u8$u8$u8:
; TRUE SMC function with immediate anchors
add_u8_u8_u8_param_a$immOP:
    LD A, 0        ; a anchor (will be patched)
add_u8_u8_u8_param_a$imm0 EQU add_u8_u8_u8_param_a$immOP+1
    LD H, A         ; Store to physical register H
add_u8_u8_u8_param_b$immOP:
    LD A, 0        ; b anchor (will be patched)
add_u8_u8_u8_param_b$imm0 EQU add_u8_u8_u8_param_b$immOP+1
    LD L, A         ; Store to physical register L
    ; r5 = r3 + r4
    LD HL, 10      ; Constant
    LD D, H
    LD E, L
    LD HL, 20      ; Constant
    ADD HL, DE
    ; return r5
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.add_u16$u16$u16
zxspectrum_test.add_u16$u16$u16:
; TRUE SMC function with immediate anchors
add_u16_u16_u16_param_a$immOP:
    LD HL, 0       ; a anchor (will be patched)
add_u16_u16_u16_param_a$imm0 EQU add_u16_u16_u16_param_a$immOP+1
    ; Register 3 already in HL
add_u16_u16_u16_param_b$immOP:
    LD HL, 0       ; b anchor (will be patched)
add_u16_u16_u16_param_b$imm0 EQU add_u16_u16_u16_param_b$immOP+1
    LD B, H
    LD C, L
    ; r5 = r3 + r4
    LD HL, 10      ; Constant
    LD D, H
    LD E, L
    LD HL, 20      ; Constant
    ADD HL, DE
    ; Register 5 already in HL
    ; return r5
    ; Register 5 already in HL
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.test_if$u8
zxspectrum_test.test_if$u8:
; TRUE SMC function with immediate anchors
test_if_u8_param_x$immOP:
    LD A, 0        ; x anchor (will be patched)
test_if_u8_param_x$imm0 EQU test_if_u8_param_x$immOP+1
    LD ($F004), A     ; Virtual register 2 to memory
    ; r3 = 10
    LD A, 10
    LD H, A         ; Store to physical register H
    ; r4 = r2 > r3
    ; Optimized: x > 10
    LD A, ($F004)     ; Virtual register 2 from memory
    CP 11          ; Compare with 10+1
    JR C, zxspectrum_test_test_if_u8_gt_false_0       ; If carry, x < 11+1, so x <= 10
    LD HL, 1       ; True: x > 10
    JR zxspectrum_test_test_if_u8_gt_done_0
zxspectrum_test_test_if_u8_gt_false_0:
    LD HL, 0       ; False: x <= 10
zxspectrum_test_test_if_u8_gt_done_0:
    ; jump_if_not r4, else_1
    LD A, 20       ; Constant
    OR A
    JP Z, zxspectrum_test_test_if_u8_else_1_i4   ; Test for zero
    ; r5 = 1
    LD A, 1
    LD L, A         ; Store to HL (low byte)
    ; return r5
    LD HL, 1      ; Constant
    RET
    ; else_1:
zxspectrum_test_test_if_u8_else_1_i4:
    ; r6 = 0
    XOR A        ; MIR hint: zero via XOR
    LD H, A         ; Store to physical register H
    ; return r6
    LD HL, 0      ; Constant
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.test_while
test_while:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 0
    XOR A        ; MIR hint: zero via XOR
    LD L, A         ; Store to physical register L
    ; r4 = 0
    EXX               ; Switch to shadow registers
    LD B, A         ; Store to shadow B' (now active)
    EXX               ; Switch back to main registers
    ; store i, r4
    XOR A          ; Constant 0
    LD ($F006), A
    ; store sum, r2
    XOR A          ; Constant 0
    LD ($F002), A
    ; loop_3:
zxspectrum_test_test_while_loop_3_i5:
    ; r5 = load i
    LD A, ($F006)
    LD L, A         ; Store to physical register L
    ; r6 = 5
    LD A, 5
    LD ($F00C), A     ; Virtual register 6 to memory
    ; r7 = r5 < r6
    ; 8-bit less-than comparison
    LD A, L
    LD B, A
    LD A, 5       ; Constant
    LD C, A
    LD A, B       ; A = Src1
    CP C          ; Compare Src1 with Src2
    JR C, zxspectrum_test_test_while_lt_true_1      ; Carry = Src1 < Src2
    LD HL, 0       ; False
    JR zxspectrum_test_test_while_lt_done_1
zxspectrum_test_test_while_lt_true_1:
    LD HL, 1       ; True
zxspectrum_test_test_while_lt_done_1:
    ; jump_if_not r7, end_loop_4
    LD A, H
    OR A
    JP Z, zxspectrum_test_test_while_end_loop_4_i5   ; Test for zero
    ; r8 = load sum
    LD A, ($F002)
    LD ($F010), A     ; Virtual register 8 to memory
    ; r9 = load i
    LD A, ($F006)
    LD H, A         ; Store to physical register H
    ; r10 = r8 + r9
    LD HL, ($F010)    ; Virtual register 8 from memory
    LD D, H
    LD E, L
    ADD HL, DE
    ; store i, r9
    LD A, H
    LD ($F006), A
    ; store sum, r10
    LD A, L
    LD ($F002), A
    ; jump loop_3
    JP zxspectrum_test_test_while_loop_3_i5
    ; end_loop_4:
zxspectrum_test_test_while_end_loop_4_i5:
    ; r14 = load sum
    LD A, ($F002)
    LD L, A         ; Store to HL (low byte)
    LD A, L

    ; *** SMART PATCHABLE RETURN SEQUENCE ***
    ; Default: Store to memory (most common complex case)
    ; For immediate use: Patch first NOP to RET for early return
zxspectrum_test.test_while_return_patch.op:
    NOP                     ; PATCH POINT: NOP or RET (C9) for early return
zxspectrum_test.test_while_store_addr.op:
zxspectrum_test.test_while_store_addr equ zxspectrum_test.test_while_store_addr.op + 1
    LD (0000), A            ; DEFAULT: Store result (address gets patched)
    RET                     ; Return after store
; Using hierarchical register allocation (physical → shadow → memory)

; Function: zxspectrum_test.test_for
test_for:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 0
    XOR A        ; MIR hint: zero via XOR
    LD H, A         ; Store to physical register H
    ; DJNZ OPTIMIZED: for i in 0..5
    NOP
    ; store sum, r2
    XOR A          ; Constant 0
    LD ($F002), A
    ; djnz_loop_5:
zxspectrum_test_test_for_djnz_loop_5_i6:
    ; r5 = load sum
    LD A, ($F002)
    LD ($F00A), A     ; Virtual register 5 to memory
    ; Increment i
    LD A, H
    INC A
    LD H, A         ; Store to physical register H
    ; DJNZ - decrement B and loop
    LD A, L
    LD B, A
    DJNZ zxspectrum_test_test_for_djnz_loop_5_i6
    LD A, B
    LD L, A         ; Store to physical register L
    ; store sum, r5
    LD A, ($F00A)     ; Virtual register 5 from memory
    LD ($F002), A
    ; r8 = load sum
    LD A, ($F002)
    LD L, A         ; Store to HL (low byte)
    LD A, L

    ; *** SMART PATCHABLE RETURN SEQUENCE ***
    ; Default: Store to memory (most common complex case)
    ; For immediate use: Patch first NOP to RET for early return
zxspectrum_test.test_for_return_patch.op:
    NOP                     ; PATCH POINT: NOP or RET (C9) for early return
zxspectrum_test.test_for_store_addr.op:
zxspectrum_test.test_for_store_addr equ zxspectrum_test.test_for_store_addr.op + 1
    LD (0000), A            ; DEFAULT: Store result (address gets patched)
    RET                     ; Return after store

; TRUE SMC PATCH-TABLE
; Format: DW anchor_addr, DB size, DB param_tag
PATCH_TABLE:
    DW add_u8_u8_u8_param_a$imm0           ; zxspectrum_test.add_u8$u8$u8.a
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_u8_u8_u8_param_b$imm0           ; zxspectrum_test.add_u8$u8$u8.b
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_u16_u16_u16_param_a$imm0           ; zxspectrum_test.add_u16$u16$u16.a
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_u16_u16_u16_param_b$imm0           ; zxspectrum_test.add_u16$u16$u16.b
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW test_if_u8_param_x$imm0           ; zxspectrum_test.test_if$u8.x
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW 0              ; End of table
PATCH_TABLE_END:

; Standard library routines
temp_result:
    DW 0           ; Temporary storage for function results


; Data section (follows code contiguously)

zxspectrum_test.test_u8:
    DB 42
zxspectrum_test.test_u16:
    DW 1000
zxspectrum_test.test_bool:
    DB 1

    END


; Assembly peephole optimization: 3 patterns applied