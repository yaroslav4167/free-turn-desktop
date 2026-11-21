import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'

const script = readFileSync(new URL('../../../internal/provider/vk/internal/captcha/manual/inject.js', import.meta.url), 'utf8')

function setup() {
  const requests=[], events=new Map(), timers=[]
  class Element {
    constructor(tag='DIV') {this.tagName=tag;this.attributes={};this.children=[];this.style={};this.hidden=false}
    setAttribute(name,value) {this.attributes[name]=value}
    getAttribute(name) {return this.attributes[name]}
    hasAttribute(name) {return Object.hasOwn(this.attributes,name)}
    querySelectorAll() {return []}
    append(...children) {this.children.push(...children)}
    addEventListener(name,callback) {this[name]=callback}
  }
  const resource = (tag,property) => {
    class Resource extends Element {constructor(){super(tag)}}
    Object.defineProperty(Resource.prototype,property,{configurable:true,get(){return this.attributes[property]},set(value){this.attributes[property]=value;requests.push(value)}})
    return Resource
  }
  class XHR {open(...args){this.args=args} send(){} }
  class Form extends Element {submit(){}}
  const root=new Element(), body=new Element('BODY')
  root.childElementCount=0
  const document={body,documentElement:new Element(),createElement:tag=>new Element(tag.toUpperCase()),querySelectorAll:()=>[],addEventListener:()=>{},getElementById:id=>id==='spa_root'?root:null}
  const calls=[]
  const window={__ftpCaptcha:{local:'http://localhost:8765',upstream:'https://id.vk.ru'},fetch:(...args)=>{calls.push(args);return Promise.resolve({})},open(){},addEventListener:(name,callback)=>events.set(name,callback)}
  const context={window,document,Element,HTMLScriptElement:resource('SCRIPT','src'),HTMLLinkElement:resource('LINK','href'),HTMLImageElement:resource('IMG','src'),HTMLIFrameElement:resource('IFRAME','src'),HTMLFormElement:Form,XMLHttpRequest:XHR,MutationObserver:class{observe(){}},URL,Request,Blob,navigator:{},location:{protocol:'http:',href:'http://localhost:8765/captcha',reload(){}},setTimeout:callback=>timers.push(callback)}
  vm.runInNewContext(script,context)
  return {context,requests,calls,events,timers,body,root}
}

test('manual captcha rewrites dynamic resource setters before a request and retains HTTPS for scheme-relative URLs', () => {
  const {context,requests}=setup()
  const script=new context.HTMLScriptElement()
  script.src='//st.vk.ru/widget.js'
  assert.equal(requests[0],'/generic_proxy?proxy_url='+encodeURIComponent('https://st.vk.ru/widget.js'))
  const element=new context.Element('SCRIPT')
  element.setAttribute('src','https://st.vk.com/widget.js')
  assert.equal(element.getAttribute('src'),'/generic_proxy?proxy_url='+encodeURIComponent('https://st.vk.com/widget.js'))
  script.src='https://id.vk.ru/asset.js'
  assert.equal(requests[1],'http://localhost:8765/asset.js')
  script.src='https://id.vk.ru.evil.test/asset.js'
  assert.equal(requests[2],'/generic_proxy?proxy_url='+encodeURIComponent('https://id.vk.ru.evil.test/asset.js'))
})

test('manual captcha fetch accepts Request and URL objects without losing method, headers or body', async () => {
  const {context,calls}=setup()
  const input=new Request('https://api.vk.ru/test',{method:'POST',headers:{'X-Test':'kept'},body:'synthetic=data'})
  await context.window.fetch(input)
  const forwarded=calls[0][0]
  assert.ok(forwarded instanceof Request)
  assert.equal(forwarded.url,new URL('/generic_proxy?proxy_url='+encodeURIComponent(input.url),'http://localhost:8765').href)
  assert.equal(forwarded.method,'POST')
  assert.equal(forwarded.headers.get('X-Test'),'kept')
  assert.equal(await forwarded.text(),'synthetic=data')
  await context.window.fetch(new URL('https://st.vk.ru/data'))
  assert.equal(calls[1][0],'/generic_proxy?proxy_url='+encodeURIComponent('https://st.vk.ru/data'))
})

test('manual captcha does not black out the VK page and explains an empty widget with a reload action', () => {
  const {body,timers}=setup()
  assert.equal(body.style.background,undefined)
  const banner=body.children[0], [text,retry]=banner.children
  assert.match(text.textContent,/Loading VK verification/)
  assert.equal(retry.hidden,true)
  timers[0]()
  assert.match(text.textContent,/VK widget did not appear/)
  assert.equal(retry.hidden,false)
  assert.equal(typeof retry.click,'function')
})

test('required VK resource errors reveal only the host, not captcha query values', () => {
  const {events,body}=setup()
  const [text,retry]=body.children[0].children
  events.get('error')({target:{tagName:'SCRIPT',src:'http://localhost:8765/generic_proxy?proxy_url='+encodeURIComponent('https://st.vk.ru/widget.js?token=PRIVATE_DEMO')}})
  assert.match(text.textContent,/st.vk.ru/)
  assert.doesNotMatch(text.textContent,/PRIVATE_DEMO|token=/)
  assert.equal(retry.hidden,false)
})

test('optional advertising errors do not claim the captcha failed', () => {
  const {events,body}=setup()
  events.get('error')({target:{tagName:'SCRIPT',src:'https://ad.mail.ru/optional.js'}})
  assert.equal(body.children[0].children[1].hidden,true)
})

test('loaded widget is not permanently covered by diagnostics; script failures restore the status', () => {
  const {events,body,timers,root}=setup()
  root.childElementCount=1
  timers[0]()
  assert.equal(body.children[0].hidden,true)
  events.get('error')({filename:'http://localhost:8765/generic_proxy?proxy_url='+encodeURIComponent('https://st.vk.ru/widget.js?sid=private-demo')})
  assert.equal(body.children[0].hidden,false)
  assert.match(body.children[0].children[0].textContent,/VK script error/)
  assert.doesNotMatch(body.children[0].children[0].textContent,/private-demo/)
})
