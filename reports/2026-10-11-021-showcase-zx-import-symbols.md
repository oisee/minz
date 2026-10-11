# ZX inline ASM symbols and explicit stdlib imports

Corrected stdlib/zx/console.nanz inline ASM references to match the actual
imported global labels zx__console__con_attr/col/row. hello_zx and
console_demo now pass the complete showcase compile/assemble path.

Import resolution now accepts stdlib.name against the configured StdlibDir
without searching stdlib/stdlib/name. A small imported main returns 7 and
passes independent MIR/assembled-Z80 assertions. The old Agon hello_world
now finds mos.minz and exposes its legacy syntax instead of module-not-found;
that separate frontend compatibility issue is not declared resolved.

Opt-in MINZ_SHOWCASE_ARTIFACTS retains exact ASM before assembly failures.
No knownFailures entries or checks were relaxed. Other agents are repairing
frontends, boolean predicates and inline-ASM reachability in isolated branches.
