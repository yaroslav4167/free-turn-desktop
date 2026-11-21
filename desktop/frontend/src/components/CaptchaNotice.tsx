import {useEffect, useState} from 'react'
import {desktop} from '../desktop'
import {tr} from '../language'

export default function CaptchaNotice({url, connected}: {url: string, connected: boolean}) {
  const [error,setError]=useState('')
  useEffect(()=>setError(''),[url])
  const open=async(external=false)=>{
    setError('')
    try {await (external?desktop().OpenCaptchaExternal():desktop().OpenCaptcha())}
    catch(e){setError(String(e))}
  }
  if (!url) return null
  return <div id="captcha-notice" className="notice info app-error captcha-notice" role="status">
    <span>{connected
      ? tr('Соединение работает. Для части потоков требуется проверка VK.')
      : tr('Для подключения требуется проверка VK.', 'VK verification is required to connect.')}</span>
    <button className="btn small primary" onClick={() => open()}>
      {tr('Открыть проверку', 'Open verification')}
    </button>
    <button className="btn small" onClick={() => open(true)}>{tr('Открыть в браузере', 'Open in browser')}</button>
    {error&&<span role="alert">{error}</span>}
  </div>
}
