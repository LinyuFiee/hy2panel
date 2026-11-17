import { computed } from 'vue'

export function useMaintenanceLock({ busy, state, singBoxVersions }) {
  const locked = computed(() => Boolean(
    busy.updating ||
    busy.singBox ||
    state.value?.update?.updating ||
    singBoxVersions.value?.operation?.status === 'running'
  ))

  function guard(message) {
    return locked.value ? message : ''
  }

  return { locked, guard }
}
