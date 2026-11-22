import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {resolveStartupProfile} from '../src/autoConnectSettings.ts'

test('startup choice keeps a saved live profile even when another profile is selected', () => {
  assert.equal(resolveStartupProfile([{id:'a'},{id:'b'}],'a','b'),'a')
})

test('a deleted or empty startup ID uses the visible live profile before the first entry', () => {
  for (const stale of ['', 'deleted']) assert.equal(resolveStartupProfile([{id:'a'},{id:'b'}],stale,'b'),'b')
  assert.equal(resolveStartupProfile([{id:'a'},{id:'b'}],'deleted','also-deleted'),'a')
  assert.equal(resolveStartupProfile([{id:'b'}],'a','a'),'b')
})

test('empty profile lists and unsaved drafts never produce an invalid startup ID', () => {
  assert.equal(resolveStartupProfile([],'deleted','a'),'')
  assert.equal(resolveStartupProfile([{id:''},{id:' '}],'',''),'')
  assert.equal(resolveStartupProfile([{id:''},{id:'saved'}],'',''),'saved')
})

test('startup resolution has no persistence or connection side effects and follows list changes', () => {
  const profiles=[{id:'a'},{id:'b'}],before=JSON.stringify(profiles)
  assert.equal(resolveStartupProfile(profiles,'b','a'),'b')
  assert.equal(JSON.stringify(profiles),before)
  assert.equal(resolveStartupProfile(profiles.filter(p=>p.id!=='b'),'b','b'),'a')
  assert.equal(resolveStartupProfile([...profiles,{id:'new'}],'','new'),'new')
})

test('settings expose the startup selector before enabling and submit its resolved ID', () => {
  const read=file=>readFileSync(new URL('../src/'+file,import.meta.url),'utf8')
  const preferences=read('components/DesktopPreferences.tsx')
  assert.match(preferences,/\(settings.autoConnect \|\| servers.length>0\) &&/)
  assert.match(preferences,/SetAutoConnect\(value,value\?startupProfileId:settings.autoConnectServerId\)/)
  assert.match(preferences,/settings.autoConnect&&missingStartupProfile\?'':startupProfileId/)
  assert.doesNotMatch(preferences,/settings.autoConnectServerId\|\|servers\[0\]/)
  assert.match(read('App.tsx'),/selectedServerId=\{selectedId\|\|undefined\}/)
  assert.doesNotMatch(preferences,/\bConnect\(/)
})
