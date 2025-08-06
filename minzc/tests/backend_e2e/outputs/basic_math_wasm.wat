;; MinZ WebAssembly generated code
;; Generated: 2025-08-06 16:51:43
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: tests.backend_e2e.sources.basic_math.main
  (func $tests.backend_e2e.sources.basic_math.main
    (local $x i32)
    (local $y i32)
    (local $sum i32)
    (local $diff i32)
    (local $prod i32)
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
    (local $r16 i32)
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
    global.get $x  ;; r10 = x
    local.set $r10
    global.get $y  ;; r11 = y
    local.set $r11
    local.get $r10  ;; r12 = r10 - r11
    local.get $r11
    i32.sub
    local.set $r12
    local.get $r12  ;; store diff
    global.set $diff
    global.get $x  ;; r14 = x
    local.set $r14
    i32.const 2  ;; r15 = 2
    local.set $r15
    local.get $r14  ;; r16 = r14 * r15
    local.get $r15
    i32.mul
    local.set $r16
    local.get $r16  ;; store prod
    global.set $prod
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
