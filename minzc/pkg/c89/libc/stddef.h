/* MinZ minimal stddef.h for Z80 targets */
#ifndef _MINZ_STDDEF_H
#define _MINZ_STDDEF_H

#ifndef NULL
#define NULL ((void*)0)
#endif

typedef unsigned int size_t;
typedef int          ptrdiff_t;

#define offsetof(type, member) ((size_t)&(((type *)0)->member))

#endif /* _MINZ_STDDEF_H */
