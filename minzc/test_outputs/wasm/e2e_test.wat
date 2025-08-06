;; MinZ WebAssembly generated code
;; Generated: 2025-08-06 12:38:56
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: tests.minz.e2e_test.main
  (func $tests.minz.e2e_test.main
    (local $x i32)
    (local $y i32)
    (local $sum i32)
    (local $r1 i32)
    (local $r2 i32)
    (local $r3 i32)
    (local $r4 i32)
    (local $r5 i32)
    (local $r6 i32)
    (local $r7 i32)
    (local $r8 i32)
    i32.const 42  ;; r2 = 42
    local.set $r2
    local.get $r2  ;; store x
    global.set $x
    i32.const 10  ;; r4 = 10
    local.set $r4
    local.get $r4  ;; store y
    global.set $y
    global.get $x  ;; r6 = x
    local.set $r6
    global.get $y  ;; r7 = y
    local.set $r7
    local.get $r6  ;; r8 = r6 + r7
    local.get $r7
    i32.add
    local.set $r8
    local.get $r8  ;; store sum
    global.set $sum
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
