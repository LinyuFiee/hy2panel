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
  Globe2,
  KeyRound,
  Link2,
  QrCode,
  Radio,
  Save,
  ShieldAlert,
  ShieldCheck,
  SlidersHorizontal
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
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle
} from '@/components/ui/card'
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
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
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
const enabledCount = computed(() => [props.form?.reality?.enabled, props.form?.hysteria2?.enabled].filter(Boolean).length)
const dirtyKnown = computed(() => typeof props.dirty === 'boolean')
const saveDisabled = computed(() => props.saving || props.conflict || props.dirty === false)
const credentialLink = computed(() => credentialProtocol.value ? (props.links?.[credentialProtocol.value] || '') : '')
const credentialName = computed(() => credentialProtocol.value === 'reality' ? realityName.value : hysteria2Name.value)
const pendingDisableName = computed(() => pendingDisable.value === 'reality' ? 'Reality' : 'Hysteria2')

const saveState = computed(() => {
  if (props.conflict) return { tone: 'danger', title: '检测到配置冲突', detail: '远端配置已更新，请刷新后再编辑，当前草稿不会覆盖新配置。' }
  if (props.saving) return { tone: 'progress', title: '正在校验并应用', detail: '启动失败时，服务端会自动恢复上一版配置。' }
  if (dirtyKnown.value && !props.dirty) return { tone: 'clean', title: '配置已同步', detail: '修改任一设置后即可保存并应用。' }
  if (props.dirty) return { tone: 'dirty', title: '有未保存的修改', detail: '保存后会校验配置并重新加载节点服务。' }
  return { tone: 'neutral', title: '保存当前工作区', detail: '应用前由服务端校验，启动失败时自动恢复上一版。' }
})

function trimField(target, key) {
  if (typeof target?.[key] === 'string') target[key] = target[key].trim()
}

function updateNumber(target, key, value) {
  const text = String(value ?? '').trim()
  target[key] = text === '' ? '' : Number(text)
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
      color: { dark: '#173f35', light: '#ffffff' }
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
  if (!open) showHysteriaPassword.value = false
})

watch(() => props.saving, saving => {
  if (!saving) return
  showRealityPrivateKey.value = false
  showHysteriaPassword.value = false
  if (credentialDialogOpen.value) handleCredentialDialog(false)
})

onBeforeUnmount(invalidateQr)
</script>

