import { getToken } from './auth'

// export const wsHost = `ws://${location.host}`
export const wsHost = location.protocol === 'https:' ? `wss://${location.host}` : `ws://${location.host}`
// export const xToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJRCI6MSwiVXNlcm5hbWUiOiJhZG1pbiIsImV4cCI6MTY4ODE4NDEyMSwiaXNzIjoiZWRnZS1nYXRld2F5IiwibmJmIjoxNjcwMDM5MTIxfQ.R4R6q49GD51yi0HAlRwFFnNgpNaBTlpjrTmhqj19Wo0'
// 不能在模块加载时求值：页面在登录前就被静态导入，会缓存 null/旧账号 token。
// 建立 WebSocket 连接时实时读取。
export const getXToken = () => getToken()
