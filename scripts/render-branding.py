#!/usr/bin/env python3
"""Render application, interface and tray branding separately (needs rsvg-convert).

Source artwork: hackdiaz-dev/turn-proxy-android, bab60c5fb620cbea2570eeeb5a0697dc355071f4.
Artwork licensing and adaptation details: docs/branding.md.
"""
from pathlib import Path
from xml.etree import ElementTree as ET
import struct
import subprocess

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / 'desktop/assets'
A = '{http://schemas.android.com/apk/res/android}'
AA = '{http://schemas.android.com/aapt}'


def svg(body, viewbox='80 56 352 400'):
    return f'<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="{viewbox}">{body}</svg>\n'


def application():
    source = ET.parse(ASSETS / 'branding-source/ic_launcher_foreground.xml')
    gradients, paths = [], []
    for i, p in enumerate(source.getroot().findall('group/path')):
        g = p.find(AA+'attr/gradient')
        coordinates = ' '.join(f'{key}="{g.attrib[A+value]}"' for key, value in
                               [('x1', 'startX'), ('y1', 'startY'), ('x2', 'endX'), ('y2', 'endY')])
        stops = ''.join(f'<stop offset="{s.attrib[A+"offset"]}" stop-color="#{s.attrib[A+"color"][-6:]}"/>'
                        for s in g.findall('item'))
        gradients.append(f'<linearGradient id="g{i}" gradientUnits="userSpaceOnUse" {coordinates}>{stops}</linearGradient>')
        paths.append(f'<path d="{p.attrib[A+"pathData"]}" fill="url(#g{i})"/>')
    return svg('<defs>'+''.join(gradients)+'</defs><g transform="translate(256 256) scale(1.04) translate(-256 -256)">'+''.join(paths)+'</g>')


def monochrome(color, dense=False):
    source = ET.parse(ASSETS / 'branding-source/ic_launcher_monochrome.xml')
    paths = []
    for i, p in enumerate(source.getroot().findall('group/path')):
        stroke = f' stroke="{color}" stroke-width="3.5" stroke-linejoin="round"' if dense and i == 0 else ''
        paths.append(f'<path d="{p.attrib[A+"pathData"]}" fill="{color}"{stroke}/>')
    transform = 'translate(256 256) scale(1.25) translate(-279.88 -255.36)' if dense else 'translate(256 256) scale(1.04) translate(-256 -256)'
    return svg(f'<g transform="{transform}">{"".join(paths)}</g>', '0 0 512 512' if dense else '80 56 352 400')


def render(source, size):
    return subprocess.check_output(['rsvg-convert', '-w', str(size), '-h', str(size), str(source)])


def ico(source, sizes):
    images = [render(source, size) for size in sizes]
    offset = 6 + 16*len(sizes)
    directory = []
    for size, image in zip(sizes, images):
        directory.append(struct.pack('<BBBBHHII', size % 256, size % 256, 0, 0, 1, 32, len(image), offset))
        offset += len(image)
    return struct.pack('<HHH', 0, 1, len(sizes)) + b''.join(directory) + b''.join(images)


def main():
    assert (ROOT / 'docs/licenses/android-branding-GPL-3.0.txt').is_file(), 'upstream artwork license is required'
    (ASSETS / 'icon.svg').write_text(application())
    (ASSETS / 'interface.svg').write_text(monochrome('#eff4ff'))
    for variant, color in [('dark', '#273247'), ('light', '#eef3ff')]:
        path = ASSETS / f'tray-{variant}.svg'
        path.write_text(monochrome(color, dense=True))
        (ASSETS / f'tray-{variant}-64.png').write_bytes(render(path, 64))
        (ASSETS / f'tray-{variant}.ico').write_bytes(ico(path, (16, 20, 22, 24, 32, 40, 48, 64)))
    for size, path in [(512, ASSETS / 'icon.png'), (64, ASSETS / 'icon-64.png'),
                       (1024, ROOT / 'desktop/build/appicon.png')]:
        path.write_bytes(render(ASSETS / 'icon.svg', size))
    (ROOT / 'desktop/build/windows/icon.ico').write_bytes(ico(ASSETS / 'icon.svg', (16, 24, 32, 48, 64, 128, 256)))
    (ROOT / 'desktop/frontend/public/freeturn.svg').write_bytes((ASSETS / 'interface.svg').read_bytes())
    (ROOT / 'desktop/frontend/public/favicon.svg').write_bytes((ASSETS / 'icon.svg').read_bytes())
    # Keep the old light-icon filenames as compatibility aliases, not old FT artwork.
    (ASSETS / 'icon-light.svg').write_bytes((ASSETS / 'tray-light.svg').read_bytes())
    (ASSETS / 'icon-light-64.png').write_bytes((ASSETS / 'tray-light-64.png').read_bytes())
    print('Rendered: transparent colour app icon, flat interface mark, dense adaptive tray icons.')


if __name__ == '__main__':
    main()
