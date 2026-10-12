"""Independent source-level ANSI stream model for the three fixed Agon demos.

This checks the output stream, not graphical rendering by a real VDP.
"""

ESC = '\x1b'


def goto(x, y):
    # Library's decimal routine emits at least two digits.
    return f'{ESC}[{y + 1:02d};{x + 1:02d}H'


def color(fg, bg, bright):
    return f'{ESC}[{30 + fg + 60 * bright:02d};{40 + bg + 60 * bright:02d}m'


def box(x, y, width, height):
    s = goto(x, y) + '+' + '-' * (width - 2) + '+'
    for row in range(1, height - 1):
        s += goto(x, y + row) + '|' + goto(x + width - 1, y + row) + '|'
    return s + goto(x, y + height - 1) + '+' + '-' * (width - 2) + '+'


def expected(name):
    clear = ESC + '[2J' + ESC + '[H'
    reset = ESC + '[0m'
    if name == 'tui_hello':
        return (clear + color(2, 0, 1) + box(5, 3, 40, 7)
                + goto(8, 5) + color(7, 0, 1) + 'Hello from MinZ on Agon!'
                + goto(8, 7) + color(3, 0, 0) + 'eZ80 ADL mode @ 18.432 MHz'
                + goto(0, 12) + reset + 'Press any key...')
    if name == 'tui_demo':
        s = clear + ESC + '[?25l' + goto(0, 0) + color(0, 6, 0)
        s += ' MinZ TUI Demo for Agon Light 2                                                '
        s += color(7, 4, 1) + box(2, 2, 36, 12)
        s += goto(4, 2) + color(0, 6, 0) + ' Panel '
        for row, fg, bright, text in [(4, 7, 1, 'White on blue'), (5, 3, 1, 'Yellow on blue'),
                                     (6, 2, 1, 'Green on blue'), (7, 1, 0, 'Red on blue'),
                                     (9, 6, 1, 'eZ80 @ 18.432 MHz'), (10, 7, 0, 'Compiled with MinZ')]:
            s += goto(4, row) + color(fg, 4, bright) + text
        s += goto(0, 24) + color(0, 6, 0)
        s += ' Press any key to exit                                                          '
        return s + ESC + '[?25h' + reset + clear
    if name == 'nc_agon':
        s = clear + ESC + '[?25l' + goto(0, 0) + color(0, 6, 0)
        s += ' MinZ NC for Agon Light 2                                                       '
        s += color(7, 4, 1) + box(1, 2, 40, 12)
        s += goto(3, 2) + color(0, 6, 0) + ' /sdcard '
        s += goto(3, 3) + color(3, 4, 1) + 'Name           Size'
        s += goto(0, 24)
        for fg, bg, text in [(0, 6, ' Up/Dn'), (7, 0, 'Nav '), (0, 6, ' Enter'),
                            (7, 0, 'Open '), (0, 6, ' Esc'), (7, 0, 'Quit                                            ')]:
            s += color(fg, bg, 0) + text
        names = ['HELLO    .BIN', 'NC_AGON  .BIN', 'TETRIS   .BIN', 'README   .TXT',
                 'STDLIB   .   ', 'DOCS     .   ', 'EXAMPLES .   ', 'PLASMA   .BIN']
        sizes = ['   166', '  1024', '  2048', '   512', ' <DIR>', ' <DIR>', ' <DIR>', '   789']
        for i, (filename, size) in enumerate(zip(names, sizes)):
            s += goto(2, 4 + i) + (color(0, 6, 0) if i == 0 else color(7, 4, 1))
            s += ' ' + filename + size + '          '
        # Frozen immediate ESC scenario: cleanup without navigation/redraw.
        return s + ESC + '[?25h' + reset + clear
    raise ValueError(name)


def decode_fake_vdp(stdout):
    """Invert only the documented CLI VDP ESC diagnostic, preserve all other bytes."""
    before, sep, rest = stdout.partition('/ *run\n')
    body, after, remainder = rest.partition('/ *echo MINZ_RETURNED\nMINZ_RETURNED\n/ *')
    if not sep or not after or stdout.count('Agon Console8 MOS Version') != 1:
        raise ValueError('missing clean MOS run/return or repeated boot banner')
    if 'Unknown packet VDU' in body.replace('Unknown packet VDU 0x1b\n', ''):
        raise ValueError('unsupported VDU diagnostic outside ESC')
    return body.replace('Unknown packet VDU 0x1b\n', ESC)
