<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  ChevronDown,
  Clipboard,
  Eye,
  EyeOff,
  Link2,
  QrCode,
  Save,
  ShieldAlert
} from '@lucide/vue'
import QRCodeGenerator from 'qrcode'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps({
  form: { type: Object, required: true },
  links: { type: Object, default: () => ({}) },
  meta: { type: Object, default: () => ({}) },
  saving: { type: Boolean, default: false },
  dirty: { type: Boolean, default: undefined },
  conflict: { type: Boolean, default: false },
  copied: { type: String, default: '' }
})

const emit = defineEmits(['save', 'copy'])

const realityAdvancedOpen = ref(false)
const hysteriaAdvancedOpen = ref(false)
const showRealityPrivateKey = ref(false)
const showHysteriaPassword = ref(false)
const showObfsPassword = ref(false)
const disableDialogOpen = ref(false)
const pendingDisable = ref('')
const credentialDialogOpen = ref(false)
const credentialProtocol = ref('')
const qrDataUrl = ref('')
const qrLoading = ref(false)
const qrError = ref('')
let qrRevision = 0

const realityName = computed(() => props.meta?.reality_name || 'VLESS Reality')
const hysteria2Name = computed(() => props.meta?.hysteria2_name || 'Hysteria2')
const dirtyKnown = computed(() => typeof props.dirty === 'boolean')
const saveDisabled = computed(() => props.saving || props.conflict || props.dirty === false)
const credentialLink = computed(() => credentialProtocol.value ? (props.links?.[credentialProtocol.value] || '') : '')
const credentialName = computed(() => credentialProtocol.value === 'reality' ? realityName.value : hysteria2Name.value)
const pendingDisableName = computed(() => pendingDisable.value === 'reality' ? 'Reality' : 'Hysteria2')

const saveState = computed(() => {
  if (props.conflict) return { tone: 'danger', title: '检测到配置冲突', detail: '远端配置已更新，当前草稿不会覆盖新配置。' }
  if (props.saving) return { tone: 'progress', title: '正在校验并应用', detail: '启动失败时服务端会自动恢复上一版配置。' }
  if (dirtyKnown.value && !props.dirty) return { tone: 'clean', title: '配置已同步', detail: '修改后即可保存并应用。' }
  if (props.dirty) return { tone: 'dirty', title: '有未保存的修改', detail: '保存后会校验配置并重启节点服务。' }
  return { tone: 'neutral', title: '保存当前配置', detail: '应用前由服务端校验。' }
})

function trimField(target, key) {
  if (typeof target?.[key] === 'string') target[key] = target[key].trim()
}

function updateNumber(target, key, value) {
  const text = String(value ?? '').trim()
  target[key] = text === '' ? '' : Number(text)
}

const unlimitedBandwidth = computed(() => formBandwidthUnlimited())

function formBandwidthUnlimited() {
  const hysteria = props.form?.hysteria2
  if (!hysteria) return false
  return hysteria.up_mbps === 0 && hysteria.down_mbps === 0
}

function setUnlimitedBandwidth(value) {
  const hysteria = props.form.hysteria2
  if (value === true) {
    hysteria.up_mbps = 0
    hysteria.down_mbps = 0
    return
  }
  if (hysteria.up_mbps === 0) hysteria.up_mbps = 100
  if (hysteria.down_mbps === 0) hysteria.down_mbps = 100
}

function randomObfsPassword() {
  const bytes = new Uint8Array(18)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
}

function setObfsType(value) {
  const hysteria = props.form.hysteria2
  hysteria.obfs_type = value
  if (!value) {
    hysteria.obfs_password = ''
    showObfsPassword.value = false
    return
  }
  if (!hysteria.obfs_password) {
    hysteria.obfs_password = randomObfsPassword()
  }
}

function requestProtocolToggle(protocol, nextValue) {
  const enabled = nextValue === true
  if (!enabled && props.form?.[protocol]?.enabled) {
    pendingDisable.value = protocol
    disableDialogOpen.value = true
    return
  }
  props.form[protocol].enabled = enabled
}

