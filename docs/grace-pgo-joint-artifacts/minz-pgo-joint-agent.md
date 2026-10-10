Implemented only `profile_joint.go` and `profile_joint_test.go`. API signature published at `/tmp/minz-pgo-joint-api.txt`; handoff at `/tmp/minz-pgo-joint-implementation.txt`.

All joint tests pass. Measured combinations cost 3476/3212/3212/2948 T-states and 107/156/156/205 bytes. Speed selects ptr/ptr; size selects orig/orig. Results and memory agree across all combinations.

The earlier workload-test failure now passes on focused rerun. No commits.