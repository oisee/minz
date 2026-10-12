# VIR same-pair follow-up: mixed backend callee corruption

Exports show unchanged MIR and differing native board_get/board_set when the experimental zero-cost same_pair pattern is enabled. scenario is PBQP fallback in both runs; it calls VIR-generated helpers. Bad board_get moves word board base DE into IXH using LD IXH,E, then reconstructs HL from only a byte. The failure is therefore not evidence that scenario PBQP alone loses the value.

Three additional probes rejected: skip tied OpMove in pre-tie pass; skip tied OpMove in save pass; constrain defined vregs by width in per-instruction SMT. Each leaves original scenario assertion0 instead16. The selected path and earlier transformed operations still require investigation; these changes do not prove the width issue resolved. All experimental compiler changes restored to HEAD semantics.

Actual before/after source, MIR, ASM and allocation retained. Existing word-return arithmetic PASS remains; DEC HL quality regression remains open. Both full immutable jobs still running, no restart.
