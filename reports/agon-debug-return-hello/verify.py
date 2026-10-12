from pathlib import Path
import re
root = Path(__file__).resolve().parent
text = re.sub(r'\x1b\[[0-9;]*m', '', (root/'debugger.txt').read_text())
rows = re.findall(r'PC=([0-9a-f]+).*?HL:([0-9a-f]+).*?SPL:([0-9a-f]+).*?IX:([0-9a-f]+).*?IY:([0-9a-f]+)', text)
assert len(rows) == 3, rows
entry, before, after = rows
assert entry[0] == '040045' and before[0] == '040051'
assert entry[2:] == before[2:], (entry, before)
assert before[1] == after[1] == '000000'
assert after[3:] == entry[3:]
assert int(after[2], 16) == int(entry[2], 16) + 3
# Decode the saved ADL return from independently dumped caller stack bytes.
dumps = re.findall(r'0bfe66: ([0-9a-f ]+) \|', text)
assert len(dumps) == 2 and dumps[0] == dumps[1]
return_address = int.from_bytes(bytes.fromhex(dumps[0])[:3], 'little')
assert int(after[0], 16) == return_address
assert text.count('0bfe76: 00 04 51 1d 00 00 00 04') == 2
print('Hello exact MOS return, SP/IY/IX and 24-byte caller frame PASS')
