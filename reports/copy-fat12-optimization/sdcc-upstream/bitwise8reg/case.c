#line 0 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/wrapper.c"
#line 0 "<built-in>"
#line 0 "<command-line>"
#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/wrapper.c"
unsigned short regression_failures, regression_checks, result;
#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c"





#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/testfwk.h"
#line 7 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/bitwise8reg.c"

unsigned char c0, c1, c2, c3;

unsigned char and8(void)
{
 unsigned char t0, t1, t2, t3, t4, t5;

 t0 = c0 + 1;
 t1 = c1 + 1;
 t2 = c2 + 1;
 t3 = c3 + 1;

 t4 = t0 & t1;
 t5 = t2 & t3;

 return(t4 + t5);
}

unsigned char or8(void)
{
 unsigned char t0, t1, t2, t3, t4, t5;

 t0 = c0 + 1;
 t1 = c1 + 1;
 t2 = c2 + 1;
 t3 = c3 + 1;

 t4 = t0 | t1;
 t5 = t2 | t3;

 return(t4 + t5);
}

void
test8(void)
{
 c0 = 2;
 c1 = 2;
 c2 = 4;
 c3 = 4;

 do { ++regression_checks; if (!(and8() == 8)) ++regression_failures; } while (0);
 do { ++regression_checks; if (!(or8() == 8)) ++regression_failures; } while (0);
}
#line 3 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/bitwise8reg/wrapper.c"
void execute(void) { regression_failures=0; regression_checks=0; test8(); result=regression_failures ? 65535 : regression_checks; }
