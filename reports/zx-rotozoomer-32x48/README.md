# ZX Spectrum 32×48 colour rotozoomer

Alice's half-cell display works: the fixed bitmap has four zero rows above four FF rows in every8×8cell. PAPER colours upper8×4, INK lower8×4; sharedBRIGHT1 FLASH0. Only768attribute bytes change perframe. Nanz uses8.8texturecoordinates and addition stepping; algorithm reference antique-toy chapter7 (HEAD7313ecfaab710d1a105b7bff3be19aaf04dc5935).

All32frames PASS in ordinary, Grace Size and Grace Speed: raw/finalVM and assembledZ80 agree with independentmodel for everyattribute; fixedbitmap and immutablecode/data remain unchanged, exactlydeclaredspilldata exempted. Calls return toPC7000/SPeff2. No host graphics callback executes. Nativepreview.gif/png decode actual screen dumps; modelpreviews are separate expectations.

Baseline1291bytes; frame0 2451932T, about1.4fps at3.5MHz beforeULAcontention. Correctnessbaseline, not yetfastdemo. Main animates with EI/HALT using standard48KROM interrupts. Nativeframecheck calls draw_frame directly; complete ROM pacing/main loop/hardware not validated yet. Load compactrotozoomer.bin at8000 and enter8000. A runnable emulator/TAP wrapper and faster innerloop remain next work.

The test found a backendbug: codegen's postallocation relocation could leave suppressedconstant1 in an uninitializedspillhome, so pointer+1 didn'tadvance. Retainedbefore image/log showfirstattribute overwritten by laterpixels. CorrectedwordADD/SUB1 usesdirectINC/DEC afterrelocation;16nativecases coverregister/spill/IX/IY andwrapboundaries; fullMIR2PASS67.108s.

Replay: fromminzc, MINZ_ZX_ROTO_EXPORT=/absolute/output go test ./pkg/pipeline -run '^TestZXHalfCellRotozoomerNative$' -count=1 -v. Beforeandafter binaries and screen dumps remain separate. No physicalhardware orULAcontention claim.
