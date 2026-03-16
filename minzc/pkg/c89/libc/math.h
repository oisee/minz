/* MinZ minimal math.h for Z80 targets (stubs) */
#ifndef _MINZ_MATH_H
#define _MINZ_MATH_H

/* FatFS only uses these behind FF_PRINT_FLOAT which defaults to 0 */
double fabs(double x);
double fmod(double x, double y);
double pow(double base, double exp);
int    isnan(double x);
int    isinf(double x);

#endif /* _MINZ_MATH_H */
