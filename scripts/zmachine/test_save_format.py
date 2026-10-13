#!/usr/bin/env python3
"""Independent boundary/corruption tests for the RAM-save reference format."""
import struct
import unittest
from save_format import encode, decode, crc16


class SaveFormatTest(unittest.TestCase):
    def setUp(self):
        self.story = bytearray(1024)
        self.story[0] = 3
        self.story[2:4] = b'\x00\x01'
        self.story[14:16] = b'\x01\x00'
        self.story[18:24] = b'260101'
        self.story[28:30] = b'\x12\x34'
        self.state = bytearray(1786)
        self.state[:16] = b'MZ3S\x00\x01260101\x12\x34\x01\x00'
        self.state[16:19] = (128).to_bytes(3, 'little')
        self.state[22:26] = b'\xcd\xab\x01\x01'
        self.state[26] = 64
        self.memory = bytearray(self.story[:256])
        self.memory[100] = 37
        self.record = encode(self.story, self.state, self.memory)

    def test_roundtrip_preserves_rng_stream_tables_and_dynamic(self):
        self.assertEqual(decode(self.record, self.story), (self.state, self.memory))
        self.assertEqual(crc16(b'123456789'), 0x29b1)

    def test_each_single_bit_corruption_rejected(self):
        for at in range(len(self.record)):
            for bit in range(8):
                data = bytearray(self.record)
                data[at] ^= 1 << bit
                with self.assertRaises(ValueError):
                    decode(data, self.story)

    def test_every_truncation_and_trailing_input_rejected(self):
        for cut in range(len(self.record)):
            with self.assertRaises(ValueError):
                decode(self.record[:cut], self.story)
        with self.assertRaises(ValueError):
            decode(self.record + b'\0', self.story)

    def test_foreign_story_configuration_and_capacity(self):
        for at in [2, 18, 28]:
            story = bytearray(self.story)
            story[at] ^= 1
            with self.assertRaises(ValueError):
                decode(self.record, story)
        for options in [dict(frames=16), dict(stack_words=128), dict(capacity=len(self.record)-1)]:
            with self.assertRaises(ValueError):
                decode(self.record, self.story, **options)
        with self.assertRaises(ValueError):
            encode(self.story, self.state, self.memory, capacity=len(self.record)-1)

    def test_semantic_state_rejection_before_encoding(self):
        for offset, value in [(16, 0), (19, 32), (21, 2), (24, 2), (25, 17), (26, 255), (42, 1)]:
            state = bytearray(self.state)
            state[offset] = value
            with self.assertRaises(ValueError):
                encode(self.story, state, self.memory)

    def test_valid_crc_does_not_hide_malformed_rle(self):
        data = bytearray(self.record)
        # First token becomes a repeat of 130, instead of a short literal.
        data[32] = 255
        data[30:32] = struct.pack('<H', crc16(data[:30] + data[32:]))
        with self.assertRaises(ValueError):
            decode(data, self.story)


if __name__ == '__main__':
    unittest.main()
