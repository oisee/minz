; MinZ 8080 generated code
; Generated: 2025-08-06 16:51:44
; Target: Intel 8080


; Code section
    ORG 08000H


; Function: tests.backend_e2e.sources.control_flow.main
; SMC enabled - parameters can be self-modified
tests.backend_e2e.sources.control_flow.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,00H
    STA F004H
    LDA F004H
    STA i
loop_1:
    LHLD i
    SHLD F006H
    MVI A,0AH
    STA F008H
    LDA F006H
    MOV B,A
    LDA F008H
    CMP B
    JC true_L1
    XRA A
    JMP end_L1
true_L1:
    MVI A,1
end_L1:
    STA F00AH
    LDA F00AH
    ORA A
    JZ end_loop_2
    LHLD i
    SHLD F00CH
    MVI A,01H
    STA F00EH
    LDA F00CH
    MOV B,A
    LDA F00EH
    ADD B
    STA F010H
    LHLD F010H
    SHLD i
    JMP loop_1
end_loop_2:
    LHLD i
    SHLD F012H
    MVI A,0AH
    STA F014H
    LDA F012H
    MOV B,A
    LDA F014H
    CMP B
    JZ true_L2
    XRA A
    JMP end_L2
true_L2:
    MVI A,1
end_L2:
    STA F016H
    LDA F016H
    ORA A
    JZ else_3
    MVI A,01H
    STA F01AH
    LDA F01AH
    STA result
    JMP end_if_4
else_3:
    MVI A,00H
    STA F01EH
    LDA F01EH
    STA result
end_if_4:
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
