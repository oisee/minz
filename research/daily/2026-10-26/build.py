#!/usr/bin/env python3
"""Render the three Markdown longreads as offline illustrated HTML."""
from pathlib import Path
from html import escape
from html.parser import HTMLParser
import argparse
import base64
import hashlib
import json
import math
import re
import shutil
import subprocess
import tempfile
from urllib.parse import unquote, urlsplit

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
SNAPSHOT = 'a716ff184a50c9e17eea4d6679cbcd1ed1f9b379'
SLUGS = ['01-languages-and-ideas', '02-melody-between-two-files', '03-borrowing-without-a-runtime']
TITLES = ['Ruby рассказывает, Z80 считает', 'Мелодия между двумя файлами', 'Чемодан без виртуальной машины']
DECKS = ['Crystal и Nanz, macro и meta, битовые схемы и идеи Erlang/OTP.',
         'Ленивость, генераторы, fusion, 64 байта RLE и нативный MIDI pipeline.',
         'Что взять у LLVM, Cranelift, Grace, C#/.NET, JVM/Kotlin, Rust и BEAM.']
MERMAID = re.compile(r'^```mermaid\s*\n(.*?)^```\s*$', re.M | re.S)
LINK = re.compile(r'(!?)\[([^\]]*)\]\(([^)]+)\)')


def data_uri(path):
    return 'data:image/svg+xml;base64,' + base64.b64encode(path.read_bytes()).decode()


def check_links(text):
    for _, _, target in LINK.findall(text):
        u = urlsplit(target)
        if u.scheme or target.startswith('#'):
            continue
        path = (HERE / unquote(u.path)).resolve()
        planned = path.parent == HERE and path.name in [s + '.html' for s in SLUGS] + ['index.html']
        if not path.exists() and not planned:
            raise ValueError('Missing local link: ' + target)


def html_links(match):
    image, label, target = match.groups()
    if urlsplit(target).scheme or target.startswith('#'):
        return match.group(0)
    path, sep, anchor = target.partition('#')
    resolved = (HERE / path).resolve()
    if image:
        return '<img src="' + data_uri(resolved) + '" alt="' + escape(label, quote=True) + '">'
    if path == 'README.md':
        target = 'index.html'
    elif path.removesuffix('.md') in SLUGS:
        target = path.removesuffix('.md') + '.html'
    else:
        target = 'https://github.com/oisee/minz/blob/' + SNAPSHOT + '/' + str(resolved.relative_to(ROOT))
    if sep:
        target += '#' + anchor
    return '[' + label + '](' + target + ')'


class Headings(HTMLParser):
    def __init__(self):
        super().__init__()
        self.items, self.current = [], None

    def handle_starttag(self, tag, attrs):
        if tag in ('h2', 'h3'):
            self.current = [tag, dict(attrs).get('id', ''), '']

    def handle_data(self, data):
        if self.current:
            self.current[2] += data

    def handle_endtag(self, tag):
        if self.current and tag == self.current[0]:
            self.items.append(self.current)
            self.current = None


def shell_page(title, body, css, js):
    return ('<!doctype html><html lang="ru"><head><meta charset="utf-8">'
            '<meta name="viewport" content="width=device-width,initial-scale=1">'
            '<meta name="color-scheme" content="light"><title>' + escape(title) + ' · MinZ</title>'
            '<style>' + css + '</style></head><body><div class="progress" aria-hidden="true"></div>'
            '<header class="sitebar"><a class="brand" href="index.html">MinZ / Дневник</a>'
            '<nav><a href="01-languages-and-ideas.html">Языки</a><a href="02-melody-between-two-files.html">Музыка</a>'
            '<a href="03-borrowing-without-a-runtime.html">Экосистемы</a>'
            '<a href="https://github.com/oisee/minz">GitHub</a></nav></header>' + body +
            '<footer>26 октября 2026 · MinZ · Две главы подготовлены совместно с Astra. '
            'Работающие возможности, эксперименты и предложения различаются в тексте. '
            '<a href="README.md">Markdown и воспроизведение</a></footer><script>' + js + '</script></body></html>')


