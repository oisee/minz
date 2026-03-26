/* MinZ assert.h for Z80 targets */
#ifndef _MINZ_ASSERT_H
#define _MINZ_ASSERT_H

#ifdef NDEBUG
#define assert(expr) ((void)0)
#else
/*
 * On Z80: HALT instruction (0x76) stops the CPU.
 * In MinZ emulator: triggers breakpoint / exit.
 * For real hardware: infinite loop as fallback.
 */
extern void __assert_fail(void);
#define assert(expr) ((expr) ? (void)0 : __assert_fail())
#endif

/* C11 _Static_assert is handled by the parser directly */
#define static_assert _Static_assert

#endif /* _MINZ_ASSERT_H */
