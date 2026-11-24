import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import zh from './zh'
import en from './en'

const savedLang = localStorage.getItem('walrus-lang') || 'zh-CN'

i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zh },
    'en-US': { translation: en },
  },
  lng: savedLang,
  fallbackLng: 'zh-CN',
  interpolation: {
    escapeValue: false,
  },
})

export function setLanguage(lang: 'zh-CN' | 'en-US') {
  localStorage.setItem('walrus-lang', lang)
  document.documentElement.lang = lang
  return i18n.changeLanguage(lang)
}

export default i18n
