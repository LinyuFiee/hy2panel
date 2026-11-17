export function usePolling() {
  const timers = new Map()
  let stopped = true

  function clearTimers() {
    for (const timer of timers.values()) window.clearInterval(timer)
    timers.clear()
  }

  function stop() {
    stopped = true
    clearTimers()
  }

  function start(jobs) {
    clearTimers()
    stopped = false
    for (const job of jobs) {
      const timer = window.setInterval(() => {
        if (!stopped && (!job.when || job.when())) job.run()
      }, job.interval)
      timers.set(job.id, timer)
    }
  }

  return {
    start,
    stop,
    isStopped: () => stopped
  }
}
