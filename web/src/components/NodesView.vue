<script setup>
import { reactive, watch } from 'vue'
import { Check, ChevronDown, Clipboard, Globe2, KeyRound, Radio, Save, ShieldCheck } from '@lucide/vue'
import QRCode from 'qrcode'

const props = defineProps({
  form: { type: Object, required: true },
  links: { type: Object, default: () => ({}) },
  saving: { type: Boolean, default: false },
  copied: { type: String, default: '' }
})
const emit = defineEmits(['save', 'copy'])
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
      <section class="panel-card node-card">
        <div class="node-heading">
          <div class="protocol-icon reality-icon"><ShieldCheck :size="22" /></div>
          <div><h3>VLESS Reality</h3><p>TCP + Vision · 稳定主力</p></div>
          <label class="toggle"><input v-model="form.reality.enabled" type="checkbox" /><span></span></label>
        </div>
        <div class="form-row">
          <label>TCP 端口<input v-model.number="form.reality.port" type="number" min="1" max="65535" required /></label>
          <label>伪装 SNI<input v-model.trim="form.reality.server_name" required /></label>
        </div>
        <details class="advanced-block">
          <summary><ChevronDown :size="15" />身份与密钥</summary>
          <label>UUID<input v-model.trim="form.reality.uuid" required /></label>
          <label>Reality 私钥<input v-model.trim="form.reality.private_key" type="password" autocomplete="off" required /></label>
          <label>Reality 公钥<input v-model.trim="form.reality.public_key" readonly required /></label>
          <label>Short ID<input v-model.trim="form.reality.short_id" required /></label>
        </details>
      </section>

      <section class="panel-card node-card">
        <div class="node-heading">
          <div class="protocol-icon hy2-icon"><Radio :size="22" /></div>
          <div><h3>Hysteria2</h3><p>UDP + QUIC · 弱网备选</p></div>
          <label class="toggle"><input v-model="form.hysteria2.enabled" type="checkbox" /><span></span></label>
        </div>
        <div class="form-row">
          <label>UDP 端口<input v-model.number="form.hysteria2.port" type="number" min="1" max="65535" required /></label>
          <label>证书域名<input v-model="form.hysteria2.domain" readonly /></label>
        </div>
        <details class="advanced-block">
          <summary><ChevronDown :size="15" />密码与带宽</summary>
          <label>连接密码<input v-model.trim="form.hysteria2.password" type="password" autocomplete="off" required /></label>
          <div class="form-row">
            <label>服务器上传 Mbps<input v-model.number="form.hysteria2.up_mbps" type="number" min="1" max="10000" required /></label>
            <label>服务器下载 Mbps<input v-model.number="form.hysteria2.down_mbps" type="number" min="1" max="10000" required /></label>
          </div>
        </details>
      </section>
    </div>

    <div class="save-bar">
      <p>保存时先校验，启动失败自动恢复上一版配置。</p>
      <button class="primary-button" type="submit" :disabled="saving"><Save :size="16" />{{ saving ? '正在应用…' : '保存并应用' }}</button>
    </div>

    <section v-if="Object.keys(links).length" class="panel-card">
      <div class="panel-heading"><div><span class="section-kicker">CLIENT ACCESS</span><h3>客户端链接与二维码</h3></div><KeyRound :size="20" class="muted-icon" /></div>
      <div class="link-grid">
        <article v-for="(link, name) in links" :key="name" class="client-link">
          <div><strong>{{ name === 'reality' ? 'VLESS Reality' : 'Hysteria2' }}</strong><button class="icon-text-button" type="button" @click="emit('copy', name)"><Check v-if="copied === name" :size="15" /><Clipboard v-else :size="15" />{{ copied === name ? '已复制' : '复制' }}</button></div>
          <details>
            <summary>显示链接与二维码</summary>
            <textarea :value="link" readonly rows="4" @focus="$event.target.select()"></textarea>
            <img v-if="qr[name]" :src="qr[name]" :alt="`${name} 客户端二维码`" />
          </details>
        </article>
      </div>
      <p class="security-note">链接包含完整连接凭据，二维码仅在本地浏览器生成，请勿公开分享。</p>
    </section>
  </form>
</template>