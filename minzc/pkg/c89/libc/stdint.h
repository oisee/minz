/* MinZ minimal stdint.h for Z80 targets */
#ifndef _MINZ_STDINT_H
#define _MINZ_STDINT_H

typedef unsigned char  uint8_t;
typedef signed char    int8_t;
typedef unsigned int   uint16_t;
typedef signed int     int16_t;
typedef unsigned long  uint32_t;
typedef signed long    int32_t;

/* 64-bit not natively supported on Z80; provide stub type */
typedef unsigned long  uint64_t;
typedef signed long    int64_t;

typedef unsigned int   size_t;
typedef int            ptrdiff_t;
typedef int            intptr_t;
typedef unsigned int   uintptr_t;

#define INT8_MIN   (-128)
#define INT8_MAX   127
#define UINT8_MAX  255
#define INT16_MIN  (-32768)
#define INT16_MAX  32767
#define UINT16_MAX 65535
#define INT32_MIN  (-2147483647L - 1)
#define INT32_MAX  2147483647L
#define UINT32_MAX 4294967295UL

#endif /* _MINZ_STDINT_H */
