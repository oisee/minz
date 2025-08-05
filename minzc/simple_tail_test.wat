;; MinZ WebAssembly generated code
;; Generated: 2025-08-05 22:10:53
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: simple_tail_test.countdown
  (func $simple_tail_test.countdown (param $n i32)  (result i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    (local $r4 i32)
    (local $r5 i32)
    (local $r6 i32)
    ;; TODO: TRUE_SMC_LOAD
    ;; Label: simple_tail_test.countdown_tail_loop
    ;; TODO: TEST
    ;; TODO: jump_if_not r4, else_1 (needs block structure)
    i32.const 0  ;; r5 = 0
    local.set $r5
    local.get $r5  ;; return
    return
    ;; Label: else_1
    ;; TODO: TRUE_SMC_LOAD
    ;; TODO: jump simple_tail_test.countdown_tail_loop (needs block structure)
    i32.const 0
  )

  ;; Function: simple_tail_test.main
  (func $simple_tail_test.main
    (local $result i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    call $countdown  ;; call countdown
    local.set $r3
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