function confirmDisable() {
  if (pendingDisable.value && props.form?.[pendingDisable.value]) {
    props.form[pendingDisable.value].enabled = false
  }
  disableDialogOpen.value = false
}

function invalidateQr() {
  qrRevision += 1
  qrLoading.value = false
  qrDataUrl.value = ''
  qrError.value = ''
}

async function generateQr() {
  const protocol = credentialProtocol.value
  const link = props.links?.[protocol] || ''
  const revision = ++qrRevision
  qrLoading.value = true
  qrDataUrl.value = ''
  qrError.value = ''

  if (!link) {
    qrLoading.value = false
    return
  }

  try {
    const dataUrl = await QRCodeGenerator.toDataURL(link, {
      width: 320,
      margin: 2,
      errorCorrectionLevel: 'M',
      color: { dark: '#1d1d1f', light: '#ffffff' }
    })
    if (
      revision !== qrRevision ||
      !credentialDialogOpen.value ||
      credentialProtocol.value !== protocol ||
      props.links?.[protocol] !== link
    ) return
    qrDataUrl.value = dataUrl
  } catch {
    if (revision !== qrRevision || !credentialDialogOpen.value) return
    qrError.value = '二维码生成失败，请改用复制链接导入。'
  } finally {
    if (revision === qrRevision) qrLoading.value = false
  }
}

function openCredentials(protocol) {
  if (!props.links?.[protocol]) return
  invalidateQr()
  credentialProtocol.value = protocol
  credentialDialogOpen.value = true
  void generateQr()
}

function handleCredentialDialog(open) {
  credentialDialogOpen.value = open
  if (open) {
    void generateQr()
    return
  }
  invalidateQr()
  credentialProtocol.value = ''
}

function handleSubmit() {
  if (saveDisabled.value) return
  showRealityPrivateKey.value = false
  showHysteriaPassword.value = false
  showObfsPassword.value = false
  emit('save')
}

watch(() => [props.links?.reality, props.links?.hysteria2], () => {
  if (!credentialDialogOpen.value) return
  if (!credentialLink.value) {
    handleCredentialDialog(false)
    return
  }
  void generateQr()
})

watch(realityAdvancedOpen, open => {
  if (!open) showRealityPrivateKey.value = false
})

watch(hysteriaAdvancedOpen, open => {
  if (!open) {
    showHysteriaPassword.value = false
    showObfsPassword.value = false
  }
})

watch(() => props.saving, saving => {
  if (!saving) return
  showRealityPrivateKey.value = false
  showHysteriaPassword.value = false
  showObfsPassword.value = false
  if (credentialDialogOpen.value) handleCredentialDialog(false)
})

onBeforeUnmount(invalidateQr)
</script>

