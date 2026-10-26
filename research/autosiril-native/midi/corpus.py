"""Wire fixtures and independent resource/error contracts, no target parser."""
import struct


def vlq(value):
    assert 0 <= value <= 0xfffffff
    parts = [value & 127]
    while value > 127:
        value >>= 7
        parts.append(128 | (value & 127))
    return bytes(reversed(parts))


def wire(events, eot=0):
    data = b''.join(vlq(delta) + bytes(body) for delta, body in events)
    return data + vlq(eot) + b'\xff\x2f\x00'


def smf(tracks, ppqn=8, extra=b''):
    return b'MThd' + struct.pack('>IHHH', 6 + len(extra), int(len(tracks) > 1), len(tracks), ppqn) + extra + b''.join(
        b'MTrk' + struct.pack('>I', len(track)) + track for track in tracks)


def n(start, off, pitch=60, source=0):
    return dict(start=start, off=off, pitch=pitch, source=source)


def fixtures():
    valid = []

    def add(name, tracks, ppqn=8, beat=8, peak=1, extra=b''):
        # tracks are (event tuples, final EOT delta), so the wire writer also
        # gives an independent publication count without decoding the file.
        valid.append(dict(name=name, midi=smf([wire(events, eot) for events, eot in tracks], ppqn, extra),
                          per_beat=beat, chunks=[1, 7, 256], peak=peak,
                          events=sum(len(events) + 1 for events, _ in tracks)))

    add('empty', [([], 0)], peak=0)
    add('single', [([(0, [144, 60, 100]), (7, [128, 60, 0])], 0)])
    add('duplicate_fifo', [([(0, [144, 60, 100]), (1, [144, 60, 100]),
                             (1, [128, 60, 0]), (3, [128, 60, 0])], 0)], peak=2)
    add('channel_isolation', [([(0, [144, 60, 100]), (1, [145, 60, 100]),
                                (1, [129, 60, 0]), (1, [128, 60, 0])], 0)], peak=2)
    add('compaction_preserves_fifo', [([(0, [144, 60, 100]), (0, [144, 61, 100]),
                                       (1, [144, 60, 100]), (1, [128, 60, 0]),
                                       (1, [144, 60, 100]), (1, [128, 60, 0]),
                                       (1, [128, 60, 0]), (0, [128, 61, 0])], 0)], peak=3)
    add('velocity_zero_and_unknown_off', [([(0, [128, 70, 0]), (0, [144, 60, 100]),
                                           (3, [144, 60, 0]), (0, [144, 70, 0])], 0)])
    add('running_status', [([(0, [144, 60, 100]), (1, [64, 110]),
                             (1, [128, 60, 0]), (1, [64, 0])], 0)], peak=2)
    add('source_ids_skip_empty_tracks', [([], 0), ([(0, [144, 60, 100]), (1, [128, 60, 0])], 0),
                                        ([], 0), ([(0, [144, 67, 100]), (2, [128, 67, 0])], 0)])
    add('unmatched_eot_inherited_delta', [([(0, [144, 60, 100]), (1, [144, 64, 100])], 3)], peak=2)
    add('round_half', [([(29, [144, 60, 100]), (1, [128, 60, 0])], 0)], ppqn=480)
    add('one_row_per_beat', [([(16383, [144, 60, 100]), (1, [128, 60, 0])], 0)], ppqn=32767, beat=1)
    add('header_extra_and_short_messages', [([(0, [192, 4]), (0, [208, 30]),
                                             (0, [144, 60, 100]), (1, [128, 60, 0])], 0)], extra=b'\x12\x34')
    add('payload_exact_512', [([(0, [255, 1, *vlq(512), *([65] * 512)])], 0)], peak=0)
    last = (65536 * 32767 - 1 - 32767 // 2) // 256
    add('last_u16_row', [([(last, [144, 127, 100]), (0, [128, 127, 0])], 0)], ppqn=32767, beat=256)
    add('active_exact_512', [([(0, [144, 60, 100])] * 512, 0)], peak=512)
    add('capacity_exact_2046', [([event for _ in range(2046) for event in
                                 [(1, [144, 60, 100]), (1, [128, 60, 0])]], 0)])
    add('source_ids_exact_64', [([(0, [144, 60 + i % 12, 100]), (1, [128, 60 + i % 12, 0])], 0) for i in range(64)])

    negative = []

    def bad(name, midi, fault, notes=(), sources=0, active=0, peak=0, accepted=None, events=None, beat=8):
        expected = dict(fault=fault, notes=list(notes), sources=sources, active=active, peak_active=peak)
        if accepted is not None:
            expected['accepted_bytes'] = accepted
        if events is not None:
            expected['events'] = events
        negative.append(dict(name=name, midi=midi, per_beat=beat, chunks=[1, 7, 256], expected=expected))

    empty = smf([wire([])])
    bad('bad_magic', b'MThX' + empty[4:], 301, accepted=4, events=0)
    bad('truncated_eot', empty[:-1], 310, accepted=len(empty) - 1, events=0)
    bad('bad_data_byte', smf([wire([(0, [144, 128, 100])])]), 306, accepted=25, events=0)
    bad('missing_running_status', smf([wire([(0, [60, 100])])]), 305, accepted=24, events=0)
    bad('five_byte_vlq', smf([b'\x81\x81\x81\x81\x00\xff\x2f\x00']), 307, accepted=26, events=0)
    bad('payload_513', smf([wire([(0, [255, 1, *vlq(513), *([65] * 513)])])]), 308, accepted=27, events=0)
    bad('trailing_byte', empty + b'\x00', 312, accepted=len(empty), events=1)
    bad('bad_eot_length', smf([b'\x00\xff\x2f\x01\x00']), 309, accepted=26, events=0)
    bad('beat_zero', empty, 208, accepted=0, events=0, beat=0)
    bad('beat_257', empty, 208, accepted=0, events=0, beat=257)
    bad('active_513', smf([wire([(0, [144, 60, 100])] * 513)]), 205,
        notes=[n(0, 0)] * 512, sources=1, active=512, peak=512, accepted=22 + 513 * 4, events=513)
    events = [event for _ in range(2047) for event in [(1, [144, 60, 100]), (1, [128, 60, 0])]]
    bad('capacity_2047', smf([wire(events)]), 204,
        notes=[n(2 * i + 1, 2 * i + 2) for i in range(2046)], sources=1, peak=1,
        accepted=22 + 2046 * 8 + 4, events=4093)
    bad('row_first_invalid', smf([wire([(last + 1, [144, 60, 100])])], ppqn=32767), 202,
        accepted=22 + len(vlq(last + 1)) + 3, events=1, beat=256)
    bad('release_row_overflow', smf([wire([(last, [144, 60, 100]), (1, [128, 60, 0])])], ppqn=32767), 202,
        notes=[n(65535, 0)], sources=1, active=1, peak=1,
        accepted=22 + len(vlq(last)) + 7, events=2, beat=256)
    bad('quantizer_wrap_prevented', smf([wire([(1 << 24, [144, 60, 100])])], ppqn=1), 202,
        accepted=22 + len(vlq(1 << 24)) + 3, events=1, beat=256)
    bad('track_65', smf([wire([])] * 65), 203, accepted=14 + 65 * 12, events=65)
    many_tracks = [wire([(0, [144, 60, 100]), (1, [128, 60, 0])]) for _ in range(65)]
    bad('source_track_65', smf(many_tracks), 203, notes=[n(0, 1, source=i) for i in range(64)],
        sources=64, peak=1, accepted=14 + 64 * 20 + 12, events=193)
    partial = smf([wire([(0, [144, 60, 100]), (0, [144, 64, 100])], eot=22000)])
    bad('eot_partial_row_overflow', partial, 202, notes=[n(0, 44000), n(0, 0, 64)],
        sources=1, active=2, peak=2, accepted=len(partial), events=3)
    synthetic = smf([wire([(0, [144, 60, 100])] + [(0xfffffff, [255, 1, 0])] * 15,
                          eot=0xfffffff)], ppqn=32767)
    bad('synthetic_eot_tick_overflow', synthetic, 207, notes=[n(0, 0)],
        sources=1, active=1, peak=1, accepted=len(synthetic), events=17, beat=1)
    # Decoder overflow occurs at the delta, before publishing its event.
    previous = [(0xfffffff, [255, 1, 0])] * 16
    bad('decoder_tick_overflow', smf([wire(previous + [(16, [255, 1, 0])])]), 311,
        accepted=22 + 16 * 7 + 1, events=16)
    return valid, negative