DEMO = '''<section class="playground" id="playground" aria-labelledby="demo-title">
<h3 id="demo-title">Попросите у RLE ровно столько, сколько нужно</h3>
<p>Светлые пары ещё не прочитаны. Попробуйте спрос 0, 2, 3 или 4: четвёртый запрос пройдёт и через пустой run.</p>
<div class="controls"><label>Пары count:value, десятичные байты<input name="runs" value="3:65, 0:90, 2:66" aria-label="Пары RLE"></label>
<label>Нужно байтов<input name="demand" type="number" min="0" max="64" value="2" aria-label="Сколько байтов потребовать"></label></div>
<button type="button">Выполнить запрос</button><div class="runs" aria-live="polite"></div>
<div class="model-stats"></div><div class="output" aria-live="polite"></div>
<p class="notice">Это поясняющая модель в браузере. Она не компилирует Nanz, не эмулирует Z80 и не измеряет его скорость. Счётчик left здесь показан после выданных байтов; исходный generator при break может не выполнить последний suffix.</p>
<noscript>Для этой модели включите JavaScript. Все статьи и схемы читаются без него.</noscript></section>'''


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mmdc', default=shutil.which('mmdc'))
    p.add_argument('--chrome', required=True)
    args = p.parse_args()
    if not args.mmdc:
        p.error('--mmdc or an installed mmdc is required')
    css, js = (HERE / 'style.css').read_text(), (HERE / 'reader.js').read_text()
    mermaid_version = subprocess.check_output([args.mmdc, '--version'], text=True).strip()
    for slug in SLUGS:
        check_links((HERE / (slug + '.md')).read_text())
    check_links((HERE / 'README.md').read_text())
    counts, outputs = [], []
    with tempfile.TemporaryDirectory(prefix='minz-articles-') as temp:
        temp = Path(temp)
        pc = temp / 'puppeteer.json'
        pc.write_text(json.dumps(dict(executablePath=args.chrome, args=['--disable-gpu'])))
        config = temp / 'mermaid.json'
        config.write_text(json.dumps(dict(theme='base', htmlLabels=False, markdownAutoWrap=False,
            themeVariables=dict(fontFamily='Arial', fontSize='17px', primaryColor='#e0efe3',
                                primaryTextColor='#233b40', primaryBorderColor='#5c8672',
                                lineColor='#607f78', secondaryColor='#f6ecd8', tertiaryColor='#e5ebf7'),
            flowchart=dict(htmlLabels=False, curve='basis'), sequence=dict(wrap=True, width=130))))
        for number, slug in enumerate(SLUGS, 1):
            source = HERE / (slug + '.md')
            text = source.read_text()
            diagrams = MERMAID.findall(text)
            fingerprint = hashlib.sha256(json.dumps([diagrams, config.read_text(), mermaid_version]).encode()).hexdigest()
            cache = HERE / 'assets' / (slug + '.render.json')
            cached = json.loads(cache.read_text()) if cache.exists() else {}
            paths = [HERE / 'assets' / (slug + '-' + str(i + 1) + '.svg') for i in range(len(diagrams))]
            hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in paths if p.exists()}
            if cached.get('fingerprint') != fingerprint or cached.get('svg_sha256') != hashes or len(hashes) != len(diagrams):
                subprocess.run([args.mmdc, '-i', str(source), '-o', str(temp / (slug + '.md')),
                                '-a', str(HERE / 'assets'), '-e', 'svg', '-j', '2',
                                '--no-font-embed', '-b', 'transparent', '-p', str(pc), '-c', str(config)], check=True)
                hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}
                cache.write_text(json.dumps(dict(fingerprint=fingerprint, svg_sha256=hashes), indent=2) + '\n')
            index = 0

            def diagram(match):
                nonlocal index
                index += 1
                path = HERE / 'assets' / (slug + '-' + str(index) + '.svg')
                svg = path.read_text()
                if '<svg' not in svg or 'Syntax error in text' in svg:
                    raise ValueError('Failed Mermaid: ' + str(path))
                box = re.search(r'viewBox="([^"]+)"', svg)
                width = min(float(box.group(1).split()[2]), 1080) if box else 900
                return ('\n<figure class="diagram"><div class="diagram-viewport"><img style="width:' + str(round(width)) +
                        'px" src="' + data_uri(path) + '" alt="Схема ' + str(number) + '.' + str(index) +
                        '"></div><figcaption>Схема ' + str(number) + '.' + str(index) +
                        ' · На узком экране схему можно прокрутить горизонтально.</figcaption>'
                        '<details><summary>Посмотреть Mermaid</summary><pre><code>' + escape(match.group(1).strip()) +
                        '</code></pre></details></figure>\n')

            # Rewrite Markdown links before inserting rendered figures, so embedded
            # Mermaid source cannot accidentally be mistaken for a Markdown link.
            text = LINK.sub(html_links, text)
            text = MERMAID.sub(diagram, text)
            title = re.match(r'# (.+)', text).group(1)
            text = text.split('\n', 1)[1]
            fragment = subprocess.run(['pandoc', '--from=gfm+raw_html', '--to=html5'], input=text,
                                      capture_output=True, text=True, check=True).stdout
            if slug.startswith('02-'):
                # Place the model immediately before the fusion explanation.
                found = re.search(r'<h2\b[^>]*>\s*Fusion:', fragment)
                if not found:
                    raise ValueError('Missing fusion section for interactive insertion')
                fragment = fragment[:found.start()] + DEMO + fragment[found.start():]
            headings = Headings()
            headings.feed(fragment)
            toc = '<aside class="toc" aria-label="Содержание"><strong>В этой главе</strong>' + ''.join(
                '<a class="' + ('sub' if tag == 'h3' else '') + '" href="#' + escape(anchor, quote=True) + '">' + escape(label) + '</a>'
                for tag, anchor, label in headings.items) + '</aside>'
            words = len(source.read_text().split())
            minutes = math.ceil(words / 200)
            hero = ('<div class="hero"><div class="eyebrow">День компилятора · Глава 0' + str(number) + '</div>'
                    '<h1>' + escape(title) + '</h1><p>' + escape(DECKS[number - 1]) + '</p><div class="meta">'
                    '<span class="pill">≈ ' + str(minutes) + ' мин</span><span class="pill cream">' + str(len(diagrams)) +
                    ' Mermaid-схем</span><span class="pill">Ruby · Crystal · Nanz · Z80</span></div></div>')
            next_number = number % 3
            next_link = '<div class="next"><a href="' + SLUGS[next_number] + '.html">Дальше: ' + escape(TITLES[next_number]) + ' →</a></div>'
            html = shell_page(title, hero + '<main class="layout">' + toc + '<article>' + fragment + next_link + '</article></main>', css, js)
            target = HERE / (slug + '.html')
            target.write_text(html)
            outputs.append(target)
            counts.append(dict(slug=slug, words_approx=words, diagrams=len(diagrams), reading_minutes_approx=minutes))
        cover = data_uri(HERE / 'assets/pipeline-workshop.svg')
        cards = ''.join('<a class="card" href="' + slug + '.html"><span class="num">0' + str(i + 1) + '</span><h2>' +
                        escape(TITLES[i]) + '</h2><p>' + escape(DECKS[i]) + '</p><span class="read">Открыть главу →</span></a>'
                        for i, slug in enumerate(SLUGS))
        body = ('<div class="hero"><div class="eyebrow">26 октября 2026 · Три лонгрида на русском</div>'
                '<h1>Один день из жизни<br>компилятора MinZ</h1><p>Как Ruby, Crystal и Nanz встретились с битовыми потоками, '
                'ленивыми мелодиями и Z80 — и что мы хотим привезти из больших экосистем.</p>'
                '<p><a href="output/epub/minz-compiler-day-2026-10-26.epub">Скачать EPUB</a> · '
                '<a href="output/pdf/minz-compiler-day-2026-10-26-A5.pdf">PDF A5, 74 страницы</a></p>'
                '<div class="meta"><span class="pill">18 Mermaid-схем</span><span class="pill cream">2 иллюстрации</span>'
                '<span class="pill">Реальный код и измерения</span></div></div><main class="index-content">'
                '<img class="index-cover" src="' + cover + '" alt="Мастерская: MIDI, Crystal/Nanz и два файла"><div class="cards">' + cards + '</div>'
                '<div class="status"><div><strong>Уже в main</strong>CP/M: MIDI → 1485 нот → 2153 события, двумя отдельными .COM.'
                '<br><small>PR #262 и #263 смержены после CI.</small></div>'
                '<div><strong>Локальный эксперимент</strong>Spectrum/Scorpion: первый MIDI → TRD этап, 180 исполнений.'
                '<br><small>Отдельная ветка; не physical ROM/hardware proof.</small></div>'
                '<div><strong>Следующий участок пути</strong>Arrangement/echo/mix и финальный VT2/VTX; общий lifetime и более широкий fusion.'
                '<br><small>Предложения в статьях обозначены явно.</small></div></div></main>')
        target = HERE / 'index.html'
        target.write_text(shell_page('Один день из жизни компилятора MinZ', body, css, js))
        outputs.append(target)
    if sum(c['diagrams'] for c in counts) != 18:
        raise ValueError('Unexpected diagram inventory')
    sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    sources = [HERE / (s + '.md') for s in SLUGS] + [HERE / p for p in ['README.md', 'build.py', 'style.css', 'reader.js']]
    manifest = dict(protocol='minz-compiler-day-articles/v1', date='2026-10-26', described_main=SNAPSHOT,
                    chapters=counts, diagrams=18, illustrations=2,
                    tools=dict(mermaid_cli=mermaid_version,
                               pandoc=subprocess.check_output(['pandoc', '--version'], text=True).splitlines()[0]),
                    source_sha256={str(p.relative_to(HERE)): sha(p) for p in sources},
                    output_sha256={p.name: sha(p) for p in outputs})
    (HERE / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print('PASS: three HTML chapters + index; 18 rendered Mermaid; all local Markdown links resolve')


if __name__ == '__main__':
    main()