<template>
  <form class="nodes" @submit.prevent="handleSubmit">
    <Alert v-if="conflict" variant="destructive" class="nodes-alert">
      <AlertTriangle />
      <AlertTitle>节点配置已在其他会话中更新</AlertTitle>
      <AlertDescription>请刷新页面确认最新设置后再修改，当前草稿不会覆盖远端配置。</AlertDescription>
    </Alert>

    <section class="group">
      <h2>接入地址</h2>
      <div class="group-card pad">
        <Field class="field">
          <FieldLabel for="node-public-host">节点公网地址</FieldLabel>
          <Input
            id="node-public-host"
            v-model="form.public_host"
            autocomplete="off"
            spellcheck="false"
            placeholder="203.0.113.10 或 node.example.com"
            aria-label="节点公网地址"
            :disabled="saving"
            required
            @blur="trimField(form, 'public_host')"
          />
          <FieldDescription>只填主机部分，不带协议和端口。Reality 客户端从此地址连接，Hysteria2 使用证书域名。</FieldDescription>
        </Field>
      </div>
    </section>

    <div class="protocol-grid">
      <section class="group">
        <div class="group-card">
          <header class="proto-head">
            <div class="proto-title">
              <h3>{{ realityName }}</h3>
              <small class="tnum">TCP · 端口 {{ form.reality.port || '—' }}</small>
            </div>
            <div class="proto-state">
              <Badge :variant="form.reality.enabled ? 'success' : 'secondary'">
                {{ form.reality.enabled ? '已启用' : '已停用' }}
              </Badge>
              <Switch
                id="reality-enabled"
                :model-value="form.reality.enabled"
                :disabled="saving"
                aria-label="启用 Reality"
                @update:model-value="requestProtocolToggle('reality', $event)"
              />
            </div>
          </header>

          <div class="proto-body">
            <Field class="field">
              <FieldLabel for="reality-port">TCP 端口</FieldLabel>
              <Input id="reality-port" :model-value="form.reality.port" type="number" min="1" max="65535" inputmode="numeric" aria-label="TCP 端口" :disabled="saving" :required="form.reality.enabled" @update:model-value="updateNumber(form.reality, 'port', $event)" />
              <FieldDescription>1–65535，不能与面板端口冲突。</FieldDescription>
            </Field>
            <Field class="field">
              <FieldLabel for="reality-sni">伪装 SNI</FieldLabel>
              <Input id="reality-sni" v-model="form.reality.server_name" autocomplete="off" spellcheck="false" aria-label="伪装 SNI" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'server_name')" />
              <FieldDescription>Reality 握手使用的有效域名。</FieldDescription>
            </Field>

            <Collapsible v-model:open="realityAdvancedOpen" class="advanced">
              <CollapsibleTrigger as-child>
                <Button type="button" variant="ghost" class="advanced-trigger">
                  <span class="advanced-copy"><strong>高级密钥与身份</strong><small>UUID、Reality 密钥与 Short ID</small></span>
                  <ChevronDown class="advanced-chevron" :class="{ open: realityAdvancedOpen }" />
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div class="advanced-content">
                  <Field class="field">
                    <FieldLabel for="reality-uuid">UUID</FieldLabel>
                    <Input id="reality-uuid" v-model="form.reality.uuid" class="mono" autocomplete="off" spellcheck="false" aria-label="UUID" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'uuid')" />
                  </Field>
                  <Field class="field">
                    <FieldLabel for="reality-private-key">Reality 私钥</FieldLabel>
                    <div class="secret-wrap">
                      <Input id="reality-private-key" v-model="form.reality.private_key" class="mono" :type="showRealityPrivateKey ? 'text' : 'password'" autocomplete="off" spellcheck="false" aria-label="Reality 私钥" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'private_key')" />
                      <Button type="button" variant="ghost" size="sm" class="secret-toggle" :aria-label="showRealityPrivateKey ? '隐藏 Reality 私钥' : '显示 Reality 私钥'" :aria-pressed="showRealityPrivateKey" :disabled="saving" @click="showRealityPrivateKey = !showRealityPrivateKey">
                        <EyeOff v-if="showRealityPrivateKey" /><Eye v-else />{{ showRealityPrivateKey ? '隐藏' : '显示' }}
                      </Button>
                    </div>
                    <FieldDescription class="danger-help">私钥只保存在此节点，不要复制给客户端。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="reality-public-key">Reality 公钥</FieldLabel>
                    <Input id="reality-public-key" v-model="form.reality.public_key" class="mono" aria-label="Reality 公钥" readonly :required="form.reality.enabled" />
                    <FieldDescription>由私钥派生，客户端链接使用此公钥。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="reality-short-id">Short ID</FieldLabel>
                    <Input id="reality-short-id" v-model="form.reality.short_id" class="mono" autocomplete="off" spellcheck="false" aria-label="Short ID" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'short_id')" />
                    <FieldDescription>1–16 字节十六进制字符串。</FieldDescription>
                  </Field>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>

          <footer class="proto-foot">
            <span class="foot-copy">
              <strong>客户端凭据</strong>
              <small>{{ links?.reality ? '按需显示，不在页面常驻' : '保存并启用协议后生成' }}</small>
            </span>
            <Button type="button" variant="outline" size="sm" :disabled="saving || !links?.reality" :aria-label="`打开 ${realityName} 客户端凭据`" @click="openCredentials('reality')">
              <QrCode />{{ links?.reality ? '打开凭据' : '暂无凭据' }}
            </Button>
          </footer>
        </div>
      </section>

      <section class="group">
        <div class="group-card">
          <header class="proto-head">
            <div class="proto-title">
              <h3>{{ hysteria2Name }}</h3>
              <small class="tnum">UDP · 端口 {{ form.hysteria2.port || '—' }}</small>
            </div>
            <div class="proto-state">
              <Badge :variant="form.hysteria2.enabled ? 'success' : 'secondary'">
                {{ form.hysteria2.enabled ? '已启用' : '已停用' }}
              </Badge>
              <Switch
                id="hysteria2-enabled"
                :model-value="form.hysteria2.enabled"
                :disabled="saving"
                aria-label="启用 Hysteria2"
                @update:model-value="requestProtocolToggle('hysteria2', $event)"
              />
            </div>
          </header>

          <div class="proto-body">
            <Field class="field">
              <FieldLabel for="hysteria-port">UDP 端口</FieldLabel>
              <Input id="hysteria-port" :model-value="form.hysteria2.port" type="number" min="1" max="65535" inputmode="numeric" aria-label="UDP 端口" :disabled="saving" :required="form.hysteria2.enabled" @update:model-value="updateNumber(form.hysteria2, 'port', $event)" />
              <FieldDescription>1–65535，可与 TCP 端口使用相同数字。</FieldDescription>
            </Field>
            <Field class="field">
              <FieldLabel for="hysteria-domain">证书域名</FieldLabel>
              <Input id="hysteria-domain" v-model="form.hysteria2.domain" aria-label="证书域名" readonly />
              <FieldDescription>由安装时签发的 TLS 证书决定。</FieldDescription>
            </Field>

            <Collapsible v-model:open="hysteriaAdvancedOpen" class="advanced">
              <CollapsibleTrigger as-child>
                <Button type="button" variant="ghost" class="advanced-trigger">
                  <span class="advanced-copy"><strong>高级认证与带宽</strong><small>连接密码与服务端带宽</small></span>
                  <ChevronDown class="advanced-chevron" :class="{ open: hysteriaAdvancedOpen }" />
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div class="advanced-content">
                  <Field class="field">
                    <FieldLabel for="hysteria-password">连接密码</FieldLabel>
                    <div class="secret-wrap">
                      <Input id="hysteria-password" v-model="form.hysteria2.password" class="mono" :type="showHysteriaPassword ? 'text' : 'password'" autocomplete="off" spellcheck="false" aria-label="连接密码" :disabled="saving" :required="form.hysteria2.enabled" @blur="trimField(form.hysteria2, 'password')" />
                      <Button type="button" variant="ghost" size="sm" class="secret-toggle" :aria-label="showHysteriaPassword ? '隐藏 Hysteria2 连接密码' : '显示 Hysteria2 连接密码'" :aria-pressed="showHysteriaPassword" :disabled="saving" @click="showHysteriaPassword = !showHysteriaPassword">
                        <EyeOff v-if="showHysteriaPassword" /><Eye v-else />{{ showHysteriaPassword ? '隐藏' : '显示' }}
                      </Button>
                    </div>
                    <FieldDescription class="danger-help">密码会进入客户端链接，只分享给受信任设备。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="hysteria-unlimited">不限速</FieldLabel>
                    <Switch id="hysteria-unlimited" :model-value="unlimitedBandwidth" :disabled="saving || !form.hysteria2.enabled" aria-label="Hysteria2 不限速" @update:model-value="setUnlimitedBandwidth" />
                    <FieldDescription>开启后不声明带宽，由客户端拥塞控制自行探测；关闭时按填写的 Mbps 作为上限提示。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="hysteria-up">服务器上传 Mbps</FieldLabel>
                    <Input id="hysteria-up" :model-value="form.hysteria2.up_mbps" type="number" min="0" max="10000" inputmode="numeric" aria-label="服务器上传 Mbps" :disabled="saving || unlimitedBandwidth" :required="form.hysteria2.enabled && !unlimitedBandwidth" @update:model-value="updateNumber(form.hysteria2, 'up_mbps', $event)" />
                    <FieldDescription>0 表示不限速。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="hysteria-down">服务器下载 Mbps</FieldLabel>
                    <Input id="hysteria-down" :model-value="form.hysteria2.down_mbps" type="number" min="0" max="10000" inputmode="numeric" aria-label="服务器下载 Mbps" :disabled="saving || unlimitedBandwidth" :required="form.hysteria2.enabled && !unlimitedBandwidth" @update:model-value="updateNumber(form.hysteria2, 'down_mbps', $event)" />
                    <FieldDescription>填略高于实测链路的值，过大会加剧拥塞。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="hysteria-bbr">BBR 拥塞档位</FieldLabel>
                    <NativeSelect id="hysteria-bbr" :model-value="form.hysteria2.bbr_profile" :disabled="saving" aria-label="Hysteria2 BBR 拥塞档位" @update:model-value="form.hysteria2.bbr_profile = $event">
                      <NativeSelectOption value="conservative">conservative（保守）</NativeSelectOption>
                      <NativeSelectOption value="standard">standard（默认）</NativeSelectOption>
                      <NativeSelectOption value="aggressive">aggressive（激进）</NativeSelectOption>
                    </NativeSelect>
                    <FieldDescription>高丢包链路可试 aggressive，代价是更容易加剧拥塞。</FieldDescription>
                  </Field>
                  <Field class="field">
                    <FieldLabel for="hysteria-obfs">QUIC 混淆</FieldLabel>
                    <NativeSelect id="hysteria-obfs" :model-value="form.hysteria2.obfs_type" :disabled="saving" aria-label="Hysteria2 QUIC 混淆" @update:model-value="setObfsType">
                      <NativeSelectOption value="">关闭</NativeSelectOption>
                      <NativeSelectOption value="salamander">salamander</NativeSelectOption>
                      <NativeSelectOption value="gecko">gecko</NativeSelectOption>
                    </NativeSelect>
                    <FieldDescription>把 QUIC 伪装成随机 UDP，可缓解运营商对 UDP 的限速；客户端必须填同样的混淆密码。</FieldDescription>
                  </Field>
                  <Field v-if="form.hysteria2.obfs_type" class="field">
                    <FieldLabel for="hysteria-obfs-password">混淆密码</FieldLabel>
                    <div class="secret-wrap">
                      <Input id="hysteria-obfs-password" v-model="form.hysteria2.obfs_password" class="mono" :type="showObfsPassword ? 'text' : 'password'" autocomplete="off" spellcheck="false" aria-label="混淆密码" :disabled="saving" :required="form.hysteria2.enabled" @blur="trimField(form.hysteria2, 'obfs_password')" />
                      <Button type="button" variant="ghost" size="sm" class="secret-toggle" :aria-label="showObfsPassword ? '隐藏混淆密码' : '显示混淆密码'" :aria-pressed="showObfsPassword" :disabled="saving" @click="showObfsPassword = !showObfsPassword">
                        <EyeOff v-if="showObfsPassword" /><Eye v-else />{{ showObfsPassword ? '隐藏' : '显示' }}
                      </Button>
                    </div>
                    <FieldDescription class="danger-help">开启后旧客户端立即断连，需重新导入链接；密码会进入客户端链接。</FieldDescription>
                  </Field>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>

          <footer class="proto-foot">
            <span class="foot-copy">
              <strong>客户端凭据</strong>
              <small>{{ links?.hysteria2 ? '按需显示，不在页面常驻' : '保存并启用协议后生成' }}</small>
            </span>
            <Button type="button" variant="outline" size="sm" :disabled="saving || !links?.hysteria2" :aria-label="`打开 ${hysteria2Name} 客户端凭据`" @click="openCredentials('hysteria2')">
              <QrCode />{{ links?.hysteria2 ? '打开凭据' : '暂无凭据' }}
            </Button>
          </footer>
        </div>
      </section>
    </div>

    <div class="save-bar" :data-tone="saveState.tone" role="region" aria-label="保存节点配置">
      <span class="save-icon" aria-hidden="true"><Spinner v-if="saving" label="正在保存节点配置" /><AlertTriangle v-else-if="conflict" /><CheckCircle2 v-else-if="dirtyKnown && !dirty" /><Save v-else /></span>
      <span class="save-copy" aria-live="polite"><strong>{{ saveState.title }}</strong><small>{{ saveState.detail }}</small></span>
      <Button type="submit" class="save-button" :disabled="saveDisabled"><Spinner v-if="saving" label="正在保存并应用" /><Save v-else />{{ saving ? '正在应用…' : conflict ? '存在冲突' : '保存并应用' }}</Button>
    </div>
  </form>

  <AlertDialog :open="disableDialogOpen" @update:open="disableDialogOpen = $event">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>停用 {{ pendingDisableName }}？</AlertDialogTitle>
        <AlertDialogDescription>保存后会停止该协议监听，现有连接中断、客户端链接失效。可在保存前重新启用。</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>保持启用</AlertDialogCancel>
        <AlertDialogAction class="bg-destructive text-destructive-foreground hover:bg-destructive/90" @click="confirmDisable">确认停用</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <Dialog :open="credentialDialogOpen" @update:open="handleCredentialDialog">
    <DialogContent v-if="credentialDialogOpen && credentialProtocol" class="credential-dialog">
      <DialogHeader>
        <DialogTitle>{{ credentialName }} 客户端凭据</DialogTitle>
        <DialogDescription>在受信任的客户端中扫码，或复制链接导入。</DialogDescription>
      </DialogHeader>
      <div class="credential-grid">
        <section class="qr-panel" aria-live="polite">
          <div v-if="qrLoading" class="qr-state"><Spinner label="正在生成客户端二维码" /><span>正在本地生成…</span></div>
          <div v-else-if="qrError" class="qr-state qr-error"><QrCode /><span>{{ qrError }}</span></div>
          <img v-else-if="qrDataUrl" :src="qrDataUrl" :alt="`${credentialName} 客户端二维码`" />
          <div v-else class="qr-state"><QrCode /><span>暂无二维码</span></div>
        </section>
        <section class="link-panel">
          <div class="link-heading"><Link2 aria-hidden="true" /><strong>导入链接</strong></div>
          <Textarea :model-value="credentialLink" readonly rows="6" class="credential-link" :aria-label="`${credentialName} 客户端链接`" @focus="$event.target.select()" />
          <Button type="button" class="copy-button" @click="emit('copy', credentialProtocol)"><Check v-if="copied === credentialProtocol" /><Clipboard v-else />{{ copied === credentialProtocol ? '已复制' : '复制链接' }}</Button>
        </section>
      </div>
      <Alert variant="warning" class="credential-warning"><ShieldAlert /><AlertTitle>链接视同密码</AlertTitle><AlertDescription>请勿截图或公开分享；用完即关。</AlertDescription></Alert>
      <DialogFooter><DialogClose as-child><Button type="button" variant="outline">关闭</Button></DialogClose></DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.nodes {
  display: grid;
  min-width: 0;
  gap: 1rem;
}

