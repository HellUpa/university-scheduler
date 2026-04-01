import { createI18n } from 'vue-i18n'
import ru from './locales/ru.json'
import kk from './locales/kk.json'

// 1. Говорим TypeScript: "Смотри, эталоном структуры будет русский файл"
type MessageSchema = typeof ru

// 2. Явно указываем типы: <[Схема], 'список' | 'доступных' | 'языков'>
export const i18n = createI18n<[MessageSchema], 'ru' | 'kk'>({
  legacy: false, 
  locale: 'ru', 
  fallbackLocale: 'ru',
  messages: {
    ru,
    kk
  }
})