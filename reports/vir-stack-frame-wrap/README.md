# Dense adapter frame across 16-bit address wrap

Nine SP placements0,1,2,23,24,25,0x4000,0xfffe,0xffff × all256 flag/input patterns:2304 actual assembled Z80 executions PASS. Frame writes/read addresses wrap modulo65536. All exposed registers/flags, exactPC/SP, every observed memory write and final all64K bytes outside the modular24-byte frame are checked, including return address/code. ROM protection disabled via existing screen-platform setter; execution uses the embedded plain CPU directly without screen/RST dispatch.

This extends the prior one-SP proof. No stack capacity, interrupts, arbitrary source maps or full six-argument Cartesian claim. Compiler behavior unchanged; permanent native boundary evidence added. Production cbf01110 and research c1744a23 are live older checkpoints and were not restarted.
