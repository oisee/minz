#!/usr/bin/env python3
"""Create a separate Tetris border diagnostic; keep release source uninstrumented."""
import argparse
from pathlib import Path


def instrument(source):
    markers = (
        ('        game_step(read_input(),elapsed)',
         '        zx_border(7) // white: input/physics\n        game_step(read_input(),elapsed)'),
        ('    if drawn_valid!=0 {\n        if board_dirty==0 {',
         '    zx_border(1) // blue: placement/checks\n    if drawn_valid!=0 {\n        if board_dirty==0 {'),
        ('    pixel_reset_spans()\n', '    zx_border(3) // magenta: attribute preparation\n    pixel_reset_spans()\n'),
        ('    if pixel_check_prepared()==false {return false}\n    zx_halt()',
         '    if pixel_check_prepared()==false {return false}\n    zx_border(5) // cyan: waiting for INT\n    zx_halt()'),
        ('    pixel_publish_direct()\n    asm z80 {EI}',
         '    zx_border(7) // white: commit entry\n    pixel_publish_direct()\n    zx_border(0) // black: commit complete\n    asm z80 {EI}'),
        ('    if drawn_valid!=0 {pixel_commit_erase()}',
         '    zx_border(2) // red: exact erase\n    if drawn_valid!=0 {pixel_commit_erase()}'),
        ('        field_board_publish_range_prepared(ptr(u16(&board)),ptr(u16(&tetris_values)),direct_board_first,direct_board_limit)',
         '        zx_border(6) // yellow: prepared board rows\n        field_board_publish_range_prepared(ptr(u16(&board)),ptr(u16(&tetris_values)),direct_board_first,direct_board_limit)'),
        ('        if direct_actor_early!=0 {pixel_commit_actor()}',
         '        if direct_actor_early!=0 {zx_border(4) pixel_commit_actor()} // green: actor outside board range'),
        ('    if game_over==0 {pixel_commit_actor()}',
         '    zx_border(4) // green: sprite output\n    if game_over==0 {pixel_commit_actor()}'),
        ('    if direct_preview!=0 {pixel_publish_previews()}\n',
         '    if direct_preview!=0 {zx_border(7) pixel_publish_previews()} // white: preview attributes\n'),
        ('    if direct_refresh!=0 {\n        pixel_publish_attributes()',
         '    if direct_refresh!=0 {\n        zx_border(7) // white: attributes outside board range\n        pixel_publish_attributes()'),
        ('        pixel_publish_attributes()\n    }\n}',
         '        zx_border(7) // white: field attributes\n        pixel_publish_attributes()\n    }\n}'),
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
