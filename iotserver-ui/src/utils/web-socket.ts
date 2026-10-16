
import qs from 'qs'

interface WebSocketOptions {
  url?: string
  path?: string
  query?: object
  timeout?: number,
  reconnection?: boolean,
  reconnectionDelay?: number,
  reconnectionDelayMax?: number,
  reconnectionAttempts?: number,
}

interface BackPackage {
  code: number
  data: {
    errorCode: number
    errorMsg?: string
    successMsg?: string
    result: any
    success: boolean
  }
}

interface SendMessage {
  code: number
  data: any
}

interface CallbackMap {
  resolveMap?: ((value: BackPackage) => void)[]
  successCallback?: (value: BackPackage) => void
}

export class WebSocketClass {
  private websocket: WebSocket
  private lockReconnect: boolean = false
  private reconnectionAttempts: number = 0
  private callbackMap: Map<number, CallbackMap> = new Map()
  private timer: ReturnType<typeof setInterval> | undefined = undefined

  private defaults: WebSocketOptions = {
    // url: `wss://${location.host}`,
    url: location.protocol === 'https:' ? `wss://${location.host}` : `ws://${location.host}`,
    reconnection: true,
    reconnectionDelay: 1000,
    reconnectionDelayMax: 5000,
    reconnectionAttempts: Infinity,
  }

  private get wsurl () {
    const { url, path, query } = this.options
    return `${url}${path}?${qs.stringify(query)}`
  }

  constructor (
    private options: WebSocketOptions,
  ) {
    this.options = Object.assign({}, this.defaults, this.options)
    this.websocket = new WebSocket(this.wsurl)
    this.bindEvents()
  }

  private bindEvents () {
    this.websocket.addEventListener('open', this.onopen.bind(this))
    this.websocket.addEventListener('error', this.onerror.bind(this))
    this.websocket.addEventListener('close', this.onclose.bind(this))
    this.websocket.addEventListener('message', this.onmessage.bind(this))
  }

  private unbindEvents () {
    this.websocket.removeEventListener('open', this.onopen.bind(this))
    this.websocket.removeEventListener('error', this.onerror.bind(this))
    this.websocket.removeEventListener('close', this.onclose.bind(this))
    this.websocket.removeEventListener('message', this.onmessage.bind(this))
  }

  private reconnect () {
    console.log('reconnect', this.lockReconnect)
    if (this.lockReconnect) return

    setTimeout(() => {
      this.lockReconnect = false
      this.reconnectionAttempts++
      this.unbindEvents()
      this.websocket = new WebSocket(this.wsurl)
      this.bindEvents()
    }, this.options.reconnectionDelay)
  }

  private onopen () {
    console.log('[ws] onopen')
    if (this.timer) {
      clearInterval(this.timer)
    }
  }

  private onerror () {
    console.log('[ws] onerror', this)
    const { reconnection, reconnectionAttempts, reconnectionDelay } = this.options

    if (reconnection && reconnectionAttempts && this.reconnectionAttempts < reconnectionAttempts) {
      setTimeout(() => {
        this.reconnect()
      }, reconnectionDelay)
    }
  }

  private onclose () {
    console.log('[ws] onclose')
  }

  private onmessage (event: MessageEvent<any>) {
    console.log('[ws] onmessage')
    this.lockReconnect = true
    try {
      const data = JSON.parse(event.data)
      const { resolveMap = [], successCallback } = this.callbackMap.get(data?.code) || {}
      resolveMap.pop()?.(data)

      if (typeof successCallback === 'function') {
        successCallback?.(data)
      }
    } catch (error) {
      console.log('[ws] onmessage error', error)
    }
  }

  /**
   * 发送请求并等待后端同 code 的响应。
   *
   * 修复了原实现的两个缺陷，否则调用方会「转圈转到天荒地老」：
   *   1. socket 未 OPEN 时 websocket.send 会抛 InvalidStateError，
   *      原实现没捕获、调用方也没 .catch，异常被静默吞掉；
   *   2. 后端不响应时 Promise 永不 settle，调用方的 loading 永远结束不了。
   *      这里加超时兜底（默认 120s，够拉一个镜像）。
   */
  public send (message: SendMessage, timeoutMs = 120 * 1000): Promise<BackPackage> {
    return new Promise((resolve, reject) => {
      // 1. 连接不可用时立即失败，而不是静默丢弃
      if (this.websocket.readyState !== WebSocket.OPEN) {
        reject(new Error('WebSocket 未连接，请刷新页面后重试'))
        return
      }

      const postMessage = typeof message === 'string' ? message : JSON.stringify(message)
      const callbackMapCopy = this.callbackMap.get(message?.code) || {}
      const oldResolveMap = callbackMapCopy.resolveMap || []

      // 用函数声明（会提升）而不是 const：wrapped 与 timer 互相引用，
      // 这样不会触发 TDZ（运行时机上 wrapped 只在 timer 赋值后才被调用）
      function wrapped (data: BackPackage) {
        clearTimeout(timer)
        resolve(data)
      }

      const timer = setTimeout(() => {
        // 超时后摘掉该 resolver，避免响应迟到时重复 resolve
        const cur = this.callbackMap.get(message?.code)
        if (cur?.resolveMap) {
          cur.resolveMap = cur.resolveMap.filter(fn => fn !== wrapped)
        }
        reject(new Error('请求超时（后端未响应），请查看服务端日志'))
      }, timeoutMs)

      callbackMapCopy.resolveMap = [...oldResolveMap, wrapped]
      this.callbackMap.set(message?.code, callbackMapCopy)

      try {
        this.websocket.send(postMessage)
      } catch (e) {
        clearTimeout(timer)
        reject(e instanceof Error ? e : new Error('消息发送失败'))
      }
    })
  }

  public sendSync (message: SendMessage & {
    success: (result: BackPackage) => void
  }): void {
    const postMessage = typeof message === 'string' ? message : JSON.stringify(message)
    const callbackMapCopy = this.callbackMap.get(message?.code) || {}
    callbackMapCopy.successCallback = message.success
    this.callbackMap.set(message?.code, callbackMapCopy)
    this.websocket.send(postMessage)
  }

  public close () {
    this.unbindEvents()
    this.websocket.close(1000, 'CLOSE_NORMAL')
  }
}

export default WebSocketClass
