;; MinZ WebAssembly generated code
;; Generated: 2025-08-05 11:41:07
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Function: test_mir_compilation.main
  (func $test_mir_compilation.main
    (local $x i32)
    (local $y i32)
    (local $sum i32)
  )

  ;; Function: test_mir_compilation.add
  (func $test_mir_compilation.add (param $a i32) (param $b i32)  (result i32)
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

  ;; Export main function
  (export "main" (func $main))
)
