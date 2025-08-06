; MinZ LLVM IR generated code
; Target: LLVM IR (compatible with LLVM 10+)

declare i32 @printf(i8*, ...)
declare i32 @putchar(i32)
declare void @exit(i32)
declare i8* @malloc(i64)
declare void @free(i8*)

; Function declarations

define void @tests_backend_e2e_sources_arrays_main() {
entry:
  %arr.addr = alloca [5 x i8]
  %first.addr = alloca i8
  %last.addr = alloca i8
  %sum.addr = alloca i8
  %r3 = load i8, i8* %arr.addr
  %r4 = add i8 0, 0
  ; TODO: LOAD_INDEX
  store i8 %r5, i8* %first.addr
  %r7 = load i8, i8* %arr.addr
  %r8 = add i8 0, 4
  ; TODO: LOAD_INDEX
  store i8 %r9, i8* %last.addr
  %r11 = load i8, i8* %first.addr
  %r12 = load i8, i8* %last.addr
  %r13 = add i8 %r11, %r12
  store i8 %r13, i8* %sum.addr
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
