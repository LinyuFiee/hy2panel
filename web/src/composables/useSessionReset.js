export function useSessionReset() {
  const resetters = new Set()
  let generation = 0

  function register(reset) {
    resetters.add(reset)
    return () => resetters.delete(reset)
  }

  function reset(reason = 'reset') {
    generation += 1
    for (const resetter of resetters) resetter(reason, generation)
    return generation
  }

  return {
    currentGeneration: () => generation,
    register,
    reset
  }
}
