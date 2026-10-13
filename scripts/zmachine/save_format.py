#!/usr/bin/env python3
"""Reference for a bounded, volatile native RAM-save format (not native I/O).

32-byte header + RLE state + RLE(dynamic XOR immutable original).
Validate the whole record before returning anything suitable for application.
CRC-16/CCITT detects accidental corruption; it does not authenticate saves.
"""
import struct
from restart_rle import pack, unpack

HEADER = struct.Struct('<4sBBHHHHH10sIH')
assert HEADER.size == 32
MAGIC = b'MZ3R'
VERSION = 1


def crc16(data):
    crc = 0xffff
    for value in data:
        crc ^= value << 8
        for _ in range(8):
            crc = ((crc << 1) ^ (0x1021 if crc & 0x8000 else 0)) & 0xffff
    return crc


def geometry(story, frames, stack_words):
    if not 2 <= frames <= 128 or not 1 <= stack_words <= 2048:
        raise ValueError('invalid configured capacities')
    if not 64 <= len(story) <= 98304 or story[0] != 3:
        raise ValueError('not a supported V3 story size')
    dynamic = int.from_bytes(story[14:16], 'big')
    if not 64 <= dynamic <= min(len(story), 16384):
        raise ValueError('dynamic memory must fit bank0')
    state = 58 + frames * 38 + stack_words * 2
    identity = story[2:4] + story[18:24] + story[28:30]
    return dynamic, state, identity


def check_state(state, story, frames, stack_words):
    dynamic, size, identity = geometry(story, frames, stack_words)
    expected_identity = b'MZ3S' + identity[:8] + identity[8:] + story[14:16]
    if len(state) != size or state[:16] != expected_identity:
        raise ValueError('state identity or length mismatch')
    pc = int.from_bytes(state[16:19], 'little')
    frame = state[19]
    depth = int.from_bytes(state[20:22], 'little')
    if not 64 <= pc < len(story) or frame >= frames or depth > stack_words:
        raise ValueError('invalid PC/frame/stack depth')
    if state[24] > 1 or state[25] > 16:
        raise ValueError('invalid output stream state')
    for i in range(state[25]):
        table = state[26+i] | state[42+i] << 8
        if table < 64 or table + 2 > dynamic:
            raise ValueError('stream3 table outside writable memory')
    base = 58 + frames * 32
    last_depth = 0
    for i in range(1, frame + 1):
        return_pc = state[base+i] | state[base+frames+i] << 8 | state[base+2*frames+i] << 16
        return_depth = state[base+4*frames+i] | state[base+5*frames+i] << 8
        if not 64 <= return_pc < len(story) or not last_depth <= return_depth <= depth:
            raise ValueError('invalid active return frame')
        last_depth = return_depth


def encode(story, state, dynamic_memory, capacity=2204, frames=32, stack_words=256):
    dynamic, state_size, identity = geometry(story, frames, stack_words)
    check_state(state, story, frames, stack_words)
    if len(dynamic_memory) != dynamic:
        raise ValueError('dynamic memory length mismatch')
    if dynamic_memory[14:16] != story[14:16]:
        raise ValueError('dynamic memory boundary changed')
    first = pack(state)
    second = pack(bytes(a ^ b for a, b in zip(dynamic_memory, story[:dynamic])))
    header = HEADER.pack(MAGIC, VERSION, frames, stack_words, dynamic, state_size,
                         len(first), len(second), identity, len(story), 0)
    payload = first + second
    result = header[:30] + struct.pack('<H', crc16(header[:30] + payload)) + payload
    if len(result) > capacity:
        raise ValueError('RAM save capacity exceeded')
    return result


def decode(record, story, capacity=2204, frames=32, stack_words=256):
    dynamic, state_size, identity = geometry(story, frames, stack_words)
    if not HEADER.size <= len(record) <= capacity:
        raise ValueError('invalid record size')
    magic, version, saved_frames, saved_stack, saved_dynamic, saved_state, first, second, saved_identity, story_size, crc = HEADER.unpack_from(record)
    if (magic, version, saved_frames, saved_stack, saved_dynamic, saved_state,
        saved_identity, story_size) != (MAGIC, VERSION, frames, stack_words,
                                        dynamic, state_size, identity, len(story)):
        raise ValueError('format/story/capacity mismatch')
    if HEADER.size + first + second != len(record):
        raise ValueError('encoded lengths mismatch')
    if crc16(record[:30] + record[32:]) != crc:
        raise ValueError('corrupt record')
    state = unpack(record[32:32+first], state_size)
    delta = unpack(record[32+first:], dynamic)
    check_state(state, story, frames, stack_words)
    memory = bytes(a ^ b for a, b in zip(delta, story[:dynamic]))
    if memory[14:16] != story[14:16]:
        raise ValueError('dynamic memory boundary changed')
    return state, memory
