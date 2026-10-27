"""TRD-specific EOF/allocation/error contracts over independent Ruby vectors."""
from copy import deepcopy
import base64


def make_cases(reference, producer):
    valid_names = {c['name'] for c in producer['cases'] if c['expected']['fault'] == 0}
    assert len(valid_names) == 21
    cases = []
    for original in reference['cases']:
        c = deepcopy(original)
        if c.get('os_fault'):
            continue  # BDOS errors are replaced by actual TRD sector failures.
        name = c['name']
        wire = base64.b64decode(c['wire'])
        if name in valid_names or name in {
            'output_exact_1024', 'output_over_1024', 'first_bank2_record',
            'last_bank2_record', 'active_513', 'reversed_interval', 'unsorted_source',
        }:
            # Strip only the encoder's physical CP/M fill. The TRD directory
            # stores exact logical length; the target reader does the clipping.
            count = len(c['notes']) if 'notes' in c else c['count']
            wire = wire[:21 + 7 * count]
        if name in valid_names:
            c['disk'] = 'trd/measured/' + name + '/scorpion-z80/disk.trd'
        if name.startswith('truncated_host_'):
            length = int(name.rsplit('_', 1)[1])
            if length == 28:
                single = next(x for x in reference['cases'] if x['name'] == 'single')
                c.update(deepcopy(single), name='exact_short_logical_file', disk='')
                wire = wire[:28]
            else:
                c.update(fault=951, count=1 if length >= 19 else 0)
        c['wire'] = base64.b64encode(wire).decode()
        c.pop('no_output', None)
        cases.append(c)
    single = deepcopy(next(c for c in cases if c['name'] == 'single'))
    chronos = deepcopy(next(c for c in cases if c['name'] == 'chronos_full'))
    for call, fault in enumerate([900, 900, 904, 902, 903, 903, 903, 903], 1):
        c = deepcopy(single)
        c.update(name='sector_failure_' + str(call), fail_call=call, fault=fault,
                 count=0 if call <= 3 else 1, published=call >= 7, disk_error=2,
                 data_writes=0 if call <= 4 else 1)
        if call <= 3:
            c.pop('notes', None)
        cases.append(c)
    for control, error, parsed, fault in [
        ('missing_input', 7, False, 900), ('existing_output', 5, False, 900),
        ('duplicate_input', 6, False, 900), ('bad_geometry', 3, False, 900),
        ('bad_directory_count', 3, False, 900), ('directory_count_mismatch', 8, False, 900),
        ('bad_source_extent', 4, False, 900), ('source_length_under', 0, True, 951),
        ('source_length_over', 0, True, 957), ('source_length_exceeds_extent', 4, False, 900),
        ('free_overlap', 8, False, 900), ('no_space', 9, True, 901),
        ('directory_full', 9, True, 901), ('last_sector', 0, True, 0),
        ('full_disk', 9, True, 901), ('bad_full_sentinel', 3, False, 900),
        ('directory_slot_15', 0, True, 0), ('directory_slot_16', 0, True, 0),
        ('directory_slot_127', 0, True, 0),
    ]:
        c = deepcopy(single)
        c.update(name=control, disk_control=control, disk_error=error, fault=fault,
                 count=1 if parsed else 0, published=control == 'existing_output',
                 data_writes=1 if fault == 0 else 0)
        if not parsed or control == 'source_length_under':
            c.pop('notes', None)
        cases.append(c)
    for control, fault, count in [('chronos_space_exact', 0, 51), ('chronos_space_one_short', 902, 50)]:
        c = deepcopy(chronos)
        c.update(name=control, disk_control=control, fault=fault, disk_error=0, data_writes=count)
        cases.append(c)
    #43 reads precede append: system, catalog and41 note sectors. These are
    # failures at first/middle/last DATA writes of the complete song.
    for call in [44, 68, 94]:
        c = deepcopy(chronos)
        c.update(name='chronos_data_failure_' + str(call), fail_call=call,
                 fault=902, disk_error=2, data_writes=call - 44)
        cases.append(c)
    assert len({c['name'] for c in cases}) == len(cases)
    return dict(cases=cases)
