import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'

const wizard = readFileSync(new URL('../src/components/CreateServerWizard.tsx', import.meta.url), 'utf8')
test('existing installations bypass provisioning and save only a local profile', () => {
  assert.match(wizard, /if \(probe.installed \|\| probe.running\)/)
  const existing = wizard.slice(wizard.indexOf("if (step === 'existing')"), wizard.indexOf("if (step === 'config')"))
  assert.match(existing, /SaveServer\(existingDraft\)/)
  assert.doesNotMatch(existing, /SetupInstall|SetupStart|SetupWgSetup|SetupCreateServer/)
  assert.ok(wizard.indexOf('const fresh = await SetupProbe(ssh)') < wizard.indexOf('await SetupCreateContainer(ssh'))
  assert.match(wizard, /if \(fresh.installed \|\| fresh.running\)/)
})

test('container reuse creates a device, never reinstalls its server', () => {
  const existing = wizard.slice(wizard.indexOf("if (step === 'existing')"), wizard.indexOf("if (step === 'config')"))
  assert.match(existing, /SetupPrepareContainer\(ssh, containerId, deviceName\)/)
  assert.match(existing, /disabled=\{!c.supported \|\| !c.running\}/)
  assert.doesNotMatch(existing, /SetupCreateContainer/)
  assert.doesNotMatch(wizard, /await SetupInstall|await SetupStart|await SetupWgSetup/)
})

test('container endpoints are reviewed before local save, never silently replaced', () => {
  const review = wizard.slice(wizard.indexOf("if (step === 'review'"), wizard.indexOf("if (step === 'existing')"))
  assert.match(review, /containerPreview.needsReview/)
  assert.match(review, /peer: containerPreview.advertisedPeer/)
  assert.match(review, /peer: containerPreview.sshPeer/)
  assert.match(review, /SaveServer\(existingDraft\)/)
  assert.match(wizard, /setExistingDraft\(\{\.\.\.preview.server, peer: ''\}/)
  assert.match(review, /disabled=\{busy \|\| !existingDraft.peer\}/)
  assert.doesNotMatch(review, /SetupPrepareContainer|SetupAttachContainer|SetupStart|SetupInstall/)
  const app = readFileSync(new URL('../../app.go', import.meta.url), 'utf8')
  const prepare = app.slice(app.indexOf('func (a *App) SetupPrepareContainer'), app.indexOf('func (a *App) SetupCreateContainer'))
  assert.match(prepare, /BuildContainerPreview/)
  assert.doesNotMatch(prepare, /store.Save|serversetup.Start|serversetup.Install/)
})

test('matching container endpoints save immediately without asking for a choice', () => {
  const start = wizard.indexOf('if (!preview.needsReview)')
  assert.ok(start >= 0)
  const fastPath = wizard.slice(start, wizard.indexOf('setContainerPreview(preview)', start))
  assert.match(fastPath, /onCreated\(await SaveServer\(preview.server\)\)/)
  assert.match(fastPath, /return/)
  assert.doesNotMatch(fastPath, /setStep\('review'\)/)
})
