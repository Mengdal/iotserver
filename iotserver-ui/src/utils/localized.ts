import { currentLocale } from '@/i18n'

/** 中英文字段按当前语言取值：英文界面优先英文，缺省回落中文 */
export function localized (zh?: string | null, en?: string | null): string {
  return currentLocale() === 'en-US' ? (en || zh || '') : (zh || '')
}