.nodes-alert {
  margin: 0;
}

.group,
.protocol-grid > .group {
  min-width: 0;
}

.group h2 {
  margin: 0 0 0.4rem 0.2rem;
  color: var(--muted-foreground);
  font-size: 0.7rem;
  font-weight: 650;
  letter-spacing: 0.015em;
}

.group-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: var(--card);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--foreground) 4%, transparent);
}

.group-card.pad {
  padding: 0.85rem 1rem;
}

.field {
  min-width: 0;
  gap: 0.38rem;
}

.field :deep([data-slot='field-label']),
.field > [data-slot='field-label'] {
  color: var(--foreground);
  font-size: 0.75rem;
  font-weight: 600;
}

.field :deep([data-slot='field-description']),
.field > [data-slot='field-description'] {
  color: var(--muted-foreground);
  font-size: 0.67rem;
  line-height: 1.45;
}

.field :deep(input) {
  min-height: 2.35rem;
  border-radius: 0.5rem;
  font-size: 0.8rem;
}

.field :deep(input[readonly]) {
  color: var(--muted-foreground);
  background: color-mix(in srgb, var(--muted) 72%, var(--card));
}

.field .mono :deep(input),
.field :deep(input.mono) {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.72rem;
}

.danger-help {
  color: color-mix(in srgb, var(--destructive) 70%, var(--muted-foreground));
}

.protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
  gap: 0.85rem;
  min-width: 0;
}

.protocol-grid .group h2 {
  display: none;
}

.proto-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  min-width: 0;
  padding: 0.8rem 1rem;
  border-bottom: 1px solid var(--border);
}

.proto-title {
  min-width: 0;
}

.proto-title h3 {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 650;
  letter-spacing: -0.015em;
  overflow-wrap: anywhere;
}

.proto-title small {
  display: block;
  margin-top: 0.15rem;
  color: var(--muted-foreground);
  font-size: 0.66rem;
}

.proto-state {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.5rem;
}

.proto-body {
  display: grid;
  gap: 0.8rem;
  padding: 0.85rem 1rem 0.9rem;
}

.advanced {
  min-width: 0;
  margin-top: 0.05rem;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.6rem;
  background: color-mix(in srgb, var(--muted) 36%, var(--card));
}

.advanced-trigger {
  width: 100%;
  height: auto;
  min-height: 2.85rem;
  justify-content: space-between;
  padding: 0.55rem 0.7rem;
  border-radius: 0;
  font-weight: 600;
  white-space: normal;
}

.advanced-copy {
  display: grid;
  min-width: 0;
  flex: 1;
  justify-items: start;
  gap: 0.08rem;
  text-align: left;
}