<template>
  <form class="nodes-workspace" @submit.prevent="handleSubmit">
    <Alert v-if="conflict" variant="destructive" class="workspace-alert">
      <AlertTriangle />
      <AlertTitle>节点配置已在其他会话中更新</AlertTitle>
      <AlertDescription>当前草稿不会自动覆盖远端配置。请刷新页面或重新登录，确认最新设置后再修改。</AlertDescription>
    </Alert>

    <Card class="endpoint-card">
      <CardContent class="endpoint-layout">
        <div class="endpoint-copy">
          <span class="endpoint-icon" aria-hidden="true"><Globe2 /></span>
          <div>
            <div class="kicker-row">
              <span class="workspace-kicker">PUBLIC ENDPOINT</span>
              <Badge variant="outline">{{ enabledCount }}/2 个协议启用</Badge>
            </div>
            <CardTitle class="endpoint-title">节点公网 Endpoint</CardTitle>
            <CardDescription class="endpoint-description">
              Reality 客户端从此地址连接；可填写公网 IPv4、IPv6 或域名。Hysteria2 继续使用证书域名。
            </CardDescription>
          </div>
        </div>
        <Field class="node-field endpoint-field">
          <FieldLabel for="node-public-host" class="field-label">节点公网地址</FieldLabel>
          <div class="input-prefix-wrap">
            <Globe2 aria-hidden="true" />
            <Input
              id="node-public-host"
              v-model="form.public_host"
              class="workspace-input prefixed-input"
              autocomplete="off"
              spellcheck="false"
              placeholder="203.0.113.10 或 node.example.com"
              aria-label="节点公网地址"
              :disabled="saving"
              required
              @blur="trimField(form, 'public_host')"
            />
          </div>
          <FieldDescription class="field-description">只填写主机部分，不要添加协议或端口。</FieldDescription>
        </Field>
      </CardContent>
    </Card>

    <section class="protocol-grid" aria-label="节点协议工作区">
      <Card class="protocol-card reality-card">
        <CardHeader class="protocol-header">
          <div class="protocol-heading">
            <span class="protocol-icon" aria-hidden="true"><ShieldCheck /></span>
            <div>
              <span class="workspace-kicker">TCP · VLESS · VISION</span>
              <CardTitle>{{ realityName }}</CardTitle>
              <CardDescription>稳定主力连接，适合日常与长连接场景。</CardDescription>
            </div>
          </div>
          <div class="protocol-state">
            <Badge :variant="form.reality.enabled ? 'success' : 'secondary'">
              <span class="status-dot" aria-hidden="true"></span>{{ form.reality.enabled ? '已启用' : '已停用' }}
            </Badge>
            <div class="switch-line">
              <Switch
                id="reality-enabled"
                :model-value="form.reality.enabled"
                :disabled="saving"
                aria-label="启用 Reality"
                @update:model-value="requestProtocolToggle('reality', $event)"
              />
              <Label for="reality-enabled" class="switch-label">启用 Reality</Label>
            </div>
          </div>
        </CardHeader>
        <Separator />
        <CardContent class="protocol-body">
          <section class="settings-section" aria-labelledby="reality-basic-title">
            <div class="section-heading">
              <div><span class="section-icon" aria-hidden="true"><SlidersHorizontal /></span><span><h4 id="reality-basic-title">基础设置</h4><p>端口与伪装站点始终可见。</p></span></div>
              <Badge variant="secondary">TCP</Badge>
            </div>
            <div class="field-stack">
              <Field class="node-field">
                <FieldLabel for="reality-port" class="field-label">TCP 端口</FieldLabel>
                <Input id="reality-port" :model-value="form.reality.port" class="workspace-input" type="number" min="1" max="65535" inputmode="numeric" aria-label="TCP 端口" :disabled="saving" :required="form.reality.enabled" @update:model-value="updateNumber(form.reality, 'port', $event)" />
                <FieldDescription class="field-description">1–65535，不能与面板端口冲突。</FieldDescription>
              </Field>
              <Field class="node-field">
                <FieldLabel for="reality-sni" class="field-label">伪装 SNI</FieldLabel>
                <Input id="reality-sni" v-model="form.reality.server_name" class="workspace-input" autocomplete="off" spellcheck="false" aria-label="伪装 SNI" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'server_name')" />
                <FieldDescription class="field-description">Reality 握手使用的有效域名。</FieldDescription>
              </Field>
            </div>
          </section>

          <Collapsible v-model:open="realityAdvancedOpen" class="advanced-settings">
            <CollapsibleTrigger as-child>
              <Button type="button" variant="ghost" class="advanced-trigger">
                <span class="advanced-icon" aria-hidden="true"><KeyRound /></span>
                <span class="advanced-copy"><strong>高级密钥与身份</strong><small>UUID、Reality 密钥与 Short ID</small></span>
                <Badge variant="outline">敏感设置</Badge>
                <ChevronDown class="advanced-chevron" :class="{ open: realityAdvancedOpen }" />
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div class="advanced-content">
                <Field class="node-field">
                  <FieldLabel for="reality-uuid" class="field-label">UUID</FieldLabel>
                  <Input id="reality-uuid" v-model="form.reality.uuid" class="workspace-input mono-input" autocomplete="off" spellcheck="false" aria-label="UUID" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'uuid')" />
                </Field>
                <Field class="node-field">
                  <FieldLabel for="reality-private-key" class="field-label">Reality 私钥</FieldLabel>
                  <div class="secret-wrap">
                    <Input id="reality-private-key" v-model="form.reality.private_key" class="workspace-input mono-input secret-input" :type="showRealityPrivateKey ? 'text' : 'password'" autocomplete="off" spellcheck="false" aria-label="Reality 私钥" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'private_key')" />
                    <Button type="button" variant="ghost" size="sm" class="secret-toggle" :aria-label="showRealityPrivateKey ? '隐藏 Reality 私钥' : '显示 Reality 私钥'" :aria-pressed="showRealityPrivateKey" :disabled="saving" @click="showRealityPrivateKey = !showRealityPrivateKey">
                      <EyeOff v-if="showRealityPrivateKey" /><Eye v-else />{{ showRealityPrivateKey ? '隐藏私钥' : '显示私钥' }}
                    </Button>
                  </div>
                  <FieldDescription class="field-description danger-help">私钥只应保存在此节点，不要复制给客户端。</FieldDescription>
                </Field>
                <Field class="node-field">
                  <FieldLabel for="reality-public-key" class="field-label">Reality 公钥</FieldLabel>
                  <Input id="reality-public-key" v-model="form.reality.public_key" class="workspace-input mono-input" aria-label="Reality 公钥" readonly :required="form.reality.enabled" />
                  <FieldDescription class="field-description">由私钥派生，客户端链接使用此公钥。</FieldDescription>
                </Field>
                <Field class="node-field">
                  <FieldLabel for="reality-short-id" class="field-label">Short ID</FieldLabel>
                  <Input id="reality-short-id" v-model="form.reality.short_id" class="workspace-input mono-input" autocomplete="off" spellcheck="false" aria-label="Short ID" :disabled="saving" :required="form.reality.enabled" @blur="trimField(form.reality, 'short_id')" />
                  <FieldDescription class="field-description">1–16 字节十六进制字符串。</FieldDescription>
                </Field>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </CardContent>
        <CardFooter class="protocol-footer">
          <div class="credential-launcher" :class="{ unavailable: !links?.reality }">
            <div><span class="credential-icon" aria-hidden="true"><KeyRound /></span><span><strong>客户端凭据</strong><small>{{ links?.reality ? '按需打开，不在工作区常驻显示' : '保存并启用协议后生成' }}</small></span></div>
            <Button type="button" variant="outline" size="sm" :disabled="saving || !links?.reality" :aria-label="`打开 ${realityName} 客户端凭据`" @click="openCredentials('reality')"><QrCode />{{ links?.reality ? '打开凭据' : '暂无凭据' }}</Button>
          </div>
        </CardFooter>
      </Card>

      <Card class="protocol-card hysteria-card">
        <CardHeader class="protocol-header">
          <div class="protocol-heading">
            <span class="protocol-icon" aria-hidden="true"><Radio /></span>
            <div>
              <span class="workspace-kicker">UDP · QUIC · TLS</span>
              <CardTitle>{{ hysteria2Name }}</CardTitle>
              <CardDescription>弱网与高延迟链路的弹性备选协议。</CardDescription>
            </div>
          </div>
          <div class="protocol-state">
            <Badge :variant="form.hysteria2.enabled ? 'success' : 'secondary'">
              <span class="status-dot" aria-hidden="true"></span>{{ form.hysteria2.enabled ? '已启用' : '已停用' }}
            </Badge>
            <div class="switch-line">
              <Switch id="hysteria2-enabled" :model-value="form.hysteria2.enabled" :disabled="saving" aria-label="启用 Hysteria2" @update:model-value="requestProtocolToggle('hysteria2', $event)" />
              <Label for="hysteria2-enabled" class="switch-label">启用 Hysteria2</Label>
            </div>
          </div>
        </CardHeader>
        <Separator />
        <CardContent class="protocol-body">
          <section class="settings-section" aria-labelledby="hysteria-basic-title">
            <div class="section-heading">
              <div><span class="section-icon" aria-hidden="true"><SlidersHorizontal /></span><span><h4 id="hysteria-basic-title">基础设置</h4><p>监听端口与证书域名始终可见。</p></span></div>
              <Badge variant="secondary">UDP</Badge>
            </div>
            <div class="field-stack">
              <Field class="node-field">
                <FieldLabel for="hysteria-port" class="field-label">UDP 端口</FieldLabel>
                <Input id="hysteria-port" :model-value="form.hysteria2.port" class="workspace-input" type="number" min="1" max="65535" inputmode="numeric" aria-label="UDP 端口" :disabled="saving" :required="form.hysteria2.enabled" @update:model-value="updateNumber(form.hysteria2, 'port', $event)" />
                <FieldDescription class="field-description">1–65535，可与 TCP 端口使用相同数字。</FieldDescription>
              </Field>
              <Field class="node-field">
                <FieldLabel for="hysteria-domain" class="field-label">证书域名</FieldLabel>
                <Input id="hysteria-domain" v-model="form.hysteria2.domain" class="workspace-input" aria-label="证书域名" readonly />
                <FieldDescription class="field-description">由安装时签发的 TLS 证书决定。</FieldDescription>
              </Field>
            </div>
          </section>

          <Collapsible v-model:open="hysteriaAdvancedOpen" class="advanced-settings">
            <CollapsibleTrigger as-child>
              <Button type="button" variant="ghost" class="advanced-trigger">
                <span class="advanced-icon" aria-hidden="true"><KeyRound /></span>
                <span class="advanced-copy"><strong>高级认证与带宽</strong><small>连接密码与服务端带宽提示</small></span>
                <Badge variant="outline">敏感设置</Badge>
                <ChevronDown class="advanced-chevron" :class="{ open: hysteriaAdvancedOpen }" />
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div class="advanced-content">
                <Field class="node-field">
                  <FieldLabel for="hysteria-password" class="field-label">连接密码</FieldLabel>
                  <div class="secret-wrap">
                    <Input id="hysteria-password" v-model="form.hysteria2.password" class="workspace-input mono-input secret-input" :type="showHysteriaPassword ? 'text' : 'password'" autocomplete="off" spellcheck="false" aria-label="连接密码" :disabled="saving" :required="form.hysteria2.enabled" @blur="trimField(form.hysteria2, 'password')" />
                    <Button type="button" variant="ghost" size="sm" class="secret-toggle" :aria-label="showHysteriaPassword ? '隐藏 Hysteria2 连接密码' : '显示 Hysteria2 连接密码'" :aria-pressed="showHysteriaPassword" :disabled="saving" @click="showHysteriaPassword = !showHysteriaPassword">
                      <EyeOff v-if="showHysteriaPassword" /><Eye v-else />{{ showHysteriaPassword ? '隐藏密码' : '显示密码' }}
                    </Button>
                  </div>
                  <FieldDescription class="field-description danger-help">密码会进入客户端链接，请只分享给受信任设备。</FieldDescription>
                </Field>
                <Field class="node-field">
                  <FieldLabel for="hysteria-up" class="field-label">服务器上传 Mbps</FieldLabel>
                  <Input id="hysteria-up" :model-value="form.hysteria2.up_mbps" class="workspace-input" type="number" min="1" max="10000" inputmode="numeric" aria-label="服务器上传 Mbps" :disabled="saving" :required="form.hysteria2.enabled" @update:model-value="updateNumber(form.hysteria2, 'up_mbps', $event)" />
                  <FieldDescription class="field-description">用于 QUIC 拥塞控制的带宽提示。</FieldDescription>
                </Field>
                <Field class="node-field">
                  <FieldLabel for="hysteria-down" class="field-label">服务器下载 Mbps</FieldLabel>
                  <Input id="hysteria-down" :model-value="form.hysteria2.down_mbps" class="workspace-input" type="number" min="1" max="10000" inputmode="numeric" aria-label="服务器下载 Mbps" :disabled="saving" :required="form.hysteria2.enabled" @update:model-value="updateNumber(form.hysteria2, 'down_mbps', $event)" />
                  <FieldDescription class="field-description">填写服务器可稳定提供的下行带宽。</FieldDescription>
                </Field>
              </div>
            </CollapsibleContent>
          </Collapsible>
        </CardContent>
        <CardFooter class="protocol-footer">
          <div class="credential-launcher" :class="{ unavailable: !links?.hysteria2 }">
            <div><span class="credential-icon" aria-hidden="true"><KeyRound /></span><span><strong>客户端凭据</strong><small>{{ links?.hysteria2 ? '按需打开，不在工作区常驻显示' : '保存并启用协议后生成' }}</small></span></div>
            <Button type="button" variant="outline" size="sm" :disabled="saving || !links?.hysteria2" :aria-label="`打开 ${hysteria2Name} 客户端凭据`" @click="openCredentials('hysteria2')"><QrCode />{{ links?.hysteria2 ? '打开凭据' : '暂无凭据' }}</Button>
          </div>
        </CardFooter>
      </Card>
    </section>

    <div class="nodes-save-bar" :data-tone="saveState.tone" role="region" aria-label="保存节点配置">
      <span class="save-icon" aria-hidden="true"><Spinner v-if="saving" label="正在保存节点配置" /><AlertTriangle v-else-if="conflict" /><CheckCircle2 v-else-if="dirtyKnown && !dirty" /><Save v-else /></span>
      <span class="save-copy" aria-live="polite"><strong>{{ saveState.title }}</strong><small>{{ saveState.detail }}</small></span>
      <Button type="submit" size="lg" class="save-button" :disabled="saveDisabled"><Spinner v-if="saving" label="正在保存并应用" /><Save v-else />{{ saving ? '正在应用…' : conflict ? '存在冲突' : '保存并应用' }}</Button>
    </div>
  </form>

  <AlertDialog :open="disableDialogOpen" @update:open="disableDialogOpen = $event">
    <AlertDialogContent>
      <AlertDialogHeader>
        <span class="danger-icon" aria-hidden="true"><AlertTriangle /></span>
        <AlertDialogTitle>停用 {{ pendingDisableName }}？</AlertDialogTitle>
        <AlertDialogDescription>保存并应用后会停止该协议的监听，现有连接将中断，客户端链接也会失效。你可以在保存前重新启用。</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>保持启用</AlertDialogCancel>
        <AlertDialogAction class="bg-destructive text-destructive-foreground hover:bg-destructive/90" @click="confirmDisable">确认停用</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <Dialog :open="credentialDialogOpen" @update:open="handleCredentialDialog">
    <DialogContent v-if="credentialDialogOpen && credentialProtocol" class="credential-dialog">
      <DialogHeader class="dialog-header">
        <div class="dialog-title-row"><span class="dialog-icon" aria-hidden="true"><KeyRound /></span><span><DialogTitle>{{ credentialName }} 客户端凭据</DialogTitle><DialogDescription>在受信任的客户端中扫码，或复制完整链接导入。</DialogDescription></span></div>
      </DialogHeader>
      <div class="credential-grid">
        <section class="qr-panel" aria-live="polite">
          <div v-if="qrLoading" class="qr-state"><Spinner label="正在生成客户端二维码" /><span>正在本地生成二维码…</span></div>
          <div v-else-if="qrError" class="qr-state qr-error"><QrCode /><span>{{ qrError }}</span></div>
          <img v-else-if="qrDataUrl" :src="qrDataUrl" :alt="`${credentialName} 客户端二维码`" />
          <div v-else class="qr-state"><QrCode /><span>暂无二维码</span></div>
        </section>
        <section class="link-panel">
          <div class="link-heading"><Link2 aria-hidden="true" /><span><strong>完整导入链接</strong><small>包含连接地址和认证信息</small></span></div>
          <Textarea :model-value="credentialLink" readonly rows="7" class="credential-link" :aria-label="`${credentialName} 客户端链接`" @focus="$event.target.select()" />
          <Button type="button" class="copy-button" @click="emit('copy', credentialProtocol)"><Check v-if="copied === credentialProtocol" /><Clipboard v-else />{{ copied === credentialProtocol ? '已复制' : '复制链接' }}</Button>
        </section>
      </div>
      <Alert variant="warning" class="credential-warning"><ShieldAlert /><AlertTitle>将链接视同密码</AlertTitle><AlertDescription>链接包含完整客户端凭据。请勿截图、公开分享或粘贴到不受信任的网站；使用完毕后关闭此窗口。</AlertDescription></Alert>
      <DialogFooter><DialogClose as-child><Button type="button" variant="outline">关闭</Button></DialogClose></DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.nodes-workspace {
  --node-shadow: 0 1px 2px color-mix(in oklab, var(--foreground) 4%, transparent), 0 12px 32px color-mix(in oklab, var(--foreground) 5%, transparent);
  display: grid;
  min-width: 0;
  gap: 1rem;
}
.workspace-alert { margin: 0; }
.endpoint-card, .protocol-card { min-width: 0; gap: 0; padding: 0; border-color: color-mix(in oklab, var(--border) 88%, var(--foreground)); box-shadow: var(--node-shadow); }
.endpoint-card { overflow: hidden; background: radial-gradient(circle at 8% 25%, color-mix(in oklab, var(--primary) 10%, transparent), transparent 28%), var(--card); }
.endpoint-layout { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(18rem, .85fr); align-items: center; gap: 2rem; padding: 1.35rem 1.5rem; }
.endpoint-copy, .protocol-heading, .section-heading > div, .credential-launcher > div, .dialog-title-row, .link-heading { display: flex; align-items: flex-start; gap: .75rem; min-width: 0; }
.endpoint-icon, .protocol-icon, .section-icon, .advanced-icon, .credential-icon, .save-icon, .dialog-icon, .danger-icon { display: grid; flex: 0 0 auto; place-items: center; }
.endpoint-icon { width: 2.8rem; height: 2.8rem; border: 1px solid color-mix(in oklab, var(--primary) 18%, var(--border)); border-radius: .85rem; color: var(--primary); background: color-mix(in oklab, var(--primary) 9%, var(--card)); }
.endpoint-icon svg, .protocol-icon svg { width: 1.2rem; height: 1.2rem; }
.kicker-row { display: flex; flex-wrap: wrap; align-items: center; gap: .55rem; margin-bottom: .45rem; }
.workspace-kicker { color: var(--primary); font-size: .63rem; font-weight: 760; letter-spacing: .11em; }
.endpoint-title { margin: 0; font-size: 1.05rem; line-height: 1.35; }
.endpoint-description { max-width: 42rem; margin: .35rem 0 0; font-size: .75rem; line-height: 1.6; }
.node-field { min-width: 0; gap: .48rem; }
.field-label { width: fit-content; margin: 0; color: var(--foreground); font-size: .75rem; line-height: 1.25; }
.field-description { margin: 0; color: var(--muted-foreground); font-size: .65rem; line-height: 1.5; }
.danger-help { color: color-mix(in oklab, var(--destructive) 76%, var(--muted-foreground)); }
.input-prefix-wrap, .secret-wrap { position: relative; min-width: 0; }
.input-prefix-wrap > svg { position: absolute; z-index: 1; top: 50%; left: .8rem; width: 1rem; height: 1rem; color: var(--muted-foreground); transform: translateY(-50%); pointer-events: none; }
.workspace-input { min-height: 2.55rem; border-radius: .6rem; background: color-mix(in oklab, var(--background) 65%, var(--card)); color: var(--foreground); font-size: .77rem; }
.workspace-input:focus-visible { background: var(--card); }
.workspace-input[readonly] { color: var(--muted-foreground); background: var(--muted); }
.prefixed-input { padding-left: 2.35rem; }
.mono-input { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: .71rem; }
.protocol-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: stretch; gap: 1rem; min-width: 0; }
.protocol-card { --protocol-accent: var(--primary); --protocol-tint: color-mix(in oklab, var(--primary) 8%, var(--card)); position: relative; height: 100%; overflow: hidden; }
.protocol-card::before { position: absolute; z-index: 1; inset: 0 0 auto; height: 3px; background: var(--protocol-accent); content: ''; }
.hysteria-card { --protocol-accent: oklch(.59 .105 286); --protocol-tint: color-mix(in oklab, oklch(.59 .105 286) 8%, var(--card)); }
.protocol-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.3rem 1.3rem 1.15rem; background: linear-gradient(145deg, var(--protocol-tint), var(--card) 58%); }
.protocol-heading > div { min-width: 0; }
.protocol-icon { width: 2.55rem; height: 2.55rem; border: 1px solid color-mix(in oklab, var(--protocol-accent) 20%, var(--border)); border-radius: .78rem; color: var(--protocol-accent); background: color-mix(in oklab, var(--protocol-accent) 10%, var(--card)); }
.protocol-heading :deep([data-slot='card-title']) { margin: .25rem 0 0; overflow-wrap: anywhere; font-size: 1rem; line-height: 1.35; }
.protocol-heading :deep([data-slot='card-description']) { margin: .28rem 0 0; font-size: .68rem; line-height: 1.5; }
.protocol-state { display: flex; flex: 0 0 auto; flex-direction: column; align-items: flex-end; gap: .6rem; }
.status-dot { width: .38rem; height: .38rem; border-radius: 999px; background: currentColor; box-shadow: 0 0 0 3px color-mix(in oklab, currentColor 14%, transparent); }
.switch-line { display: flex; align-items: center; gap: .5rem; }
.switch-label { margin: 0; color: var(--foreground); font-size: .71rem; line-height: 1; cursor: pointer; white-space: nowrap; }
.protocol-body { display: flex; flex: 1 1 auto; flex-direction: column; gap: 1rem; padding: 1.2rem 1.3rem; }
.settings-section, .field-stack { display: grid; gap: 1rem; min-width: 0; }
.section-heading, .section-heading > div { display: flex; align-items: center; justify-content: space-between; gap: .65rem; }
.section-icon, .advanced-icon, .credential-icon { width: 2rem; height: 2rem; border-radius: .58rem; color: var(--protocol-accent); background: color-mix(in oklab, var(--protocol-accent) 9%, var(--card)); }
.section-icon svg, .advanced-icon svg, .credential-icon svg { width: .95rem; height: .95rem; }
.section-heading h4 { margin: 0; color: var(--foreground); font-size: .81rem; line-height: 1.3; }
.section-heading p { margin: .14rem 0 0; color: var(--muted-foreground); font-size: .64rem; line-height: 1.4; }
.advanced-settings { overflow: hidden; border: 1px solid var(--border); border-radius: .8rem; background: color-mix(in oklab, var(--muted) 35%, var(--card)); }
.advanced-trigger { width: 100%; height: auto; min-height: 3.8rem; justify-content: flex-start; padding: .75rem .85rem; border-radius: 0; white-space: normal; }
.advanced-copy { display: grid; min-width: 0; flex: 1; justify-items: start; gap: .1rem; text-align: left; }
.advanced-copy strong { color: var(--foreground); font-size: .75rem; }
.advanced-copy small { color: var(--muted-foreground); font-size: .63rem; line-height: 1.35; }
.advanced-chevron { width: 1rem; height: 1rem; color: var(--muted-foreground); transition: transform .18s ease; }
.advanced-chevron.open { transform: rotate(180deg); }
.advanced-content { display: grid; gap: 1rem; padding: 1rem; border-top: 1px solid var(--border); background: var(--card); }
.secret-input { padding-right: 7.2rem; }
.secret-toggle { position: absolute; top: 50%; right: .3rem; height: 2rem; padding-inline: .55rem; color: var(--muted-foreground); font-size: .66rem; transform: translateY(-50%); }
.protocol-footer { display: block; margin-top: auto; padding: 0 1.3rem 1.3rem; }
.credential-launcher { display: flex; align-items: center; justify-content: space-between; gap: .8rem; padding: .85rem; border: 1px solid var(--border); border-radius: .75rem; background: color-mix(in oklab, var(--muted) 52%, var(--card)); }
.credential-launcher.unavailable { background: color-mix(in oklab, var(--muted) 75%, var(--card)); }
.credential-launcher > div { align-items: center; }
.credential-launcher strong, .credential-launcher small { display: block; }
.credential-launcher strong { color: var(--foreground); font-size: .75rem; }
.credential-launcher small { margin-top: .18rem; color: var(--muted-foreground); font-size: .62rem; line-height: 1.4; }
.nodes-save-bar { position: sticky; z-index: 20; bottom: .75rem; display: flex; align-items: center; gap: .8rem; min-width: 0; padding: .72rem; border: 1px solid color-mix(in oklab, var(--border) 80%, var(--primary)); border-radius: .88rem; background: color-mix(in oklab, var(--card) 92%, transparent); box-shadow: 0 16px 45px color-mix(in oklab, var(--foreground) 14%, transparent); backdrop-filter: blur(18px) saturate(1.25); }
.save-icon { width: 2.3rem; height: 2.3rem; border-radius: .68rem; color: var(--primary); background: color-mix(in oklab, var(--primary) 10%, transparent); }
.save-icon svg { width: 1.02rem; height: 1.02rem; }
.nodes-save-bar[data-tone='danger'] .save-icon { color: var(--destructive); background: color-mix(in oklab, var(--destructive) 10%, transparent); }
.save-copy { display: grid; min-width: 0; flex: 1; gap: .12rem; }
.save-copy strong { color: var(--foreground); font-size: .75rem; line-height: 1.3; }
.save-copy small { color: var(--muted-foreground); font-size: .64rem; line-height: 1.45; }
.save-button { min-width: 9.2rem; }
.danger-icon, .dialog-icon { width: 2.5rem; height: 2.5rem; border-radius: .72rem; }
.danger-icon { margin-bottom: .2rem; color: var(--destructive); background: color-mix(in oklab, var(--destructive) 11%, transparent); }
.dialog-icon { color: var(--primary); background: color-mix(in oklab, var(--primary) 10%, transparent); }
.danger-icon svg, .dialog-icon svg { width: 1.15rem; height: 1.15rem; }
.credential-dialog { width: calc(100% - 2rem); max-width: 46rem; max-height: calc(100dvh - 2rem); overflow-y: auto; }
.dialog-header { padding-right: 1.7rem; }
.dialog-title-row > span:last-child { min-width: 0; }
.credential-grid { display: grid; grid-template-columns: minmax(13rem, .8fr) minmax(0, 1.2fr); gap: 1rem; }
.qr-panel, .link-panel { min-width: 0; border: 1px solid var(--border); border-radius: .82rem; background: var(--card); }
.qr-panel { display: grid; min-height: 17rem; padding: 1rem; place-items: center; background-image: linear-gradient(color-mix(in oklab, var(--border) 45%, transparent) 1px, transparent 1px), linear-gradient(90deg, color-mix(in oklab, var(--border) 45%, transparent) 1px, transparent 1px); background-size: 18px 18px; }
.qr-panel img { display: block; width: min(14.5rem, 100%); aspect-ratio: 1; padding: .4rem; border: 1px solid var(--border); border-radius: .7rem; background: #fff; }
.qr-state { display: grid; justify-items: center; gap: .65rem; color: var(--muted-foreground); font-size: .72rem; text-align: center; }
.qr-state svg { width: 1.4rem; height: 1.4rem; color: var(--primary); }
.qr-error { color: var(--destructive); }
.link-panel { display: flex; flex-direction: column; gap: .8rem; padding: 1rem; }
.link-heading { align-items: center; }
.link-heading > svg { width: 1rem; height: 1rem; color: var(--primary); }
.link-heading strong, .link-heading small { display: block; }
.link-heading strong { color: var(--foreground); font-size: .75rem; }
.link-heading small { margin-top: .18rem; color: var(--muted-foreground); font-size: .63rem; }
.credential-link { min-height: 9.4rem; resize: none; overflow-wrap: anywhere; word-break: break-all; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: .7rem; line-height: 1.55; background: var(--muted); }
.copy-button { width: 100%; margin-top: auto; }
.credential-warning { margin: 0; }
@media (max-width: 900px) {
  .endpoint-layout { grid-template-columns: 1fr; gap: 1.2rem; }
  .protocol-grid { grid-template-columns: 1fr; }
}
@media (max-width: 580px) {
  .endpoint-layout, .protocol-header, .protocol-body { padding-right: 1rem; padding-left: 1rem; }
  .protocol-header { flex-direction: column; }
  .protocol-state { width: 100%; flex-direction: row; align-items: center; justify-content: space-between; }
  .protocol-footer { padding-right: 1rem; padding-left: 1rem; }
  .credential-launcher { align-items: stretch; flex-direction: column; }
  .credential-launcher > button { width: 100%; }
  .credential-grid { grid-template-columns: 1fr; }
  .qr-panel { min-height: 13.5rem; }
  .qr-panel img { width: min(12.5rem, 100%); }
  .nodes-save-bar { bottom: .4rem; align-items: stretch; flex-wrap: wrap; }
  .save-copy { align-self: center; }
  .save-button { width: 100%; min-width: 0; }
}
@media (max-width: 390px) {
  .advanced-trigger { gap: .5rem; padding-inline: .65rem; }
  .advanced-trigger :deep([data-slot='badge']), .advanced-copy small { display: none; }
  .secret-input { padding-right: 2.8rem; }
  .secret-toggle { width: 2rem; padding: 0; font-size: 0; }
}
@media (prefers-reduced-motion: reduce) { .advanced-chevron { transition: none; } }
</style>
