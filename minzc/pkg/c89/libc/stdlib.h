/* MinZ minimal stdlib.h for Z80 targets */
#ifndef _MINZ_STDLIB_H
#define _MINZ_STDLIB_H

#ifndef NULL
#define NULL ((void*)0)
#endif

typedef unsigned int size_t;

void *malloc(size_t size);
void  free(void *ptr);
void *calloc(size_t nmemb, size_t size);
void *realloc(void *ptr, size_t size);

int   atoi(const char *s);
long  atol(const char *s);

void  abort(void);
void  exit(int status);

int   abs(int n);
long  labs(long n);

#endif /* _MINZ_STDLIB_H */
