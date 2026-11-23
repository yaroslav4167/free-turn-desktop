import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mkdtempSync, readFileSync, rmSync} from 'node:fs'
import {fileURLToPath, pathToFileURL} from 'node:url'
import {join} from 'node:path'
import {build} from 'esbuild'
import {JSDOM} from 'jsdom'
import React from 'react'
import {renderToStaticMarkup} from 'react-dom/server'

test('shared progress bar preserves fractional values and accessible determinate/indeterminate states', async t => {
  const dir = mkdtempSync(fileURLToPath(new URL('./.update-progress-', import.meta.url)))
  t.after(() => rmSync(dir, {recursive: true, force: true}))
  const output = join(dir, 'bar.mjs')
  await build({entryPoints: [fileURLToPath(new URL('../src/components/UpdateProgressBar.tsx', import.meta.url))],
    outfile: output, bundle: true, format: 'esm', platform: 'node', jsx: 'automatic', external: ['react', 'react/jsx-runtime']})
  const {default: Bar} = await import(pathToFileURL(output))
  const render = value => {
    const dom = new JSDOM(renderToStaticMarkup(React.createElement(Bar, {value, label: 'Download'})))
    t.after(() => dom.window.close())
    return dom.window.document.querySelector('[role="progressbar"]')
  }
  const fractional = render(25.25)
  assert.equal(fractional.getAttribute('aria-label'), 'Download')
  assert.equal(fractional.getAttribute('aria-valuemin'), '0')
  assert.equal(fractional.getAttribute('aria-valuemax'), '100')
  assert.equal(fractional.getAttribute('aria-valuenow'), '25.25')
  assert.equal(fractional.firstElementChild.style.transform, 'scaleX(0.2525)')
  assert.equal(render(0).getAttribute('aria-valuenow'), '0')
  assert.equal(render(-1).getAttribute('aria-valuenow'), '0')
  assert.equal(render(120).getAttribute('aria-valuenow'), '100')
  for (const value of [null, NaN, Infinity]) {
    const bar = render(value)
    assert.equal(bar.hasAttribute('aria-valuenow'), false)
    assert.equal(bar.classList.contains('indeterminate'), true)
    assert.equal(bar.firstElementChild.hasAttribute('style'), false)
  }
})

test('both update screens use the same animated bar with reduced-motion support', () => {
  const component = readFileSync(new URL('../src/components/UpdatePreferences.tsx', import.meta.url), 'utf8')
  assert.equal([...component.matchAll(/<UpdateProgressBar key=\{progress.phase\} label=\{progress.title\} value=\{progress.value\}/g)].length, 2)
  assert.doesNotMatch(component, /<progress\b/)
  const css = readFileSync(new URL('../src/style.css', import.meta.url), 'utf8')
  assert.match(css, /\.update-progress-fill\s*\{[^}]*transition:\s*transform 500ms linear/)
  assert.match(css, /\.update-progress\.indeterminate \.update-progress-fill\s*\{[^}]*animation:\s*update-progress-sweep/)
  assert.match(css, /@media \(prefers-reduced-motion:\s*reduce\)\s*\{\s*\.update-progress-fill\s*\{\s*transition:\s*none/)
})
