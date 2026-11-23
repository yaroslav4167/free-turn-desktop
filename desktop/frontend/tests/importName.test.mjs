import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mkdtempSync, rmSync} from 'node:fs'
import {fileURLToPath, pathToFileURL} from 'node:url'
import {join} from 'node:path'
import {build} from 'esbuild'
import {JSDOM} from 'jsdom'
import React, {act} from 'react'

test('link names fill the import form, follow the current link and preserve manual edits', async t => {
  const dir = mkdtempSync(fileURLToPath(new URL('./.import-name-', import.meta.url)))
  t.after(() => rmSync(dir, {recursive: true, force: true}))
  const output = join(dir, 'modal.mjs')
  await build({entryPoints: [fileURLToPath(new URL('../src/components/AddServerModal.tsx', import.meta.url))],
    outfile: output, bundle: true, format: 'esm', platform: 'node', jsx: 'automatic', external: ['react', 'react-dom', 'react-dom/*'],
    plugins: [{name: 'import-api', setup(b) {
      b.onResolve({filter: /\/api$/}, () => ({path: 'api', namespace: 'mock'}))
      b.onResolve({filter: /\/CreateServerWizard$/}, () => ({path: 'wizard', namespace: 'mock'}))
      b.onLoad({filter: /.*/, namespace: 'mock'}, ({path}) => ({loader: 'js', contents: path === 'wizard' ? 'export default function Wizard(){return null}' : `
        export const ParseLink=raw=>new Promise((resolve,reject)=>globalThis.importNameTest.requests.push({raw,resolve,reject}));
        export const NewServer=async()=>({name:'',vkLinks:[]});
        export const ImportLink=async(raw,base)=>{globalThis.importNameTest.imported.push({raw,base});return {...base,id:'saved'}};
        export const SaveServer=async base=>base;
      `}))
    }}]})
  const dom = new JSDOM('<div id="root"></div>', {url: 'http://localhost'})
  for (const name of ['window', 'document', 'HTMLElement', 'HTMLDetailsElement', 'Node']) globalThis[name] = dom.window[name]
  globalThis.requestAnimationFrame = cb => setTimeout(cb, 0)
  globalThis.cancelAnimationFrame = clearTimeout
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  globalThis.importNameTest = {requests: [], imported: []}
  const {createRoot} = await import('react-dom/client')
  const {default: Modal} = await import(pathToFileURL(output))
  const root = createRoot(document.getElementById('root'))
  t.after(async () => { await act(async () => root.unmount()); dom.window.close(); delete globalThis.importNameTest })
  await act(async () => root.render(React.createElement(Modal, {onClose: () => {}, onCreated: () => {}})))
  const edit = async (selector, value) => act(async () => {
    const element = document.querySelector(selector)
    const prototype = element.tagName === 'TEXTAREA' ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype
    Object.getOwnPropertyDescriptor(prototype, 'value').set.call(element, value)
    element.dispatchEvent(new window.Event('input', {bubbles: true}))
  })
  const parse = async raw => {
    await edit('textarea', raw)
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 280)) })
    return importNameTest.requests.at(-1)
  }
  const preview = name => ({name, peer: '203.0.113.5:56000', vkLink: '', wgConf: ''})
  const field = () => document.getElementById('import-profile-name')
  const first = await parse('freeturn://first')
  await act(async () => first.resolve(preview('Германия 🇩🇪')))
  assert.equal(field().value, 'Германия 🇩🇪')
  const importButton = () => [...document.querySelectorAll('button')].find(button => button.textContent === 'Импортировать')
  await act(async () => importButton().click())
  assert.equal(importNameTest.imported.at(-1).base.name, 'Германия 🇩🇪')
  const unnamed = await parse('freeturn://unnamed')
  await act(async () => unnamed.resolve(preview('')))
  assert.equal(field().value, '')
  const stale = await parse('freeturn://stale')
  const current = await parse('freeturn://current')
  await act(async () => current.resolve(preview('Canada')))
  await act(async () => stale.resolve(preview('Stale name')))
  assert.equal(field().value, 'Canada')
  const edited = await parse('freeturn://edited')
  await edit('#import-profile-name', 'Мой сервер')
  await act(async () => edited.resolve(preview('Name from link')))
  assert.equal(field().value, 'Мой сервер')
  await act(async () => importButton().click())
  assert.equal(importNameTest.imported.at(-1).base.name, 'Мой сервер')
  assert.equal(importNameTest.imported.at(-1).raw, 'freeturn://edited')
})
