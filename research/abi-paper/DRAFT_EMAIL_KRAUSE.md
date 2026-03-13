# Draft Email to Philipp Klaus Krause

**To:** Philipp Klaus Krause (via ResearchGate, GitHub spth, or university email)
**Subject:** Per-function calling convention optimization — generalizing your 2022 work

---

Dear Dr. Krause,

I hope this message finds you well. I'm writing to share some results that directly build on your work on calling conventions for irregular architectures (arXiv:2112.01397) and optimal register allocation (CC 2013).

Your 2022 paper demonstrated that empirically searching for a better *global* calling convention yields significant improvements in SDCC — enough to justify breaking ABI compatibility. This result inspired us to ask: what if we go further and optimize the calling convention *per function*?

We formalized this as **Per-Function Calling Convention Optimization (PFCCO)**: given a call graph and a cost model over register classes, choose a convention for each internal function to minimize total transfer cost. The key results:

1. **O(n) optimal algorithm** for acyclic call graphs with bounded parameter counts, via greedy DP in reverse topological order. Your empirically-optimized global ABI is a special case where all functions receive the same assignment.

2. **Production implementation** in the Nanz compiler (Z80 target), showing 1.3–17x code size reduction over SDCC 4.2.0 on representative examples. The largest wins come from multi-return functions and iterator fusion with ClassCounter(B) enabling DJNZ.

3. **Emergent optimizations** we didn't program: zero-byte function aliases (`min_of EQU minmax`) and "ABI-as-implementation" degenerate functions where the calling convention *is* the function semantics (swap becomes bare RET).

4. **Cross-architecture validation** on the MOS 6502 — the same algorithm (zero code changes, different cost table) produces correct optimized conventions on a 3-register architecture. Verified by a 5-path round-trip oracle (MIR2 VM, C99, QBE, QBE-round-trip, 6502 emulator all agree).

I attached the draft paper (10 pages, SIGPLAN format). A few specific points where your expertise would be invaluable:

- **Is the optimality claim sound?** The proof relies on the standard bottom-up DP argument for DAGs, with a heuristic fallback for recursive functions.
- **Comparison fairness:** We compare against SDCC 4.2.0 with your optimized convention. We tried to be honest — noting that for simple leaf functions, SDCC's global ABI is near-optimal. Are we missing important cases where the global convention wins?
- **Venue suggestion:** Would CC, LCTES, or SCOPES be the right fit?

Your work on SDCC is remarkable — both the theoretical foundations and the practical impact on a real compiler used by thousands of developers. We consider our work a direct extension: you found the optimal *single* convention; we show that per-function conventions can go further, and that this generalizes across architectures.

I would be grateful for any feedback, even brief. If you're interested, I'm happy to share the implementation (open source, Go) or discuss potential collaboration.

Best regards,
[Name]

---

## Notes for ourselves

**Where to find him:**
- GitHub: https://github.com/spth (active, SDCC contributions)
- ResearchGate: https://www.researchgate.net/scientific-contributions/Philipp-Klaus-Krause-2048004142
- DBLP: https://dblp.org/pid/36/7988.html
- SDCC mailing list: active participant

**Attachment:** paper-v2.pdf

**Tone check:**
- [x] Respectful — "your work inspired us", "direct extension"
- [x] Concrete — specific results, not vague claims
- [x] Honest — acknowledging where SDCC is competitive
- [x] Specific ask — 3 concrete questions, not "what do you think?"
- [x] Not adversarial — "we generalize your result" not "we beat SDCC"
