#!/usr/bin/env python3
"""Offline VDP rendering of a retained native stream; not CPU/MOS execution.

Run with system Python to avoid a Conda libstdc++ overriding the VDP ABI.
"""
import argparse
import ctypes
import hashlib
import json
from pathlib import Path
import threading
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--vdp', type=Path, required=True)
    parser.add_argument('--stream', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--terminal', action='store_true')
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    lib = ctypes.CDLL(str(args.vdp.resolve()))
    lib.z80_send_to_vdp.argtypes = [ctypes.c_ubyte]
    lib.z80_uart0_is_cts.restype = ctypes.c_bool
    lib.set_startup_screen_mode(0)

    def worker():
        lib.vdp_setup()
        lib.vdp_loop()

    threading.Thread(target=worker, daemon=True).start()
    time.sleep(1)

    def send(data):
        for byte in data:
            deadline = time.monotonic() + 5
            while not lib.z80_uart0_is_cts():
                if time.monotonic() > deadline:
                    raise RuntimeError('VDP UART did not become ready')
                time.sleep(.001)
            lib.z80_send_to_vdp(byte)
            time.sleep(.001)

    prefix = b'\x17\x00\x80\x01'  # MOS general poll releases VDP boot wait.
    send(prefix)
    time.sleep(.2)
    if args.terminal:
        prefix += b'\x17\x00\xff'
        send(b'\x17\x00\xff')
        time.sleep(.2)
    data = args.stream.read_bytes()
    send(data)
    time.sleep(1)
    width, height, fps = ctypes.c_uint(), ctypes.c_uint(), ctypes.c_float()
    buffer = (ctypes.c_ubyte * (1024 * 768 * 3))()
    lib.copyVgaFramebuffer(ctypes.byref(width), ctypes.byref(height), buffer, ctypes.byref(fps))
    pixels = bytes(buffer)[:width.value * height.value * 3]
    (out / 'frame.ppm').write_bytes(f'P6\n{width.value} {height.value}\n255\n'.encode() + pixels)
    (out / 'stream.bin').write_bytes(data)
    (out / 'initialization.bin').write_bytes(prefix)
    result = {'vdp_path': str(args.vdp.resolve()), 'vdp_sha256': hashlib.sha256(args.vdp.read_bytes()).hexdigest(),
              'stream_path': str(args.stream.resolve()), 'stream_sha256': hashlib.sha256(data).hexdigest(),
              'terminal_mode_requested': args.terminal, 'width': width.value, 'height': height.value,
              'fps': fps.value, 'scope': 'offline retained native stream through full VDP library; no simultaneous CPU/MOS or independent pixel oracle'}
    (out / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
    lib.vdp_shutdown()
    print(json.dumps(result), flush=True)


if __name__ == '__main__':
    main()
