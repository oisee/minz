#!/usr/bin/env python3
"""Exact six-bank LZ storage candidate; not yet a runtime story-memory backend.

Mutable dynamic prefix stays at logical address0. Independent static blocks
use raw fallback. Four-byte descriptors: bank bits, LE offset, raw flag.
Decoded length follows from block index and story length. Trusted LZ archive
must reside in a nonzero bank. Largest-first best-fit placement is deterministic.
"""
import argparse
import hashlib
import json
from pathlib import Path
from functools import lru_cache
from restart_lz import pack as encode, unpack

pack = lru_cache(maxsize=2048)(encode)

BANK_SIZE = 16384
PAGES = [0, 1, 3, 4, 6, 7]


def build(story, block_size, reserve_bytes=0, reserve_page=5, strategy=0):
    if len(story) < 64 or story[0] != 3 or len(story) > 98304:
        raise ValueError('expected V3 story, 64..98304 bytes')
    dynamic = int.from_bytes(story[14:16], 'big')
    if not 64 <= dynamic <= min(len(story), BANK_SIZE):
        raise ValueError('dynamic prefix must fit bank0')
    if block_size not in (256, 512, 1024, 2048, 4096):
        raise ValueError('unsupported cache block size')
    blocks = []
    for begin in range(dynamic, len(story), block_size):
        raw = story[begin:begin + block_size]
        encoded = pack(raw)
        assert unpack(encoded, len(raw)) == raw
        compressed = len(encoded) < len(raw)
        blocks.append({'name': f'block{len(blocks)}', 'begin': begin,
                       'decoded_bytes': len(raw), 'raw': not compressed,
                       'data': encoded if compressed else raw})
    archive = pack(story[:dynamic])
    assert unpack(archive, dynamic) == story[:dynamic]
    # Store the descriptor table in the same six-bank budget.
    index = {'name': 'index', 'data': bytes(4 * len(blocks))}
    objects = blocks + [{'name': 'restart', 'data': archive}, index]
    if not 0 <= reserve_bytes <= BANK_SIZE or not 1 <= reserve_page <= 5:
        raise ValueError('invalid save reservation')
    limits = [BANK_SIZE] * 6
    limits[reserve_page] -= reserve_bytes
    used = [dynamic, 0, 0, 0, 0, 0]
    image = bytearray(98304)
    image[:dynamic] = story[:dynamic]
    for obj in sorted(objects, key=lambda o: (-len(o['data']), o['name'])):
        size = len(obj['data'])
        eligible = [i for i in range(6) if limits[i] - used[i] >= size
                    and not (obj['name'] == 'restart' and i == 0)]
        if not eligible:
            raise ValueError(f"cannot place {obj['name']} ({size} bytes) in one bank")
        if strategy == 0:
            page = min(eligible, key=lambda i: (limits[i] - used[i] - size, i))
        else:
            page = min(eligible, key=lambda i: ((i - strategy + 1) % 6))
        obj['page'], obj['offset'] = page, used[page]
        at = page * BANK_SIZE + used[page]
        image[at:at + size] = obj['data']
        used[page] += size
    table = bytearray()
    for obj in blocks:
        table.extend(bytes([PAGES[obj['page']] + 0x10]))
        table.extend(obj['offset'].to_bytes(2, 'little'))
        table.append(int(obj['raw']))
    at = index['page'] * BANK_SIZE + index['offset']
    image[at:at + len(table)] = table
    descriptors = []
    for obj in objects:
        descriptors.append({k: v for k, v in obj.items() if k != 'data'} |
                           {'stored_bytes': len(obj['data']),
                            'physical_bank': PAGES[obj['page']]})
    metadata = {'story_sha256': hashlib.sha256(story).hexdigest(),
                'story_bytes': len(story), 'dynamic_bytes': dynamic,
                'block_bytes': block_size, 'index_bytes': len(table),
                'restart_bytes': len(archive), 'used_bytes': sum(used),
                'placement_strategy': strategy,
                'reserved_save_bytes': reserve_bytes,
                'reserved_save_bank': PAGES[reserve_page],
                'reserved_save_offset': limits[reserve_page],
                'banks': [{'bank': PAGES[i], 'used_bytes': n,
                           'tail_bytes': BANK_SIZE - n} for i, n in enumerate(used)],
                'largest_nonzero_tail_bytes': max(BANK_SIZE - n for n in used[1:]),
                'objects': descriptors,
                'scope': 'exact host placement and reconstruction; no native cache, restart or SAVE integration'}
    assert reconstruct(bytes(image), metadata) == story
    return bytes(image), metadata


def reconstruct(image, metadata):
    dynamic = metadata['dynamic_bytes']
    result = bytearray(image[:dynamic])
    objects = metadata['objects']
    index = next(o for o in objects if o['name'] == 'index')
    at = index['page'] * BANK_SIZE + index['offset']
    table = image[at:at + index['stored_bytes']]
    for i, obj in enumerate(o for o in objects if o['name'].startswith('block')):
        entry = table[i * 4:i * 4 + 4]
        page = PAGES.index(entry[0] - 0x10)
        offset = int.from_bytes(entry[1:3], 'little')
        start = page * BANK_SIZE + offset
        data = image[start:start + obj['stored_bytes']]
        result.extend(data if entry[3] else unpack(data, obj['decoded_bytes']))
    archive = next(o for o in objects if o['name'] == 'restart')
    at = archive['page'] * BANK_SIZE + archive['offset']
    assert unpack(image[at:at + archive['stored_bytes']], dynamic) == result[:dynamic]
    return bytes(result)


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('story', type=Path)
    p.add_argument('output', type=Path)
    a = p.parse_args()
    a.output.mkdir(parents=True, exist_ok=True)
    story = a.story.read_bytes()
    summary = []
    for size in (256, 512, 1024, 2048, 4096):
        try:
            last_error = None
            for strategy in range(7):
                try:
                    image, metadata = build(story, size, strategy=strategy)
                    break
                except ValueError as e:
                    last_error = e
            else:
                raise last_error
            # Try to keep all otherwise free space in one bank. This heuristic
            # may fail even when a different packing exists; never claim optimal.
            free = 98304 - metadata['used_bytes']
            for reserved in sorted({free, *[n for n in (3072, 2048, 1536, 1271, 1024, 768, 512, 256) if n <= free]}, reverse=True):
                found = False
                for attempt in range(35):
                    page = (5, 4, 3, 2, 1)[attempt // 7]
                    strategy = attempt % 7
                    try:
                        image, metadata = build(story, size, reserved, page, strategy)
                        found = True
                        break
                    except ValueError:
                        pass
                if found:
                    break
        except ValueError as e:
            summary.append({'block_bytes': size, 'fits': False, 'reason': str(e), 'scope': 'no placement found by tested heuristics; not an impossibility proof'})
            continue
        (a.output / f'layout-{size}.bin').write_bytes(image)
        (a.output / f'layout-{size}.json').write_text(json.dumps(metadata, indent=2) + '\n')
        summary.append({'block_bytes': size, 'fits': True,
                        'used_bytes': metadata['used_bytes'],
                        'largest_nonzero_tail_bytes': metadata['largest_nonzero_tail_bytes'],
                        'reserved_save_bytes': metadata['reserved_save_bytes'],
                        'reserved_save_bank': metadata['reserved_save_bank']})
    (a.output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary, indent=2))
