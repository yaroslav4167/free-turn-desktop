import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mkdtempSync, rmSync} from 'node:fs'
import {fileURLToPath, pathToFileURL} from 'node:url'
import {join} from 'node:path'
import {build} from 'esbuild'
import {JSDOM} from 'jsdom'
import React, {act} from 'react'
import {createRoot} from 'react-dom/client'

// Exercise App's real action handlers, Sidebar and confirmation dialog with a
// synthetic backend. No saved profiles, credentials or real VPN are accessed.
test('profile actions work without browser confirm and target the context-menu profile', async t => {
  const dir = mkdtempSync(fileURLToPath(new URL('./.profile-actions-', import.meta.url)))
  t.after(() => rmSync(dir, {recursive:true, force:true}))
  const output = join(dir, 'app.mjs')
  await build({entryPoints:[fileURLToPath(new URL('../src/App.tsx', import.meta.url))], outfile:output, bundle:true, format:'esm', platform:'node', logOverride:{'duplicate-object-key':'silent'}, jsx:'automatic', external:['react','react-dom','react-dom/*'], plugins:[{
    name:'synthetic-backend', setup(builder) {
      builder.onResolve({filter:/\/api$/}, () => ({path:'api',namespace:'mock'}))
      builder.onResolve({filter:/\/desktop$/}, () => ({path:'desktop',namespace:'mock'}))
      builder.onResolve({filter:/\/components\//}, args => {
        const name=args.path.split('/').at(-1)
        if (!['Sidebar','ConfirmDialog'].includes(name)) return {path:name,namespace:'mock'}
      })
      builder.onLoad({filter:/.*/,namespace:'mock'}, args => {
        let contents
        if(args.path==='api') contents=`export const backend={Server:{createFrom:s=>({...s})}}; export const ListServers=async()=>globalThis.profileTest.servers; export const CoreVersion=async()=>"test"; export const Status=async()=>globalThis.profileTest.status||({state:"idle",serverId:"",txTotal:0,rxTotal:0}); export const EventsOn=()=>()=>{}; export const EventsOff=()=>{}; export const SaveServer=async s=>s; export const Connect=async id=>{globalThis.profileTest.connected.push(id)}; export const Disconnect=async()=>{globalThis.profileTest.disconnected++}; export const DeleteServer=async id=>{if(globalThis.profileTest.failDelete)throw Error("delete failed");globalThis.profileTest.deleted.push(id)}`
        else if(args.path==='desktop') contents='export const desktop=()=>({ProfileRecoveryNotice:async()=>"",DesktopSettings:async()=>({})})'
        else if(args.path==='Navigation') contents='export default function Navigation({onChange}){return <button onClick={()=>onChange("servers")}>Profiles</button>}'
        else if(args.path==='ServerDetail') contents='export default function Detail(p){return <div data-detail={p.server.id}><input aria-label="Name" value={p.server.name} onInput={e=>p.onChange({...p.server,name:e.target.value})}/><button onClick={p.onDelete}>Delete selected</button></div>}'
        else if(args.path==='UpdatePreferences') contents='export const UpdateNotice=()=>null'
        else contents='export default ()=>null'
        return {contents,loader:'tsx'}
      })
    }
  }]})
  const dom = new JSDOM('<div id="root"></div>',{url:'http://localhost'})
  for(const name of ['window','document','HTMLElement','HTMLDetailsElement','Node','localStorage']) globalThis[name]=dom.window[name]
  globalThis.requestAnimationFrame=cb=>setTimeout(cb,0)
  globalThis.cancelAnimationFrame=clearTimeout
  globalThis.IS_REACT_ACT_ENVIRONMENT=true
  window.confirm=()=>{throw Error('native browser confirm must not be used')}
  globalThis.profileTest={servers:[{id:'a',name:'Alpha',peer:'192.0.2.1'},{id:'b',name:'Beta',peer:'192.0.2.2'}],deleted:[],connected:[],disconnected:0}
  const {default:App}=await import(pathToFileURL(output))
  const root=createRoot(document.getElementById('root'))
  t.after(async()=>{await act(async()=>root.unmount());dom.window.close();delete globalThis.profileTest})
  const click=async button=>{assert.ok(button);await act(async()=>button.dispatchEvent(new window.MouseEvent('click',{bubbles:true})))}
  const button=(text,scope=document)=>[...scope.querySelectorAll('button')].find(b=>b.textContent===text)
  const context=async id=>{await act(async()=>document.querySelectorAll('.server-item')[id].dispatchEvent(new window.MouseEvent('contextmenu',{bubbles:true,clientX:20,clientY:20})));return document.querySelector('[role="menu"]')}
  await act(async()=>root.render(React.createElement(App)))
  await click(button('Profiles'))
  const input=document.querySelector('input[aria-label="Name"]')
  await act(async()=>{input.value='Edited';input.dispatchEvent(new window.Event('input',{bubbles:true}))})
  await click(document.querySelectorAll('.server-item')[1])
  assert.ok(document.querySelector('[role="alertdialog"]'))
  await click(button('Отмена'))
  assert.equal(document.querySelector('[data-detail]').dataset.detail,'a')
  assert.equal(document.querySelector('input').value,'Edited')
  await click(document.querySelectorAll('.server-item')[1])
  await click(button('Продолжить'))
  assert.equal(document.querySelector('[data-detail]').dataset.detail,'b')
  await click(document.querySelectorAll('.server-item')[0])
  await click(button('Delete selected'))
  assert.match(document.querySelector('[role="alertdialog"]').textContent,/Alpha/)
  await click(button('Отмена'))
  assert.deepEqual(profileTest.deleted,[])
  let menu=await context(1)
  await click(button('Подключиться',menu))
  assert.deepEqual(profileTest.connected,['b'])
  assert.equal(document.querySelector('[data-detail]').dataset.detail,'b')
  menu=await context(0)
  await click(button('Удалить профиль',menu))
  assert.match(document.querySelector('[role="alertdialog"]').textContent,/Alpha/)
  await click(button('Удалить профиль',document.querySelector('[role="alertdialog"]')))
  assert.deepEqual(profileTest.deleted,['a'])
  assert.equal(document.querySelector('[data-detail]').dataset.detail,'b')
  assert.equal(document.querySelectorAll('.server-item').length,1)
  profileTest.failDelete=true
  await click(button('Delete selected'))
  await click(button('Удалить профиль',document.querySelector('[role="alertdialog"]')))
  assert.match(document.querySelector('[role="alert"]').textContent,/delete failed/)
  assert.equal(document.querySelectorAll('.server-item').length,1)
  profileTest.failDelete=false
  await click(button('Delete selected'))
  await click(button('Удалить профиль',document.querySelector('[role="alertdialog"]')))
  assert.deepEqual(profileTest.deleted,['a','b'])
  assert.equal(document.querySelectorAll('.server-item').length,0)
  profileTest.status={state:'connected',serverId:'a',txTotal:0,rxTotal:0}
  await act(async()=>root.render(React.createElement(App,{key:'active'})))
  await click(button('Profiles'))
  menu=await context(0)
  assert.equal(button('Удалить профиль',menu).disabled,true)
  await click(button('Отключить',menu))
  assert.equal(profileTest.disconnected,1)
})
