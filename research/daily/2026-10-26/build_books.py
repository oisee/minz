#!/usr/bin/env python3
"""Build an A5 illustrated PDF and reflowable EPUB from the checked articles.

Uses cached Mermaid SVG (like abap-deep-dive's build_v2 approach), Chromium for
vector body pages, ReportLab for book front matter/page furniture and Pandoc for
EPUB. No LaTeX installation is needed. Outputs are ready to send to a reader.
"""
from pathlib import Path
from html import escape
import argparse
import base64
import importlib.util
import io
import json
import re
import subprocess
import sys

sys.dont_write_bytecode = True
import tempfile
import zipfile

from pypdf import PdfReader, PdfWriter
from reportlab.pdfgen import canvas
from reportlab.lib import colors
from reportlab.lib.pagesizes import A5
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import Paragraph

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
SLUGS = ['01-languages-and-ideas', '02-melody-between-two-files', '03-borrowing-without-a-runtime']
TITLES = ['Ruby рассказывает, Z80 считает', 'Мелодия между двумя файлами', 'Чемодан без виртуальной машины']
TITLE = 'Один день из жизни компилятора MinZ'
MERMAID = re.compile(r'^```mermaid\s*\n(.*?)^```\s*$', re.M | re.S)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--chrome', required=True)
    p.add_argument('--node-modules', type=Path, required=True)
    p.add_argument('--font-dir', type=Path, default=Path('/System/Library/Fonts/Supplemental'))
    args = p.parse_args()
    pdf_dir, epub_dir = HERE / 'output/pdf', HERE / 'output/epub'
    pdf_dir.mkdir(parents=True, exist_ok=True)
    epub_dir.mkdir(parents=True, exist_ok=True)
    for slug in SLUGS:
        if not (HERE / (slug + '.html')).exists():
            raise RuntimeError('Run build.py first')
    pdfmetrics.registerFont(TTFont('BookSans', str(args.font_dir / 'Arial.ttf')))
    pdfmetrics.registerFont(TTFont('BookBold', str(args.font_dir / 'Arial Bold.ttf')))
    spec = importlib.util.spec_from_file_location('article_build', HERE / 'build.py')
    build = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(build)
    final_pdf = pdf_dir / 'minz-compiler-day-2026-10-26-A5.pdf'
    final_epub = epub_dir / 'minz-compiler-day-2026-10-26.epub'
    with tempfile.TemporaryDirectory(prefix='minz-books-') as temp:
        temp = Path(temp)
        figures = []
        for svg in sorted((HERE / 'assets').glob('*.svg')):
            figures.append(dict(svg=str(svg), png=str(temp / (svg.stem + '.png'))))
        chapters = [dict(html=str(HERE / (s + '.html')), pdf=str(temp / (s + '.pdf'))) for s in SLUGS]
        cfg = temp / 'render.json'
        cfg.write_text(json.dumps(dict(modules=str(args.node_modules.resolve()), chrome=args.chrome,
                                      figures=figures, chapters=chapters, css=(HERE / 'book.css').read_text())))
        subprocess.run(['node', str(HERE / 'book-render.mjs'), str(cfg)], check=True)
        counts = [len(PdfReader(c['pdf']).pages) for c in chapters]
        starts = [3, 3 + counts[0], 3 + counts[0] + counts[1]]
        front = temp / 'front.pdf'
        w, h = A5
        c = canvas.Canvas(str(front), pagesize=A5)
        c.setTitle(TITLE)

        def para(text, x, y, width, size=12, bold=False, leading=None, color='#203e43'):
            style = ParagraphStyle('p', fontName='BookBold' if bold else 'BookSans', fontSize=size,
                                   leading=leading or size * 1.45, textColor=colors.HexColor(color))
            item = Paragraph(escape(text), style)
            _, height = item.wrap(width, h)
            item.drawOn(c, x, y - height)
            return height

        c.setFillColor(colors.HexColor('#f2eee2')); c.rect(0, 0, w, h, stroke=0, fill=1)
        c.setFillColor(colors.HexColor('#245e48')); c.rect(30, h - 65, 55, 5, stroke=0, fill=1)
        para('26 ОКТЯБРЯ 2026 / ДНЕВНИК КОМПИЛЯТОРА', 30, h - 82, w - 60, 9, True)
        para('Один день из жизни компилятора MinZ', 30, h - 125, w - 60, 31, True, 35)
        para('Ruby и Crystal. Битовые потоки и идеи BEAM. Ленивые мелодии, генераторы и Z80.',
             30, h - 290, w - 60, 13, leading=19)
        para('Три иллюстрированных лонгрида', 30, 168, w - 60, 13, True)
        para('18 Mermaid-схем / реальные примеры / измеряемые результаты', 30, 137, w - 60, 10)
        para('MinZ project / при участии Astra', 30, 72, w - 60, 10)
        c.showPage()
        para('Содержание', 30, h - 42, w - 60, 24, True)
        y = h - 100
        for i, (title, page) in enumerate(zip(TITLES, starts), 1):
            used = para(f'{i:02d}. {title}', 30, y, w - 100, 14, True)
            para(str(page), w - 55, y, 30, 14, True)
            y -= used + 27
        y -= 15
        used = para('В PDF сохранены векторные схемы. Широкие иллюстрации размещены на горизонтальных страницах A5; большие схемы при необходимости разделены на две части.',
                    30, y, w - 60, 10)
        y -= used + 19
        para('Срез main: a716ff184. CP/M-цепочка уже проверена. Spectrum - отдельный локальный эксперимент. Полного нативного MIDI -> VT2/VTX-конвертера пока нет.',
             30, y, w - 60, 10)
        c.showPage(); c.save()
        writer = PdfWriter()
        writer.append(str(front), import_outline=False)
        for title, chapter in zip(TITLES, chapters):
            writer.append(chapter['pdf'], outline_item=title, import_outline=False)
        # Global page numbers, independent of chapter and page orientation.
        total = len(writer.pages)
        for i, page in enumerate(writer.pages):
            if i == 0:
                continue
            width, height = float(page.mediabox.width), float(page.mediabox.height)
            buf = io.BytesIO()
            overlay = canvas.Canvas(buf, pagesize=(width, height))
            overlay.setFont('BookSans', 7.5)
            overlay.setFillColor(colors.HexColor('#5e7469'))
            overlay.drawString(34, 23, 'MinZ / 26.10.2026')
            overlay.drawRightString(width - 34, 23, f'{i + 1} / {total}')
            overlay.save(); buf.seek(0)
            page.merge_page(PdfReader(buf).pages[0])
        writer.add_metadata({'/Title': TITLE, '/Author': 'MinZ project; Astra', '/Lang': 'ru-RU'})
        with final_pdf.open('wb') as stream:
            writer.write(stream)
        subprocess.run(['pdftoppm', '-f', '1', '-singlefile', '-scale-to', '1600', '-png', str(final_pdf), str(temp / 'cover')], check=True)
        image_map = {Path(f['svg']).name: Path(f['png']) for f in figures}
        prepared = []
        for number, slug in enumerate(SLUGS, 1):
            text = (HERE / (slug + '.md')).read_text()
            text = re.sub(r'^\[Все три статьи\].*\n', '', text, flags=re.M)
            index = 0

            def diagram(match):
                nonlocal index
                index += 1
                path = image_map[slug + '-' + str(index) + '.svg']
                return f'\n![Схема {number}.{index}]({path})\n'

            text = MERMAID.sub(diagram, text)

            def link(match):
                image, label, target = match.groups()
                if image:
                    path = Path(target)
                    if path.suffix == '.svg':
                        path = image_map[path.name]
                    return '![' + label + '](' + str(path) + ')'
                return build.html_links(match)

            text = build.LINK.sub(link, text)
            prepared.append(text)
        combined = temp / 'book.md'
        combined.write_text('\n\n'.join(prepared))
        epub_css = temp / 'epub.css'
        epub_css.write_text('body{font-family:serif;line-height:1.5}h1,h2,h3{font-family:sans-serif;line-height:1.2}img{max-width:100%;height:auto}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:.78em;background:#f2f3ed;padding:.7em}code{font-size:.88em}table{border-collapse:collapse;font-size:.8em;width:100%}td,th{padding:.35em;border-bottom:1px solid #b9cbbd}blockquote{border-left:.2em solid #84ac91;padding-left:1em}a{color:#245e48}')
        subprocess.run(['pandoc', str(combined), '--from=gfm', '--to=epub3', '--toc', '--toc-depth=2',
                        '--metadata', 'title=' + TITLE, '--metadata', 'author=MinZ project; при участии Astra',
                        '--metadata', 'date=2026-10-26', '--metadata', 'lang=ru-RU',
                        '--epub-cover-image=' + str(temp / 'cover.png'), '--css=' + str(epub_css),
                        '-o', str(final_epub)], check=True)
        # Packaging and text checks complement the separate visual PDF review.
        reader = PdfReader(str(final_pdf))
        text = '\n'.join(p.extract_text() or '' for p in reader.pages)
        for word in ['Crystal', 'Nanz', 'RLE', 'LLVM', 'OTP']:
            if word not in text:
                raise ValueError('Missing book text: ' + word)
        sizes = sorted({tuple(round(float(x), 1) for x in (p.mediabox.width, p.mediabox.height)) for p in reader.pages})
        if any(abs(min(size) - A5[0]) > 1 or abs(max(size) - A5[1]) > 1 for size in sizes):
            raise ValueError('Non-A5 page in book: ' + str(sizes))
        with zipfile.ZipFile(final_epub) as archive:
            if archive.read('mimetype') != b'application/epub+zip':
                raise ValueError('EPUB package')
            image_count = len([n for n in archive.namelist() if n.endswith('.png')])
            if image_count < 21:
                raise ValueError('Missing EPUB illustrations: ' + str(image_count))
        receipt = dict(protocol='minz-compiler-day-books/v1', pdf_pages=len(reader.pages), page_sizes_points=sizes,
                       chapter_pages=counts, epub_png_images=image_count,
                       pdf=str(final_pdf.relative_to(HERE)), epub=str(final_epub.relative_to(HERE)),
                       diagrams=18, illustrations=2, vector_pdf=True)
        (HERE / 'output/books.json').write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + '\n')
        print(json.dumps(receipt, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