.advanced-copy strong {
  color: var(--foreground);
  font-size: 0.76rem;
}

.advanced-copy small {
  color: var(--muted-foreground);
  font-size: 0.63rem;
  font-weight: 400;
}

.advanced-chevron {
  width: 0.95rem;
  height: 0.95rem;
  flex: 0 0 auto;
  color: var(--muted-foreground);
  transition: transform 0.18s ease;
}

.advanced-chevron.open {
  transform: rotate(180deg);
}

.advanced-content {
  display: grid;
  gap: 0.85rem;
  padding: 0.8rem 0.7rem;
  border-top: 1px solid var(--border);
  background: var(--card);
}

.advanced-content[hidden] {
  display: none;
}

.secret-wrap {
  position: relative;
  min-width: 0;
}

.secret-wrap :deep(input) {
  padding-right: 5.2rem;
}

.secret-toggle {
  position: absolute;
  top: 50%;
  right: 0.25rem;
  height: 1.85rem;
  padding-inline: 0.5rem;
  color: var(--muted-foreground);
  font-size: 0.67rem;
  transform: translateY(-50%);
}

.proto-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  min-width: 0;
  padding: 0.7rem 1rem;
  border-top: 1px solid var(--border);
  background: color-mix(in srgb, var(--muted) 48%, var(--card));
}

