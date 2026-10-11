# Original directory caller after all three correctness repairs

At a3e0fe29, original write_dir_entry native/raw/final VM 18 cases PASS with exact disk_write ABI, 512-byte cached sector, full 64 KiB state/code and balanced SP. Default: 13574 T / 9694 B. GraceSize: 13574 T / 9871 B. GraceSpeed: 10948 T / 9887 B. The three correctness fixes preserve these caller T-states; whole-module binary sizes change because other functions are now correctly generated. Historical f5c7a09e directory artifacts remain untouched. This is not a controlled measured selector or new heldout workload claim.
