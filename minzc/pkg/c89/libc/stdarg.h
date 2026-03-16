/* MinZ minimal stdarg.h for Z80 targets */
#ifndef _MINZ_STDARG_H
#define _MINZ_STDARG_H

typedef void *va_list;
#define va_start(ap, last) ((void)((ap) = (void*)(&(last) + 1)))
#define va_end(ap)         ((void)0)
#define va_arg(ap, type)   (*(type*)((ap) = (char*)(ap) + sizeof(type), (char*)(ap) - sizeof(type)))

#endif /* _MINZ_STDARG_H */
