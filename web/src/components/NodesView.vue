<script setup>
import { computed, reactive, watch } from 'vue'
import { Check, Clipboard, Globe2, KeyRound, Radio, Save, ShieldCheck } from '@lucide/vue'
import QRCode from 'qrcode'

const props = defineProps({
  form: { type: Object, required: true },
  links: { type: Object, default: () => ({}) },
  meta: { type: Object, default: () => ({}) },
  saving: { type: Boolean, default: false },
  copied: { type: String, default: '' }
})
const emit = defineEmits(['save', 'copy'])
const realityName = computed(() => props.meta.reality_name || 'VLESS Reality')
const hysteria2Name = computed(() => props.meta.hysteria2_name || 'Hysteria2')
const qr = reactive({ reality: '', hysteria2: '' })
const previousLinks = {}

watch(() => props.links, async links => {
  for (const name of ['reality', 'hysteria2']) {
    if (previousLinks[name] === links[name]) continue
    previousLinks[name] = links[name]
    qr[name] = links[name] ? await QRCode.toDataURL(links[name], { width: 360, margin: 1, color: { dark: '#29463c', light: '#ffffff' } }) : ''
  }
}, { immediate: true, deep: true })
</script>

<template>
  <form class="view-stack" @submit.prevent="emit('save')">
    <section class="panel-card address-card">
      <div>
        <span class="section-kicker">PUBLIC ENDPOINT</span>
        <h3>节点公网地址</h3>
        <p>Reality 客户端使用此地址连接，可以是 IPv4、IPv6 或域名。</p>
      </div>
      <div class="inline-input"><Globe2 :size="18" /><input v-model.trim="form.public_host" aria-label="节点公网地址" required /></div>
    </section>

    <div class="node-grid">
      <section class="panel-card node-card reality-card">
        <div class="node-heading">
          <div class="protocol-icon reality-icon"><ShieldCheck :size="22" /></div>
          <div><h3>{{ realityName }}</h3><p>TCP + Vision · 稳定主力</p></div>
          <label class="toggle"><input v-model="form.reality.enabled" type="checkbox" aria-label="启用 Reality" /><span></span></label>
        </div>

        <div class="config-section">
          <span class="config-title">连接设置</span>
          <div class="form-row">
            <label>TCP 端口<input v-model.number="form.reality.port" type="number" min="1" max="65535" required /></label>
            <label>伪装 SNI<input v-model.trim="form.reality.server_name" required /></label>
          </div>
        </div>

        <div class="config-section">
          <span class="config-title">身份与密钥</span>
          <label>UUID<input v-model.trim="form.reality.uuid" required /></label>
          <label>Reality 私钥<input v-model.trim="form.reality.private_key" type="password" autocomplete="off" required /></label>
          <label>Reality 公钥<input v-model.trim="form.reality.public_key" readonly required /></label>
          <label>Short ID<input v-model.trim="form.reality.short_id" required /></label>
        </div>

        <div class="client-section">
          <div class="client-heading"><KeyRound :size="16" /><span>客户端接入</span><button v-if="links.reality" class="icon-text-button" type="button" @click="emit('copy', 'reality')"><Check v-if="copied === 'reality'" :size="15" /><Clipboard v-else :size="15" />{{ copied === 'reality' ? '已复制' : '复制链接' }}</button></div>
          <textarea v-if="links.reality" :value="links.reality" readonly rows="3" class="link-text" aria-label="Reality 客户端链接" @focus="$event.target.select()"></textarea>
          <p v-else class="access-empty">保存并应用后生成客户端链接。</p>
          <div class="qr-wrap"><img v-if="qr.reality" :src="qr.reality" alt="Reality 客户端二维码" /><span v-else class="qr-empty">暂无二维码</span></div>
          <p class="access-note">使用支持 VLESS Reality 的客户端扫码或复制链接导入</p>
        </div>
      </section>

      <section class="panel-card node-card hy2-card">
        <div class="node-heading">
          <div class="protocol-icon hy2-icon"><Radio :size="22" /></div>
          <div><h3>{{ hysteria2Name }}</h3><p>UDP + QUIC · 弱网备选</p></div>
          <label class="toggle"><input v-model="form.hysteria2.enabled" type="checkbox" aria-label="启用 Hysteria2" /><span></span></label>
        </div>

        <div class="config-section">
          <span class="config-title">连接</span>
          <div class="form-row">
            <label>UDP 端口<input v-model.number="form.hysteria2.port" type="number" min="1" max="65535" required /></label>
            <label>证书域名<input v-model="form.hysteria2.domain" readonly /></label>
          </div>
        </div>

        <div class="config-section">
          <span class="config-title">认证与带宽</span>
          <label>连接密码<input v-model.trim="form.hysteria2.password" type="password" autocomplete="off" required /></label>
          <div class="form-row">
            <label>服务器上传 Mbps<input v-model.number="form.hysteria2.up_mbps" type="number" min="1" max="10000" required /></label>
            <label>服务器下载 Mbps<input v-model.number="form.hysteria2.down_mbps" type="number" min="1" max="10000" required /></label>
          </div>
        </div>

        <div class="client-section">
          <div class="client-heading"><KeyRound :size="16" /><span>客户端接入</span><button v-if="links.hysteria2" class="icon-text-button" type="button" @click="emit('copy', 'hysteria2')"><Check v-if="copied === 'hysteria2'" :size="15" /><Clipboard v-else :size="15" />{{ copied === 'hysteria2' ? '已复制' : '复制链接' }}</button></div>
          <textarea v-if="links.hysteria2" :value="links.hysteria2" readonly rows="3" class="link-text" @focus="$event.target.select()" aria-label="Hysteria2 客户端链接"></textarea>
          <p v-else class="access-empty">保存并应用后生成客户端链接。</p>
          <div class="qr-wrap"><img v-if="qr.hysteria2" :src="qr.hysteria2" alt="Hysteria2 客户端二维码" /><span v-else class="qr-empty">暂无二维码</span></div>
          <p class="access-note">使用支持 Hysteria2 的客户端扫码或复制链接导入</p>
        </div>
      </section>
    </div>

    <div class="save-bar">
      <p>保存时先校验，启动失败自动恢复上一版配置。链接包含完整凭据，请勿公开分享。</p>
      <button class="primary-button" type="submit" :disabled="saving"><Save :size="16" />{{ saving ? '正在应用…' : '保存并应用' }}</button>
    </div>
  </form>
</template>