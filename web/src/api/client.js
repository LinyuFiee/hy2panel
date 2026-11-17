const JSON_CONTENT_TYPE = 'application/json'

export class ApiError extends Error {
  constructor(message, { status = 0, payload = null, cause } = {}) {
    super(message, cause ? { cause } : undefined)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

export class StaleSessionError extends Error {
  constructor() {
    super('请求所属会话已失效')
    this.name = 'StaleSessionError'
  }
}

async function readPayload(response) {
  const text = await response.text()
  if (!text) return {}

  try {
    return JSON.parse(text)
  } catch {
    return {}
  }
}

function forwardAbort(signal, controller) {
  if (!signal) return () => {}
  if (signal.aborted) {
    controller.abort(signal.reason)
    return () => {}
  }

  const abort = () => controller.abort(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  return () => signal.removeEventListener('abort', abort)
}

export function createApiClient({ onUnauthorized, fetchImpl = (...args) => fetch(...args) } = {}) {
  let generation = 0
  let unauthorizedGeneration = null
  const controllers = new Set()

  function abortInFlight(reason = 'session-reset') {
    for (const controller of controllers) controller.abort(reason)
    controllers.clear()
  }

  function resetSession() {
    generation += 1
    unauthorizedGeneration = null
    abortInFlight()
    return generation
  }

  async function request(path, options = {}) {
    const { json, signal, ...fetchOptions } = options
    const requestGeneration = generation
    const controller = new AbortController()
    const removeAbortForwarder = forwardAbort(signal, controller)
    controllers.add(controller)

    const headers = new Headers(fetchOptions.headers || {})
    headers.set('Accept', JSON_CONTENT_TYPE)
    headers.set('X-Requested-With', 'fetch')

    let body = fetchOptions.body
    if (json !== undefined) body = JSON.stringify(json)
    if (body !== undefined && body !== null && !headers.has('Content-Type')) {
      headers.set('Content-Type', JSON_CONTENT_TYPE)
    }

    let response
    let payload
    try {
      response = await fetchImpl(path, {
        ...fetchOptions,
        body,
        credentials: 'same-origin',
        mode: 'same-origin',
        headers,
        signal: controller.signal
      })
      payload = await readPayload(response)
    } catch (failure) {
      controllers.delete(controller)
      if (requestGeneration !== generation) throw new StaleSessionError()
      throw failure
    } finally {
      removeAbortForwarder()
    }

    if (requestGeneration !== generation) {
      controllers.delete(controller)
      throw new StaleSessionError()
    }

    if (response.status === 401 && path !== '/api/login') {
      const failure = new ApiError(payload.error || '请先登录', { status: 401, payload })
      if (unauthorizedGeneration !== requestGeneration) {
        unauthorizedGeneration = requestGeneration
        controllers.delete(controller)
        onUnauthorized?.(failure)
      } else {
        controllers.delete(controller)
      }
      throw failure
    }

    controllers.delete(controller)

    if (!response.ok) {
      throw new ApiError(payload.error || `请求失败 (${response.status})`, {
        status: response.status,
        payload
      })
    }

    return payload
  }

  return {
    request,
    resetSession,
    abortInFlight,
    currentGeneration: () => generation
  }
}

export function isCanceledRequest(failure) {
  return failure?.name === 'AbortError' || failure instanceof StaleSessionError
}
