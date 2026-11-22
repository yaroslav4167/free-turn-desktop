import {tr} from './language'

export function progressLabel(stage: string): string {
  switch (stage) {
    case 'validation': return tr('Проверка профиля…', 'Validating profile…')
    case 'dns': return tr('Разрешение адресов DNS…', 'Resolving DNS addresses…')
    case 'routing-lists': return tr('Подготовка списков маршрутизации…', 'Preparing routing lists…')
    case 'helper': return tr('Запуск VPN-адаптера…', 'Starting VPN adapter…')
    case 'relay': return tr('Подключение к VK / TURN…', 'Connecting to VK / TURN…')
    case 'routes': return tr('Настройка маршрутов и DNS…', 'Configuring routes and DNS…')
    case 'captcha': return tr('Ожидание проверки VK…', 'Waiting for VK verification…')
    default: return ''
  }
}
