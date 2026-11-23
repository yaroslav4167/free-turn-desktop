import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mkdtempSync, rmSync} from 'node:fs'
import {fileURLToPath, pathToFileURL} from 'node:url'
import {join} from 'node:path'
import {build} from 'esbuild'
import {JSDOM} from 'jsdom'
import React, {act} from 'react'

test('WireGuard deletion confirms and share VK edits regenerate only the outgoing link', async t => {
  const dir=mkdtempSync(fileURLToPath(new URL('./.peer-actions-',import.meta.url)))
  t.after(()=>rmSync(dir,{recursive:true,force:true}))
  const output=join(dir,'peers.mjs')
  await build({entryPoints:[fileURLToPath(new URL('../src/components/PeersPanel.tsx',import.meta.url))],outfile:output,bundle:true,format:'esm',platform:'node',jsx:'automatic',external:['react','react-dom','react-dom/*'],logOverride:{'duplicate-object-key':'silent'},plugins:[{name:'synthetic-api',setup(b){
    b.onResolve({filter:/\/api$/},()=>({path:'api',namespace:'mock'}))
    b.onResolve({filter:/\/desktop$/},()=>({path:'desktop',namespace:'mock'}))
    b.onLoad({filter:/.*/,namespace:'mock'},({path})=>({loader:'js',contents:path==='api'?`
      export const CopyText=async text=>globalThis.peerTest.copied.push(text);
      export const ServerPeers=async()=>({peers:globalThis.peerTest.peers,serverNow:1000});
      export const ServerPeerRemove=async(id,pub)=>{if(globalThis.peerTest.failDelete)throw Error('delete failed');globalThis.peerTest.deleted.push({id,pub});globalThis.peerTest.peers=globalThis.peerTest.peers.filter(p=>p.pub!==pub)};
      export const ServerPeerAdd=async()=>{}; export const ServerPeerShare=async()=>{};
      export const backend={};export const serversetup={};
    `:`export const desktop=()=>({UpdateGuestShareVKLink:(raw,vk)=>new Promise((resolve,reject)=>globalThis.peerTest.requests.push({raw,vk,resolve,reject}))})`}))
  }}]})
  const dom=new JSDOM('<div id="root"></div>',{url:'http://localhost'})
  for(const name of ['window','document','HTMLElement','HTMLDetailsElement','Node'])globalThis[name]=dom.window[name]
  globalThis.requestAnimationFrame=cb=>setTimeout(cb,0)
  globalThis.cancelAnimationFrame=clearTimeout
  globalThis.IS_REACT_ACT_ENVIRONMENT=true
  window.confirm=()=>{throw Error('browser confirm is unsupported')}
  const {createRoot}=await import('react-dom/client')
  const {default:PeersPanel,ShareModal}=await import(pathToFileURL(output))
  const root=createRoot(document.getElementById('root'))
  t.after(async()=>{await act(async()=>root.unmount());dom.window.close();delete globalThis.peerTest})
  globalThis.peerTest={peers:[{pub:'owner',name:'Owner',ip:'10.0.0.1',isOwner:true},{pub:'guest',name:'Guest',ip:'10.0.0.2',isOwner:false}],deleted:[],requests:[],copied:[]}
  const button=(text,scope=document)=>[...scope.querySelectorAll('button')].find(b=>b.textContent===text)
  const click=async node=>{assert.ok(node);await act(async()=>node.click())}
  const server={id:'server-a',vkLinks:['https://vk.ru/call/join/owner']}
  await act(async()=>root.render(React.createElement(PeersPanel,{server})))
  assert.equal([...document.querySelectorAll('.peer-row')][0].querySelector('.danger'),null)
  await click(document.querySelector('.peer-row .danger'))
  assert.match(document.querySelector('[role="alertdialog"]').textContent,/Guest/)
  await click(button('Отмена'))
  assert.deepEqual(peerTest.deleted,[])
  peerTest.failDelete=true
  await click(document.querySelector('.peer-row .danger'))
  await click(button('Удалить',document.querySelector('[role="alertdialog"]')))
  assert.match(document.querySelector('.notice.error').textContent,/delete failed/)
  assert.equal(document.querySelectorAll('.peer-row').length,2)
  peerTest.failDelete=false
  await click(document.querySelector('.peer-row .danger'))
  await click(button('Удалить',document.querySelector('[role="alertdialog"]')))
  assert.deepEqual(peerTest.deleted,[{id:'server-a',pub:'guest'}])
  assert.equal(document.querySelectorAll('.peer-row').length,1)

  const original={link:'freeturn://base',qr:'data:image/png;base64,YQ==',includesVk:false}
  await act(async()=>root.render(React.createElement(ShareModal,{share:original,peerName:'Guest',defaultVKLink:server.vkLinks[0],onClose:()=>{}})))
  const copyButton=()=>button('Скопировать ссылку')
  assert.equal(document.querySelector('#share-vk-link'),null)
  await click(document.querySelector('input[type="checkbox"]'))
  assert.equal(document.querySelector('#share-vk-link').value,server.vkLinks[0])
  assert.equal(copyButton().disabled,true)
  assert.equal(document.querySelector('textarea').value,'')
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,330))})
  assert.equal(peerTest.requests[0].vk,server.vkLinks[0])
  const edit=async value=>act(async()=>{
    Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set.call(document.querySelector('#share-vk-link'),value)
    document.querySelector('#share-vk-link').dispatchEvent(new window.Event('input',{bubbles:true}))
  })
  await edit('https://vk.ru/call/join/guest')
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,330))})
  assert.equal(peerTest.requests[1].vk,'https://vk.ru/call/join/guest')
  await act(async()=>peerTest.requests[0].resolve({...original,link:'freeturn://stale',includesVk:true}))
  assert.equal(copyButton().disabled,true)
  await act(async()=>peerTest.requests[1].resolve({...original,link:'freeturn://guest',includesVk:true}))
  assert.equal(document.querySelector('textarea').value,'freeturn://guest')
  await click(copyButton())
  assert.deepEqual(peerTest.copied,['freeturn://guest'])
  assert.equal(server.vkLinks[0],'https://vk.ru/call/join/owner')
  await edit('')
  assert.equal(copyButton().disabled,true)
  assert.equal(document.querySelector('img'),null)
  await click(document.querySelector('input[type="checkbox"]'))
  assert.equal(document.querySelector('textarea').value,'')
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,330))})
  assert.equal(peerTest.requests.at(-1).vk,'')
  await act(async()=>peerTest.requests.at(-1).resolve(original))
  assert.equal(document.querySelector('textarea').value,original.link)
  assert.equal(copyButton().disabled,false)
})
