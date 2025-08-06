; MinZ LLVM IR generated code
; Target: LLVM IR (compatible with LLVM 10+)

declare i32 @printf(i8*, ...)
declare i32 @putchar(i32)
declare void @exit(i32)
declare i8* @malloc(i64)
declare void @free(i8*)

; Function declarations

define i8 @tests_backend_e2e_sources_function_calls_add_u8_u8(i8 %a, i8 %b) {
entry:
  ; TODO: LOAD_PARAM
  ; TODO: LOAD_PARAM
  %r5 = add i8 %r3, %r4
  ret void %r5
}

define void @tests_backend_e2e_sources_function_calls_main() {
entry:
  %result.addr = alloca i8
  %doubled.addr = alloca i8
  %r2 = add i8 0, 5
  %r3 = add i8 0, 3
  %r4 = add i8 0, 5
  %r5 = add i8 0, 3
  %r6 = call void @tests_backend_e2e_sources_function_calls_add_u8_u8(i8 %r4, i8 %r5)
  store i8 %r6, i8* %result.addr
  %r8 = load i8, i8* %result.addr
  %r9 = load i8, i8* %result.addr
  %r10 = load i8, i8* %result.addr
  %r11 = load i8, i8* %result.addr
  %r12 = call void @tests_backend_e2e_sources_function_calls_add_u8_u8(i8 %r10, i8 %r11)
  store i8 %r12, i8* %doubled.addr
  ret void
}


; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}

@.str.u8 = private constant [4 x i8] c"%u\0A\00"

; main wrapper
define i32 @main() {
  call void @examples_test_llvm_main()
  ret i32 0
}