.foot-copy {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.foot-copy strong {
  color: var(--foreground);
  font-size: 0.75rem;
  font-weight: 600;
}

.foot-copy small {
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.save-bar {
  position: sticky;
  z-index: 20;
  bottom: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: color-mix(in srgb, var(--card) 94%, transparent);
  box-shadow: 0 8px 24px color-mix(in srgb, var(--foreground) 10%, transparent);
  backdrop-filter: blur(16px) saturate(150%);
  -webkit-backdrop-filter: blur(16px) saturate(150%);
}

.save-icon {
  width: 2rem;
  height: 2rem;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 0.5rem;
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 9%, transparent);
}

.save-icon svg {
  width: 0.95rem;
  height: 0.95rem;
}

.save-bar[data-tone='danger'] .save-icon {
  color: var(--destructive);
  background: color-mix(in srgb, var(--destructive) 9%, transparent);
}

.save-copy {
  display: grid;
  min-width: 0;
  flex: 1;
  gap: 0.05rem;
}

.save-copy strong {
  color: var(--foreground);
  font-size: 0.75rem;
}

.save-copy small {
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.save-button {
  min-width: 8.5rem;
}

.credential-dialog {
  width: calc(100% - 1.5rem);
  max-width: 42rem;
  max-height: calc(100dvh - 1.5rem);
  overflow-y: auto;
}

.credential-grid {
  display: grid;
  grid-template-columns: minmax(11rem, 0.75fr) minmax(0, 1.25fr);
  gap: 0.75rem;
}

.qr-panel,
.link-panel {
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 0.65rem;
  background: color-mix(in srgb, var(--muted) 52%, var(--card));
}

.qr-panel {
  display: grid;
  min-height: 14rem;
  padding: 0.8rem;
  place-items: center;
}

.qr-panel img {
  display: block;
  width: min(12.5rem, 100%);
  aspect-ratio: 1;
  padding: 0.35rem;
  border-radius: 0.45rem;
  background: #fff;
}

.qr-state {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
  color: var(--muted-foreground);
  font-size: 0.7rem;
  text-align: center;
}

.qr-state svg {
  width: 1.25rem;
  height: 1.25rem;
  color: var(--primary);
}

.qr-error {
  color: var(--destructive);
}

.link-panel {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.8rem;
}

.link-heading {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--foreground);
  font-size: 0.75rem;
  font-weight: 600;
}

.link-heading svg {
  color: var(--primary);
}

.credential-link {
  min-height: 7.5rem;
  resize: none;
  overflow-wrap: anywhere;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.68rem;
  line-height: 1.5;
  background: var(--card);
}

.copy-button {
  width: 100%;
  margin-top: auto;
}

@media (max-width: 900px) {
  .protocol-grid {
    grid-template-columns: 1fr;
  }

  .save-bar {
    bottom: calc(4.75rem + env(safe-area-inset-bottom));
  }
}

@media (max-width: 580px) {
  .nodes {
    gap: 0.8rem;
  }

  .group-card.pad,
  .proto-head,
  .proto-body,
  .proto-foot {
    padding-inline: 0.8rem;
  }

  .credential-grid {
    grid-template-columns: 1fr;
  }

  .qr-panel {
    min-height: 11.5rem;
  }

  .save-bar {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
  }

  .save-button {
    grid-column: 1 / -1;
    width: 100%;
    min-width: 0;
  }
}

@media (max-width: 390px) {
  .proto-head {
    align-items: flex-start;
  }

  .proto-state {
    flex-direction: column-reverse;
    align-items: flex-end;
  }

  .proto-foot {
    align-items: stretch;
    flex-direction: column;
  }

  .proto-foot :deep(button) {
    width: 100%;
  }

  .advanced-copy small {
    display: none;
  }

  .secret-wrap :deep(input) {
    padding-right: 2.65rem;
  }

  .secret-toggle {
    width: 1.9rem;
    padding: 0;
    font-size: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .advanced-chevron {
    transition: none;
  }
}
</style>
