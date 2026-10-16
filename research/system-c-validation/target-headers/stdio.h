/* Target declarations only. File APIs are not part of this validation. */
#ifndef SYSTEM_C_VALIDATION_STDIO_H
#define SYSTEM_C_VALIDATION_STDIO_H
#include <stddef.h>
typedef struct { int unavailable; } FILE;
FILE *fopen(const char *, const char *);
int fclose(FILE *);
char *fgets(char *, int, FILE *);
#endif
