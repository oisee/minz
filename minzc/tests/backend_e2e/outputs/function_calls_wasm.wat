;; MinZ WebAssembly generated code
;; Generated: 2025-08-06 16:51:44
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: tests.backend_e2e.sources.function_calls.add$u8$u8
  (func $tests.backend_e2e.sources.function_calls.add$u8$u8 (param $a i32) (param $b i32)  (result i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    (local $r4 i32)
    (local $r5 i32)
    ;; TODO: LOAD_PARAM
    ;; TODO: LOAD_PARAM
    local.get $r3  ;; r5 = r3 + r4
    local.get $r4
    i32.add
    local.set $r5
    local.get $r5  ;; return
    return
  )

  ;; Function: tests.backend_e2e.sources.function_calls.main
  (func $tests.backend_e2e.sources.function_calls.main
    (local $result i32)
    (local $doubled i32)
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
    i32.const 5  ;; r2 = 5
    local.set $r2
    i32.const 3  ;; r3 = 3
    local.set $r3
    i32.const 5  ;; r4 = 5
    local.set $r4
    i32.const 3  ;; r5 = 3
    local.set $r5
    call $tests.backend_e2e.sources.function_calls.add$u8$u8  ;; call tests.backend_e2e.sources.function_calls.add$u8$u8
    local.set $r6
    local.get $r6  ;; store result
    global.set $result
    global.get $result  ;; r8 = result
    local.set $r8
    global.get $result  ;; r9 = result
    local.set $r9
    global.get $result  ;; r10 = result
    local.set $r10
    global.get $result  ;; r11 = result
    local.set $r11
    call $tests.backend_e2e.sources.function_calls.add$u8$u8  ;; call tests.backend_e2e.sources.function_calls.add$u8$u8
    local.set $r12
    local.get $r12  ;; store doubled
    global.set $doubled
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
