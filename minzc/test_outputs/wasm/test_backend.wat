;; MinZ WebAssembly generated code
;; Generated: 2025-08-05 11:38:29
;; Note: WASM uses stack-based calling convention, no SMC

(module
  ;; Import memory
  (import "env" "memory" (memory 1))
  (import "env" "print_char" (func $print_char (param i32)))
  (import "env" "print_i32" (func $print_i32 (param i32)))

  ;; Export main function
  (export "main" (func $main))
)
