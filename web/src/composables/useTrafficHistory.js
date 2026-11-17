import { ref } from 'vue'
import { isCanceledRequest } from '../api/client.js'

export const TRAFFIC_HISTORY_RANGES = Object.freeze(['1h', '6h', '24h', '7d'])

export function isTrafficHistoryRange(range) {
  return TRAFFIC_HISTORY_RANGES.includes(range)
}

export function appendNetworkSample(samples, status, limit = 30) {
  if (!status) return samples
  return [...samples, {
    rx: Number(status.rx_bytes_per_second || 0),
    tx: Number(status.tx_bytes_per_second || 0)
  }].slice(-limit)
}

export function useTrafficHistory({ api, canRequest, reportError }) {
  const networkSamples = ref([])
  const trafficLive = ref(null)
  const trafficHistory = ref(null)
  const trafficHistoryRange = ref('1h')
  const trafficHistoryLoading = ref(false)
  let liveRefreshing = false
  let historyRefreshing = false

  function sample(status) {
    networkSamples.value = appendNetworkSample(networkSamples.value, status)
  }

  async function refreshTrafficLive(showError = false) {
    if (liveRefreshing || !canRequest()) return false
    liveRefreshing = true
    try {
      trafficLive.value = await api.trafficLive()
      return true
    } catch (failure) {
      if (showError && failure.status !== 401 && !isCanceledRequest(failure)) reportError(failure.message)
      return false
    } finally {
      liveRefreshing = false
    }
  }

  async function loadTrafficHistory(range = trafficHistoryRange.value, showError = false) {
    if (!isTrafficHistoryRange(range) || historyRefreshing || !canRequest()) return false
    historyRefreshing = true
    trafficHistoryLoading.value = true
    try {
      const payload = await api.trafficHistory(range)
      trafficHistoryRange.value = range
      trafficHistory.value = payload
      return true
    } catch (failure) {
      if (showError && failure.status !== 401 && !isCanceledRequest(failure)) reportError(failure.message)
      return false
    } finally {
      historyRefreshing = false
      trafficHistoryLoading.value = false
    }
  }

  function setTrafficHistoryRange(range) {
    if (range === trafficHistoryRange.value && trafficHistory.value) return
    loadTrafficHistory(range, true)
  }

  function reset() {
    networkSamples.value = []
    trafficLive.value = null
    trafficHistory.value = null
    trafficHistoryRange.value = '1h'
    trafficHistoryLoading.value = false
    liveRefreshing = false
    historyRefreshing = false
  }

  function stop() {
    liveRefreshing = false
    historyRefreshing = false
    trafficHistoryLoading.value = false
  }

  return {
    networkSamples,
    trafficLive,
    trafficHistory,
    trafficHistoryRange,
    trafficHistoryLoading,
    sample,
    refreshTrafficLive,
    loadTrafficHistory,
    setTrafficHistoryRange,
    reset,
    stop
  }
}
