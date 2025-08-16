ORG $8000

; This should NOT be treated as a local label (starts with 3 dots)
...games.snake.SCREEN_WIDTH:
    DB 32

; This SHOULD be treated as a local label (starts with 1 dot)  
main:
.loop:
    NOP
    JR .loop
    
END