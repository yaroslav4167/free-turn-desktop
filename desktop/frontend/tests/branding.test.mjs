import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import {join} from 'node:path'

const root = fileURLToPath(new URL('../../../', import.meta.url))

test('application favicon and flat interface icon use separate canonical SVGs', () => {
  const original = readFileSync(join(root, 'desktop/assets/icon.svg'))
  const flat = readFileSync(join(root, 'desktop/assets/interface.svg'))
  assert.deepEqual(readFileSync(join(root, 'desktop/frontend/public/freeturn.svg')), flat)
  assert.deepEqual(readFileSync(join(root, 'desktop/frontend/public/favicon.svg')), original)
  assert.notDeepEqual(original, flat)
  assert.doesNotMatch(flat.toString(), /<rect|linearGradient|radialGradient/)
  assert.doesNotMatch(original.toString(), /<rect/)
  assert.match(readFileSync(join(root, 'desktop/frontend/index.html'), 'utf8'), /href="\/favicon\.svg"/)
})

test('native tray implementations use dedicated dense symbolic artwork', () => {
  const windows = readFileSync(join(root, 'desktop/tray/icon_windows.go'), 'utf8')
  assert.match(windows, /appassets.TrayIconICO/)
  assert.match(windows, /appassets.TrayLightIconICO/)
  assert.match(windows, /SystemUsesLightTheme/)
  assert.doesNotMatch(readFileSync(join(root, 'desktop/tray/tray_windows.go'), 'utf8'), /ExtractIconExW/)
  const mac = readFileSync(join(root, 'desktop/tray/tray_darwin.m'), 'utf8')
  assert.match(mac, /dataWithBytes:icon/)
  assert.match(mac, /setTemplate:YES/)
  assert.match(mac, /imagePosition=NSImageOnly/)
  assert.ok(readFileSync(join(root, 'docs/licenses/android-branding-GPL-3.0.txt'), 'utf8').includes('GNU GENERAL PUBLIC LICENSE'))
})

test('Wails build uses the generated high-resolution application icon', () => {
  const png = readFileSync(join(root, 'desktop/build/appicon.png'))
  assert.equal(png.subarray(1, 4).toString(), 'PNG')
  assert.equal(png.readUInt32BE(16), 1024)
  assert.equal(png.readUInt32BE(20), 1024)
})

test('Windows EXE icon contains the current logo at shell and high-DPI sizes', () => {
  const ico = readFileSync(join(root, 'desktop/build/windows/icon.ico'))
  assert.equal(ico.readUInt16LE(0), 0)
  assert.equal(ico.readUInt16LE(2), 1)
  const sizes = []
  for (let i = 0; i < ico.readUInt16LE(4); i++) {
    const entry = 6 + i * 16
    const size = ico[entry] || 256
    sizes.push(size)
    assert.equal(ico[entry + 1] || 256, size)
    const length = ico.readUInt32LE(entry + 8)
    const offset = ico.readUInt32LE(entry + 12)
    const png = ico.subarray(offset, offset + length)
    assert.equal(png.length, length)
    assert.equal(png.subarray(1, 4).toString(), 'PNG')
    assert.equal(png.readUInt32BE(16), size)
    assert.equal(png.readUInt32BE(20), size)
    if (size === 64) {
      assert.deepEqual(png, readFileSync(join(root, 'desktop/assets/icon-64.png')))
    }
  }
  for (const size of [16, 32, 48, 64, 128, 256]) assert.ok(sizes.includes(size))
})

test('README screenshots are present and large enough to read', () => {
  const readme = readFileSync(join(root, 'README.md'), 'utf8')
  const images = [...readme.matchAll(/!\[[^\]]*\]\((docs\/screenshots\/[^)]+\.png)\)/g)]
  assert.equal(images.length, 3)
  for (const [, path] of images) {
    const png = readFileSync(join(root, path))
    assert.equal(png.subarray(1, 4).toString(), 'PNG')
    assert.ok(png.readUInt32BE(16) >= 900, `screenshot is too narrow: ${path}`)
    assert.ok(png.readUInt32BE(20) >= 600, `screenshot is too short: ${path}`)
  }
})

test('Russian README opens with the canonical logo and interface collage', () => {
  const readme = readFileSync(join(root, 'README.md'), 'utf8')
  const logo = readme.indexOf('src="desktop/assets/icon.svg"')
  const collage = readme.indexOf('src="docs/screenshots/interface-collage.png"')
  assert.ok(logo >= 0 && collage > logo)
  assert.ok(collage < readme.indexOf('## Возможности'))
  for (const heading of ['## Возможности', '## Скачать', '## Установка', '## Сборка из исходников']) {
    assert.ok(readme.includes(heading), heading)
  }
  assert.match(readme, /демонстрационн/i)
  const png = readFileSync(join(root, 'docs/screenshots/interface-collage.png'))
  assert.equal(png.subarray(1, 4).toString(), 'PNG')
  assert.equal(png.readUInt32BE(16), 1920)
  assert.equal(png.readUInt32BE(20), 1100)
})
