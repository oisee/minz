#!/usr/bin/env python3
"""Create a separate Tetris border diagnostic; keep release source uninstrumented."""
import argparse
from pathlib import Path


def instrument(source):
    markers = (
        ('    if drawn_valid!=0 {\n        if board_dirty==0 {',
         '    zx_border(1) // blue: placement/checks\n    if drawn_valid!=0 {\n        if board_dirty==0 {'),
        ('    pixel_reset_spans()\n', '    zx_border(3) // magenta: attribute preparation\n    pixel_reset_spans()\n'),
        ('    if pixel_check_prepared()==false {return false}\n    zx_halt()',
         '    if pixel_check_prepared()==false {return false}\n    zx_border(5) // cyan: waiting for INT\n    zx_halt()'),
        ('    pixel_publish_direct()\n    asm z80 {EI}',
         '    zx_border(7) // white: commit entry\n    pixel_publish_direct()\n    zx_border(0) // black: commit complete\n    asm z80 {EI}'),
        ('    if drawn_valid!=0 {pixel_commit_erase()}',
         '    zx_border(2) // red: exact erase\n    if drawn_valid!=0 {pixel_commit_erase()}'),
        ('    if direct_refresh!=0 {pixel_restore_board()}',
         '    zx_border(6) // yellow: board refresh\n    if direct_refresh!=0 {pixel_restore_board()}'),
        ('    if game_over==0 {pixel_commit_actor()}',
         '    zx_border(4) // green: sprite output\n    if game_over==0 {pixel_commit_actor()}'),
        ('    pixel_publish_previews()\n',
         '    zx_border(7) // white: preview attributes\n    pixel_publish_previews()\n'),
        ('        pixel_publish_attributes()\n',
         '        zx_border(7) // white: field attributes\n        pixel_publish_attributes()\n'),
        ('    if row==direct_actor_lastrow {pixel_commit_actor()}',
         '    if row==direct_actor_lastrow {zx_border(4) pixel_commit_actor() zx_border(6)}'),
    )
    for before, after in markers:
        if source.count(before) != 1:
            raise ValueError(f'border marker requires one exact anchor: {before!r}')
        source = source.replace(before, after, 1)
    return source


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    rendered = instrument(args.source.read_text())
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(rendered)


if __name__ == '__main__':
    main()
