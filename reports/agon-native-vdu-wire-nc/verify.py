from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts"))
from agon_native_vdu_model import expected
root = Path(__file__).resolve().parent
wire = (root / "wire.bin").read_bytes()
boot = b"\n\r\n\r\x07"
start = wire.index(boot) + len(boot)
end = wire.index(b"MINZ_FULL_RETURNED\r\n/ *", start)
assert wire.count(b"Agon Console8 MOS Version") == 1
assert wire.count(b"MINZ_FULL_RETURNED") == 1
assert wire[start:end] == expected("nc_agon", (10,)*9+(13,11,13,27)) == (root / "expected.bin").read_bytes()
print("Exact native VDU NC navigation PASS:", end-start, "bytes")
