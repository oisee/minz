"""Disk controls derived from independent MIDI expectations, never target output."""
from copy import deepcopy


def trd_cases(raw, disk):
    cases = deepcopy(raw['cases'])
    assert len(cases) == 38
    for c in cases:
        if c['name'] == 'trailing_byte':
            c['expected']['fault'] = 905
    # The first four disk additions are Ruby-backed record boundaries and empty
    # input. TRD uses exact directory length, so no CP/M padding reinterpretation.
    extra = disk['cases'][38:42]
    assert [c['name'] for c in extra] == [
        'smf_exact_physical_record', 'wire_end_exact_physical_record',
        'padding_value_inside_midi', 'empty_host_file']
    cases += deepcopy(extra)
    single = next(c for c in cases if c['name'] == 'single')
    empty = dict(notes=[], sources=0, active=0, peak_active=0,
                 accepted_bytes=0, events=0)
    # First system/catalog reads, input sector, output data, then finish's
    # catalog read/write and system read/write. Publication is not atomic.
    for call, fault in enumerate([900, 900, 904, 902, 903, 903, 903, 903], 1):
        c = deepcopy(single)
        c.update(name='sector_failure_' + str(call), fail_call=call,
                 disk_error=2, allow_data=call >= 4, published=call >= 7)
        c['expected'] = deepcopy(empty if call <= 3 else single['expected'])
        c['expected']['fault'] = fault
        cases.append(c)
    for control, error, parsed, fault in [
        ('missing_input', 7, False, 900),
        ('existing_output', 5, False, 900),
        ('duplicate_input', 6, False, 900),
        ('bad_geometry', 3, False, 900),
        ('bad_directory_count', 3, False, 900),
        ('bad_source_extent', 4, False, 900),
        ('free_overlap', 8, False, 900),
        ('no_space', 9, True, 901),
        ('directory_full', 9, True, 901),
        ('last_sector', 0, True, 0),
        ('full_disk', 9, True, 901),
        ('bad_full_sentinel', 3, False, 900),
    ]:
        c = deepcopy(single)
        c.update(name=control, disk_control=control, disk_error=error,
                 published=control == 'existing_output')
        c['expected'] = deepcopy(single['expected'] if parsed else empty)
        c['expected']['fault'] = fault
        cases.append(c)
    assert len(cases) == 62
    return dict(cases=cases)
