# Nanz wide global initializers

Previously scalar u24/u32 literals returned no initializer bytes; array elements of those widths emitted only one byte. The source declaration value:u32=4294967294 therefore became zero in both VM and native code. Independent source-byte expectations reproduce the loss.

parseInitializer now serializes each byte of declared scalar/element width little-endian. Regressions cover scalar u24/u32 and two-element arrays, including maximum unsigned values. FullNanz passes. Wide native memory/caller behavior is being verified separately; this change corrects frontend data, not allu32 operations. Nonliteral initializer semantics are unchanged.
