;; MinZ WebAssembly generated code
;; Generated: 2025-08-06 16:51:44
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: tests.backend_e2e.sources.control_flow.main
  (func $tests.backend_e2e.sources.control_flow.main
    (local $i i32)
    (local $result i32)
    (local $result i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    (local $r4 i32)
    (local $r5 i32)
    (local $r6 i32)
    (local $r7 i32)
    (local $r8 i32)
    (local $r9 i32)
    (local $r10 i32)
    (local $r11 i32)
    (local $r12 i32)
    (local $r13 i32)
    (local $r14 i32)
    (local $r15 i32)
    i32.const 0  ;; r2 = 0
    local.set $r2
    local.get $r2  ;; store i
    global.set $i
    ;; Label: loop_1
    global.get $i  ;; r3 = i
    local.set $r3
    i32.const 10  ;; r4 = 10
    local.set $r4
    local.get $r3  ;; r5 = r3 < r4
    local.get $r4
    i32.lt_s
    local.set $r5
    ;; TODO: jump_if_not r5, end_loop_2 (needs block structure)
    global.get $i  ;; r6 = i
    local.set $r6
    i32.const 1  ;; r7 = 1
    local.set $r7
    local.get $r6  ;; r8 = r6 + r7
    local.get $r7
    i32.add
    local.set $r8
    local.get $r8  ;; store i
    global.set $i
    ;; TODO: jump loop_1 (needs block structure)
    ;; Label: end_loop_2
    global.get $i  ;; r9 = i
    local.set $r9
    i32.const 10  ;; r10 = 10
    local.set $r10
    local.get $r9  ;; r11 = r9 == r10
    local.get $r10
    i32.eq
    local.set $r11
    ;; TODO: jump_if_not r11, else_3 (needs block structure)
    i32.const 1  ;; r13 = 1
    local.set $r13
    local.get $r13  ;; store result
    global.set $result
    ;; TODO: jump end_if_4 (needs block structure)
    ;; Label: else_3
    i32.const 0  ;; r15 = 0
    local.set $r15
    local.get $r15  ;; store result
    global.set $result
    ;; Label: end_if_4
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
