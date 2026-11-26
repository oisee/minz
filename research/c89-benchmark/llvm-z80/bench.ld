/* Bare link for the benchmark: code at 0x8000, data at 0xC000, as for SDCC
   (--code-loc 0x8000 --data-loc 0xC000). No startup code, no OS. */
SECTIONS {
  . = 0x8000;
  .text : { *(.text .text.*) *(.rodata .rodata.*) }
  . = 0xC000;
  .data : { *(.data .data.*) }
  .bss : { *(.bss .bss.*) *(COMMON) }
  /DISCARD/ : { *(.note.GNU-stack) *(.comment) *(.llvm_addrsig) }
}
