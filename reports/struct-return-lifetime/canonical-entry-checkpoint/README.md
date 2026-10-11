Canonical entry follow-up to 00a674e8.

Canonical parameter classes now use one shared deterministic nonoverlapping slot allocator for address-taken definitions and indirect calls. It preserves a previous preferred slot when free, then assigns free HL/DE/IY word slots or A/C/B/E/D/L/H byte slots. Parameters wider than16bits or exhausted physical capacity explicitly reject. IX is reserved for the indirect target. Contract classes fix external slots; the body may allocate or spill freely behind the entry bridge.

The bridge runs once before the first block, not at a CFG jump target; unused inputs are omitted. Defined direct callers use canonical slots. FixedIndirectABI blocks contract optimization even after address-taking instructions disappear. Checked local tables explicitly reject these functions until their bridge, costs and function attributes are part of table admission. Ordinary tables remain unchanged.

Typed VM target checks reject same-width CY-error and same-sized differently named/layout struct targets. Zero-argument typed return signatures appear in MIR dump. Controls pass with loop/value-copy/native123+30 cases. This is still an intermediate general ABI implementation: independent multi-result overlapping-buffer/native proof and distinct multi-return register slots remain pending; no complete ABI claim.
