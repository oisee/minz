#line 0 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/wrapper.c"
#line 0 "<built-in>"
#line 0 "<command-line>"
#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/wrapper.c"
unsigned short regression_failures, regression_checks, result;
#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c"




#line 1 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/testfwk.h"
#line 6 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c"


#pragma std_c99



static int a[199];

static void
foo ()
{
  int i;
  for (i = 198; i >= 0; i--)
    a[i] = i;
}


void
testTortureExecute (void)
{

  int i;
  foo ();
  for (i = 0; i < 199; i++)
    if (a[i] != i)
      do { ++regression_checks; if (!(0)) ++regression_failures; } while (0);

  return;
}
#line 3 "/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/gcc-torture-execute-loop-11/wrapper.c"
void execute(void) { regression_failures=0; regression_checks=0; testTortureExecute(); result=regression_failures ? 65535 : regression_checks; }
