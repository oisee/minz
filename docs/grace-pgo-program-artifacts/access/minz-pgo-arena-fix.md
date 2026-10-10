Fixed u16 address/destination overlap for IX, IY, HL, DE, and BC, preserving AF during scratch use. Added 75 assembled regression cases; all `pkg/mir2` tests pass. No commits.

Arena now passes `test_alloc_sequence`. A distinct residual remains: `test_oom` returns 3 instead of 0, caused by byte-to-word promotion destroying B.

Details: the preserved backend tests.
Current parent fix also corrects high-byte promotion; all 15 arena calls now pass. Historical residual above is superseded by verified-tests.txt.
