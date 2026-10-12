# Frozen heldout replay after latest backend/adapter fixes

Checkpoint1d56e49c: original frozen parser and hashmap protocols repeat without changes to training/heldout inputs or independent models. Terminal PASS22.468s, exit0. Parser heldout2195027→2137641T (−2.61%),2377→2478B; hashmap48 heldout calls258962→237948T (−8.11%),996→976B, both Speed/Size selectmask3. Tests require identical selectedBIN between train and heldout. Source/input/model/decision/ASM/BIN packets retained. This repeats known gains, not a new blind study or hardware result; Speed has no byte penalty.

The full production cbf01110, full research7bfe4476 and freshCLI1d56e49c remain separate live jobs. No fullcurrent PASS inferred from this focused replay. Lizp/PLM policy cases remain unresolved.
