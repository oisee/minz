;; MinZ WebAssembly generated code
;; Generated: 2025-08-06 16:51:43
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: tests.backend_e2e.sources.arrays.main
  (func $tests.backend_e2e.sources.arrays.main
    (local $arr i32)
    (local $first i32)
    (local $last i32)
    (local $sum i32)
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
    global.get $arr  ;; r3 = arr
    local.set $r3
    i32.const 0  ;; r4 = 0
    local.set $r4
    ;; TODO: LOAD_INDEX
    local.get $r5  ;; store first
    global.set $first
    global.get $arr  ;; r7 = arr
    local.set $r7
    i32.const 4  ;; r8 = 4
    local.set $r8
    ;; TODO: LOAD_INDEX
    local.get $r9  ;; store last
    global.set $last
    global.get $first  ;; r11 = first
    local.set $r11
    global.get $last  ;; r12 = last
    local.set $r12
    local.get $r11  ;; r13 = r11 + r12
    local.get $r12
    i32.add
    local.set $r13
    local.get $r13  ;; store sum
    global.set $sum
    return
  )

  ;; Export main function
  (export "main" (func $main))
)
