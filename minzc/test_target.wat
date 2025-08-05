;; MinZ WebAssembly generated code
;; Generated: 2025-08-05 11:23:00

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: test_target.print_char
  (func $test_target.print_char (param $c i32) 
    return
  )

  ;; Function: test_target.main
  (func $test_target.main
    (local $msg i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    (local $r4 i32)
    (local $r5 i32)
    i32.const 65  ;; r2 = 65
    local.set $r2
    local.get $r2  ;; store 
    global.set $
    i32.const 0  ;; TODO: string offset for str_0
    local.set $r3
    ;; TODO: print string (needs memory access)
    global.get $msg  ;; r4 = msg
    local.set $r4
    call $print_char  ;; call print_char
    local.set $r5
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
