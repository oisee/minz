"""Explicit malformed wire/resource contracts, not a target parser oracle."""
import base64
import struct


def encode(notes, sources):
    data = b'ASIR' + bytes([1, 1, 6, 0]) + struct.pack('<HH', sources, 0)
    data += b''.join(b'\x01' + struct.pack('<HHBB', n['start'], n['off'], n['pitch'], n['source']) for n in notes)
    data += b'\xff' + struct.pack('<I', len(notes)) + b'END!'
    return data + bytes((-len(data)) % 128)


def interval(start=0, off=1, pitch=60, source=0):
    return dict(start=start, off=off, pitch=pitch, source=source)


def resources():
    return [
        dict(name='output_exact_1024', notes=[interval(i * 2, i * 2 + 1) for i in range(512)], sources=1),
        dict(name='output_over_1024', notes=[interval(i * 2, i * 2 + 1) for i in range(513)], sources=1),
        dict(name='first_bank2_record', notes=[interval(0, 0, source=2)] * 1364 + [interval(65534, 65535, 127)], sources=3),
        dict(name='last_bank2_record', notes=[interval(0, 0, source=2)] * 2045 + [interval(65534, 65535, 127)], sources=3),
        dict(name='active_513', notes=[interval()] * 513, sources=1, fault=108),
    ]


def malformed():
    note = interval(off=7)
    single, empty = encode([note], 1), encode([], 0)
    result = []

    def add(name, data, fault, count=0, no_output=True, os_fault=None):
        c = dict(name=name, wire=base64.b64encode(data).decode(), fault=fault, count=count, no_output=no_output)
        if os_fault:
            c['os_fault'] = os_fault
        result.append(c)

    def mutation(name, offset, value, fault, count=0):
        data = bytearray(single)
        data[offset] = value
        add(name, bytes(data), fault, count)

    for name, offset, value, fault in [
        ('bad_magic', 0, 0, 950), ('unknown_version', 4, 2, 952),
        ('wrong_kind', 5, 2, 953), ('wrong_width', 6, 5, 953),
        ('unknown_flags', 7, 1, 953), ('reserved_nonzero', 10, 1, 953),
        ('sources_65', 8, 65, 954), ('unknown_tag', 12, 2, 955),
        ('pitch_128', 17, 128, 958), ('source_outside_header', 18, 1, 958),
    ]:
        mutation(name, offset, value, fault)
    mutation('count_mismatch', 20, 2, 956, 1)
    mutation('count_high_word', 22, 1, 956, 1)
    mutation('end_signature', 27, 0, 956, 1)
    mutation('nonzero_padding', 28, 1, 957, 1)
    add('missing_empty_end', empty[:12] + bytes(116), 955)
    add('missing_empty_count_and_signature', empty[:13] + bytes(115), 956)
    add('extra_physical_record', single + bytes(128), 957, 1)
    # CP/M fills a short host record with 1a. These expected errors follow the
    # explicit replacement bytes, not a target-generated classification.
    for length, fault, count in [
        (0, 951, 0), (2, 950, 0), (4, 952, 0), (5, 953, 0),
        (12, 955, 0), (13, 958, 0), (18, 958, 0), (19, 955, 1),
        (20, 956, 1), (24, 956, 1), (27, 956, 1), (28, 957, 1),
    ]:
        add('truncated_host_' + str(length), single[:length], fault, count)
    add('physical_eof_inside_record', encode([note] * 20, 1)[:128], 951, 16)
    add('note_2047', encode([interval(0, 0, source=2)] * 2047, 3), 959, 2046)
    add('reversed_interval', encode([interval(8, 7)], 1), 105, 1)
    add('unsorted_source', encode([interval(10, 11), interval(2, 3)], 1), 106, 2)
    add('note_with_zero_sources', encode([note], 0), 958)
    for name, fn, occurrence, fault, count, no_output in [
        ('open_failure', 15, 1, 900, 0, True),
        ('read_failure', 20, 1, 904, 0, True),
        ('eof_probe_io_failure', 20, 2, 904, 1, True),
        ('input_close_failure', 16, 1, 906, 1, True),
        ('create_failure', 22, 1, 901, 1, True),
        ('write_failure', 21, 1, 902, 1, False),
        ('output_close_failure', 16, 2, 903, 1, False),
    ]:
        add(name, single, fault, count, no_output, dict(function=fn, occurrence=occurrence))
    return result
