<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Copy } from '@lucide/vue'
import { Button } from './ui/button'
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogTitle } from './ui/alert-dialog'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from './ui/dialog'

const props = defineProps({
  subscriptionStatus: { type: Function, required: true },
  manageSubscription: { type: Function, required: true },
  session: { type: Object, required: true },
  hasProtocol: Boolean,
  copied: Boolean
})
const emit = defineEmits(['copied', 'revoked'])
const enabled = ref(null)
const pending = ref('')
const error = ref('')
const confirm = ref('')
// This is the only retained copy of the URL, and exists only for the open dialog.
const manualAddress = ref('')
const field = ref(null)
const scope = new AbortController()
const unregister = props.session.register(dispose)
const buttonLabel = computed(() => {
  if (pending.value === 'status') return '正在读取订阅…'
  if (pending.value) return '正在处理…'
  if (enabled.value === null) return '重试订阅状态'
  if (enabled.value && props.copied) return '已复制'
  return enabled.value ? '复制订阅' : '创建并复制订阅'
})

function dispose() {
  scope.abort()
  manualAddress.value = ''
  confirm.value = ''
  enabled.value = null
}
async function request(action) {
  if (pending.value || scope.signal.aborted) return
  confirm.value = ''
  error.value = ''
  pending.value = action
  try {
    const result = action === 'status'
      ? await props.subscriptionStatus(scope.signal)
      : await props.manageSubscription(action, scope.signal)
    if (scope.signal.aborted) return
    if (typeof result.enabled !== 'boolean') throw new Error()
    enabled.value = result.enabled
    if (action === 'status') return
    if (action === 'revoke') {
      emit('revoked')
      return
    }
    if (!result.enabled || typeof result.url !== 'string' || !result.url) throw new Error()
    // Only an acknowledged browser write counts as copied. A refused write
    // must not put this credential into an off-screen legacy copy buffer.
    try {
      await navigator.clipboard.writeText(result.url)
      if (!scope.signal.aborted) emit('copied')
    } catch {
      if (!scope.signal.aborted) manualAddress.value = result.url
    }
  } catch {
    if (!scope.signal.aborted) {
      // A mutation may have reached the server. Re-read status before retrying.
      enabled.value = null
      error.value = action === 'status' ? '未能读取订阅状态，请重试。' : '未能确认订阅操作结果，请重新读取状态。'
    }
  } finally {
    pending.value = ''
  }
}
function selectAddress(event) {
  event.preventDefault()
  field.value?.focus()
  field.value?.select()
}
onMounted(() => request('status'))
onBeforeUnmount(() => { unregister(); dispose() })
</script>

<template>
  <p class="hint">{{ hasProtocol ? '在 sing-box 客户端添加远程配置，粘贴订阅地址。请勿分享此地址。' : '两个协议均已停用。请先在节点页启用一个协议。' }}</p>
  <div class="actions primary-actions">
    <Button type="button" :disabled="!!pending || (!hasProtocol && enabled !== null)" @click="request(enabled === null ? 'status' : enabled ? 'copy' : 'create')">
      <Copy aria-hidden="true" />{{ buttonLabel }}
    </Button>
    <slot />
  </div>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <details v-if="enabled" class="manage">
    <summary>订阅管理</summary>
    <p>更换或停用地址不会删除设备已下载的配置。</p>
    <div class="actions">
      <Button type="button" size="sm" variant="outline" :disabled="!!pending" @click="confirm = 'rotate'">重置订阅</Button>
      <Button type="button" size="sm" variant="outline" :disabled="!!pending" @click="confirm = 'revoke'">撤销订阅</Button>
    </div>
  </details>

  <AlertDialog :open="!!confirm" @update:open="open => { if (!open) confirm = '' }">
    <AlertDialogContent>
      <AlertDialogTitle>{{ confirm === 'rotate' ? '重置订阅地址？' : '撤销订阅地址？' }}</AlertDialogTitle>
      <AlertDialogDescription>{{ confirm === 'rotate' ? '旧地址会立即失效。请在各设备上替换为新地址。' : '旧地址会立即失效，设备将无法再用它更新配置。' }}</AlertDialogDescription>
      <AlertDialogFooter>
        <AlertDialogCancel>取消</AlertDialogCancel>
        <Button type="button" variant="destructive" @click="request(confirm)">{{ confirm === 'rotate' ? '确认重置' : '确认撤销' }}</Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
  <Dialog v-if="manualAddress" :open="true" @update:open="open => { if (!open) manualAddress = '' }">
    <DialogContent :show-close-button="false" @open-auto-focus="selectAddress">
      <DialogTitle>手动复制订阅</DialogTitle>
      <DialogDescription>浏览器未能写入剪贴板。请复制下方地址，仅在自己的设备上使用。</DialogDescription>
      <textarea ref="field" aria-label="订阅地址" readonly spellcheck="false" autocomplete="off" rows="3" :value="manualAddress" @focus="$event.target.select()" />
      <Button type="button" class="justify-self-end" @click="manualAddress = ''">关闭</Button>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.actions { display: flex; flex-wrap: wrap; align-items: center; gap: .65rem; }
.primary-actions { margin-top: 1.25rem; }
.primary-actions :deep(button) { min-height: 2.75rem; padding-inline: 1.05rem; }
.hint, .manage, .error { margin: .55rem 0 0; font-size: .8125rem; }
.hint { max-width: 30em; line-height: 1.65; }
@media (max-width: 450px) {
  .primary-actions { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); gap: .5rem; }
  .primary-actions :deep(button) { min-width: 0; padding-inline: .375rem; }
}
.hint, .manage { color: var(--muted-foreground); }
.error { color: var(--destructive); }
.manage summary { width: fit-content; cursor: pointer; padding: .35rem .125rem .35rem 0; }
.manage summary:hover { color: var(--foreground); }
.manage p { margin: .25rem 0 .6rem; }
textarea { resize: vertical; width: 100%; padding: .55rem .7rem; border: 1px solid var(--input); border-radius: .4rem; background: var(--background); color: var(--foreground); font: inherit; overflow-wrap: anywhere; }
</style>
