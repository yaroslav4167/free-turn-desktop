import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {bypassLocalNetworks,withLocalNetworkBypass} from '../src/localNetworkRouting.ts'

test('old and new profiles bypass LAN by default while an explicit opt-out survives', () => {
  assert.equal(bypassLocalNetworks({}),true)
  assert.equal(bypassLocalNetworks({routeLocalNetworks:false}),true)
  assert.equal(bypassLocalNetworks({routeLocalNetworks:true}),false)
  const original={id:'test',name:'Demo',routeLocalNetworks:false}
  const changed=withLocalNetworkBypass(original,false)
  assert.equal(changed.routeLocalNetworks,true)
  assert.equal(bypassLocalNetworks(withLocalNetworkBypass(changed,true)),true)
  assert.equal(original.routeLocalNetworks,false)
  assert.equal(changed.name,'Demo')
})

test('LAN switch belongs to VPN routing and honors connection edit locks', () => {
  const source=readFileSync(new URL('../src/components/VPNRoutingLists.tsx',import.meta.url),'utf8')
  assert.match(source,/checked=\{bypassLocalNetworks\(server\)\} disabled=\{disabled\}/)
  assert.match(source,/withLocalNetworkBypass\(server,e.target.checked\)/)
  const app=readFileSync(new URL('../src/App.tsx',import.meta.url),'utf8')
  assert.match(app,/\.\.\.saved, \.\.\.backend.Server.createFrom\(saved\)/)
})
