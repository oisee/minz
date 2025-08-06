; MinZ 8080 generated code
; Generated: 2025-08-06 16:51:44
; Target: Intel 8080


; Code section
    ORG 08000H


; Function: tests.backend_e2e.sources.function_calls.add$u8$u8
; SMC enabled - parameters can be self-modified
tests.backend_e2e.sources.function_calls.add$u8$u8:
tests.backend_e2e.sources.function_calls.add$u8$u8$param_a:
    MVI A,0    ; SMC anchor for a
    PUSH B
    PUSH D
    PUSH H
    LDA tests.backend_e2e.sources.function_calls.add$u8$u8$param_a+1
    STA F006H
    LDA tests.backend_e2e.sources.function_calls.add$u8$u8$param_a+1
    STA F008H
    LDA F006H
    MOV B,A
    LDA F008H
    ADD B
    STA F00AH
    LDA F00AH
    POP H
    POP D
    POP B
    RET

; Function: tests.backend_e2e.sources.function_calls.main
; SMC enabled - parameters can be self-modified
tests.backend_e2e.sources.function_calls.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,05H
    STA F004H
    MVI A,03H
    STA F006H
    MVI A,05H
    STA F008H
    MVI A,03H
    STA F00AH
    LDA F008H
    STA tests.backend_e2e.sources.function_calls.add$u8$u8$param_param+1
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    STA F00CH
    LDA F00CH
    STA result
    LHLD result
    SHLD F010H
    LHLD result
    SHLD F012H
    LHLD result
    SHLD F014H
    LHLD result
    SHLD F016H
    LDA F014H
    STA tests.backend_e2e.sources.function_calls.add$u8$u8$param_param+1
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    STA F018H
    LDA F018H
    STA doubled
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
