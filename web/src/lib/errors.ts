import i18n from '@/i18n'

export interface ApiErrorPayload {
  code?: unknown
  params?: unknown
  error?: unknown
  status?: number
}

const SHARD_KEY_MAP: Record<string, string> = {
  master: 'common.masterShort',
  caves: 'common.cavesShort',
}

const STATE_KEY_MAP: Record<string, string> = {
  running: 'common.running',
  starting: 'common.starting',
  stopping: 'common.stopping',
  stopped: 'common.stopped',
  crashed: 'common.crashed',
}

function localizeParams(raw: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
  if (!raw) return undefined
  const out: Record<string, unknown> = { ...raw }
  if (typeof out.shard === 'string') {
    const key = SHARD_KEY_MAP[out.shard.toLowerCase()]
    if (key) out.shard = i18n.t(key)
  }
  if (typeof out.state === 'string') {
    const key = STATE_KEY_MAP[out.state.toLowerCase()]
    if (key) out.state = i18n.t(key)
  }
  return out
}

export function formatApiError(payload: ApiErrorPayload): string {
  const code = typeof payload.code === 'string' && payload.code.trim() ? payload.code.trim() : undefined
  const params =
    payload.params && typeof payload.params === 'object' && !Array.isArray(payload.params)
      ? (payload.params as Record<string, unknown>)
      : undefined
  const fallback = typeof payload.error === 'string' && payload.error.trim() ? payload.error.trim() : undefined

  if (code) {
    const key = `errors.${code}`
    if (i18n.exists(key)) {
      return i18n.t(key, localizeParams(params))
    }
  }
  if (fallback) {
    return fallback
  }
  if (typeof payload.status === 'number' && payload.status > 0) {
    return i18n.t('errors.HTTP_ERROR', { status: payload.status })
  }
  return i18n.t('errors.UNKNOWN_ERROR')
}

export class ApiError extends Error {
  status: number
  code?: string
  params?: Record<string, unknown>
  rawMessage: string

  constructor(
    message: string,
    status: number,
    options?: { code?: string; params?: Record<string, unknown>; rawMessage?: string },
  ) {
    const raw = options?.rawMessage ?? message
    const localized = formatApiError({
      code: options?.code,
      params: options?.params,
      error: raw,
      status,
    })
    super(localized)
    this.name = 'ApiError'
    this.status = status
    this.code = options?.code
    this.params = options?.params
    this.rawMessage = raw
  }

  getLocalizedMessage(): string {
    return formatApiError({
      code: this.code,
      params: this.params,
      error: this.rawMessage,
      status: this.status,
    })
  }
}

export function buildApiError(data: unknown, status: number): ApiError {
  const obj = typeof data === 'object' && data !== null ? (data as Record<string, unknown>) : {}
  const code = typeof obj.code === 'string' && obj.code.trim() ? obj.code.trim() : undefined
  const params =
    obj.params && typeof obj.params === 'object' && !Array.isArray(obj.params)
      ? (obj.params as Record<string, unknown>)
      : undefined
  const rawMessage = typeof obj.error === 'string' && obj.error.trim() ? obj.error.trim() : `HTTP ${status}`
  return new ApiError(rawMessage, status, { code, params, rawMessage })
}

export function formatError(err: unknown): string {
  if (err instanceof ApiError) {
    return err.getLocalizedMessage()
  }
  if (err instanceof TypeError) {
    const msg = err.message.toLowerCase()
    if (msg.includes('fetch') || msg.includes('network') || msg.includes('load failed')) {
      return i18n.t('errors.NETWORK_ERROR')
    }
  }
  if (err instanceof Error) {
    return err.message || i18n.t('errors.UNKNOWN_ERROR')
  }
  if (err && typeof err === 'object') {
    const obj = err as ApiErrorPayload
    if (obj.code || obj.error) {
      return formatApiError(obj)
    }
  }
  return String(err || i18n.t('errors.UNKNOWN_ERROR'))
}
