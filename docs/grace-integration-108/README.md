# Shared engines preserved; third upstream correction

Root31320033 shared rewrite datalog/grace/isle/sexpr, Z80 assembler and asmutil PASS; rootrewrite package has no tests. This verifies retained consumers after LIR removal; it is not a native whole-corpus gate.

Third standalone correction against unchangedmain4743: draftPR https://github.com/oisee/minz/pull/73, branchfix/upstream-bitcmp-admission head22c4c2e0. Before scanner removes matchedAND/shift/zero for unencodableBIToperand and native IX=1 can return0. Physical encoding guard preserves definitions for fallback. Independent3072native IX/IY/spill ×direct/shift ×Eq/Ne ×all256 cases pass plusfullMIR2/nativeABI smoke. Main remains unchanged, researchbranch already contains this productionfix13d2acc0. Separate upstream artifacts are on agent-artifacts/upstream-bitcmp-admission-22c4c2e0; no researchcherry-pick needed.

Prior drafts71(registeridentity) and72(unsigneddiv workspace) remain separate reviewable branches. FullCLId44c newrun remains live after64PASStests; old59db71/71PASS remains preserved. Fullpipeline8bf live. Sitesv65 published; fullwholecopy live networkI/O with verifiedprogress, finalmanifest notyetclaimed. Overallgoalactive.
