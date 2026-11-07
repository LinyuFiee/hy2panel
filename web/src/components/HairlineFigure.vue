<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { router } from '@lucasmarkes/hairline'

const host = ref(null)
let figure

onMounted(() => {
  figure = router(host.value, {
    play: true,
    intensity: 0.45,
    theme: 'light',
    label: '随指针互动的服务器路由器线稿'
  })
})

onBeforeUnmount(() => figure?.destroy())
</script>

<template>
  <div ref="host" class="hairline-figure"></div>
</template>