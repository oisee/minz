; MinZ 8080 generated code
; Generated: 2025-08-06 16:51:43
; Target: Intel 8080


; Code section
    ORG 08000H


; Function: tests.backend_e2e.sources.basic_math.main
; SMC enabled - parameters can be self-modified
tests.backend_e2e.sources.basic_math.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,2AH
    STA F004H
    LDA F004H
    STA x
    MVI A,0AH
    STA F008H
    LDA F008H
    STA y
    LHLD x
    SHLD F00CH
    LHLD y
    SHLD F00EH
    LDA F00CH
    MOV B,A
    LDA F00EH
    ADD B
    STA F010H
    LDA F010H
    STA sum
    LHLD x
    SHLD F014H
    LHLD y
    SHLD F016H
    LDA F014H
    MOV B,A
    LDA F016H
    SUB B
    STA F018H
    LDA F018H
    STA diff
    LHLD x
    SHLD F01CH
    MVI A,02H
    STA F01EH
    LDA F01CH
    MOV B,A
    LDA F01EH
    CALL multiply_8x8
    STA F020H

; 8x8 multiply routine
multiply_8x8:
    ; A = multiplicand, B = multiplier
    MOV C,A
    XRA A
mult_loop:
    ADD C
    DCR B
    JNZ mult_loop
    RET
    LDA F020H
    STA prod
    POP H
    POP D
    POP B
    RET

; Print helpers
print_char:
    ; Platform-specific print routine
    ; For CP/M: CALL 0005H with C=02H, E=char
    MOV E,A
    MVI C,02H
    CALL 0005H
    RET

print_newline:
    MVI A,0DH    ; CR
    CALL print_char
    MVI A,0AH    ; LF
    CALL print_char
    RET

; End of generated code
    END
