; MinZ 8080 generated code
; Generated: 2025-08-06 12:14:00
; Target: Intel 8080


; Code section
    ORG 08000H


; Function: tests.minz.e2e_test.main
; SMC enabled - parameters can be self-modified
tests.minz.e2e_test.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,2AH
    STA F004H
    LHLD F004H
    SHLD 
    MVI A,0AH
    STA F008H
    LHLD F008H
    SHLD 
    LHLD x
    SHLD F00CH
    LHLD y
    SHLD F00EH
    LDA F00CH
    MOV B,A
    LDA F00EH
    ADD B
    STA F010H
    LHLD F010H
    SHLD 
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
