#ifndef PILOT_TESTFWK_H
#define PILOT_TESTFWK_H
#define ASSERT(cond) do { ++regression_checks; if (!(cond)) ++regression_failures; } while (0)
#endif
