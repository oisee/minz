; MinZ 8080 generated code
; Generated: 2025-08-06 23:01:52
; Target: Intel 8080


; Code section
    ORG 08000H


; Function: test_simple_i8080.main
; SMC enabled - parameters can be self-modified
test_simple_i8080.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,00H
    STA F002H
    LDA F002H
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
