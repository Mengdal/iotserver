/**
 * 安全 JSON 解析：接口/历史数据里的字符串字段可能是空串或脏数据，
 * 直接 JSON.parse 会让整页渲染或点击事件抛异常。解析失败时返回 fallback。
 */
export function safeJsonParse <T> (value: unknown, fallback: T): T {
  if (typeof value !== 'string' || !value.trim()) {
    return fallback
  }
  try {
    const parsed = JSON.parse(value)
    return parsed === null || parsed === undefined ? fallback : (parsed as T)
  } catch {
    return fallback
  }
}
