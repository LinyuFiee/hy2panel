<script setup>
import { ref } from 'vue'
import { CheckCircle2, CloudCog, Database, KeyRound, LockKeyhole, RotateCcw, ShieldCheck, Trash2, Zap } from '@lucide/vue'

const props = defineProps({
  state: { type: Object, required: true },
  busy: { type: Object, required: true }
})
const emit = defineEmits(['save-racknerd', 'clear-racknerd', 'apply-bbr'])
const apiKey = ref('')
const apiHash = ref('')

function clearCredentials() {
  apiKey.value = ''
  apiHash.value = ''
}

defineExpose({ clearCredentials })
</script>

<template>
  <div class="settings-grid">
    <section class="panel-card">
      <div class="settings-heading">
        <div class="protocol-icon mint"><CloudCog :size="22" /></div>
        <div><h3>RackNerd 官方流量</h3><p>只读调用 info 接口，不执行关机或重启。</p></div>
        <span v-if="state.racknerd.configured" class="status-pill ok"><CheckCircle2 :size="14" />已配置</span>
      </div>
      <form @submit.prevent="emit('save-racknerd', { apiKey, apiHash })">
        <label>API Key<input v-model="apiKey" type="password" autocomplete="off" :placeholder="state.racknerd.configured ? '已保存，输入新 Key 可替换' : '输入每台 VPS 独立的 API Key'" /></label>
        <label>API Hash<input v-model="apiHash" type="password" autocomplete="off" :placeholder="state.racknerd.configured ? '已保存，输入新 Hash 可替换' : '输入 API Hash'" /></label>
        <p class="field-help">凭据仅保存在 VPS 的 600 权限配置中，不会通过状态 API 回显。留空的字段会保留原值。</p>
        <div class="button-row">
          <button class="primary-button" type="submit" :disabled="busy.racknerd"><KeyRound :size="16" />{{ busy.racknerd ? '正在验证…' : '保存并获取流量' }}</button>
          <button v-if="state.racknerd.configured" class="secondary-button danger-button" type="button" :disabled="busy.racknerd" @click="emit('clear-racknerd')"><Trash2 :size="16" />清除</button>
        </div>
      </form>
      <p v-if="state.racknerd.error" class="warning-text">{{ state.racknerd.error }}</p>
    </section>

    <section class="panel-card">
      <div class="settings-heading">
        <div class="protocol-icon lavender"><Zap :size="22" /></div>
        <div><h3>BBR 网络优化</h3><p>应用项目内置的 BBR + fq 保守调优模板。</p></div>
        <span class="status-pill" :class="state.status.bbr_enabled ? 'ok' : 'warn'">{{ state.status.bbr_enabled ? '正常' : '待处理' }}</span>
      </div>
      <div class="setting-readout">
        <div><span>拥塞控制</span><strong>{{ state.status.congestion_control }}</strong></div>
        <div><span>队列规则</span><strong>{{ state.status.default_qdisc }}</strong></div>
      </div>
        <p class="field-help">仅应用固定模板，不执行来自浏览器的任意系统命令。Hysteria2 不受 BBR 直接加速。</p>
      <button class="secondary-button" :disabled="busy.bbr" @click="emit('apply-bbr')"><RotateCcw :size="16" />{{ busy.bbr ? '正在应用…' : '应用 BBR + fq' }}</button>
    </section>

    <section class="panel-card security-card">
      <div class="settings-heading">
        <div class="protocol-icon peach"><LockKeyhole :size="22" /></div>
        <div><h3>访问安全</h3><p>当前安装的安全边界。</p></div>
      </div>
      <ul class="security-list">
        <li><ShieldCheck :size="16" /><span>面板域名<strong>{{ state.config.panel.domain }}:8443</strong></span></li>
        <li><ShieldCheck :size="16" /><span>管理白名单<strong>{{ state.config.panel.admin_cidr }}</strong></span></li>
        <li><ShieldCheck :size="16" /><span>传输安全<strong>HTTPS · HttpOnly · SameSite Strict</strong></span></li>
        <li><Database :size="16" /><span>敏感配置<strong>仅 root / hy2panel 可读取</strong></span></li>
      </ul>
    </section>
  </div>
</template>