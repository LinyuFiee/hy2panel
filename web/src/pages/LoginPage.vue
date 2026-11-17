<script setup>
import { computed } from 'vue'
import { ArrowRight, KeyRound, LockKeyhole, ShieldCheck } from '@lucide/vue'
import HairlineFigure from '@/components/HairlineFigure.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps({
  password: { type: String, default: '' },
  error: { type: String, default: '' },
  busy: { type: Boolean, default: false }
})

const emit = defineEmits(['update:password', 'submit'])

const passwordModel = computed({
  get: () => props.password,
  set: value => emit('update:password', value)
})

const figureTheme = computed(() => document.documentElement.classList.contains('dark') ? 'dark' : 'light')
</script>

<template>
  <main id="main-content" class="login-control-plane" tabindex="-1">
    <section class="login-visual" aria-labelledby="login-product-title">
      <header class="login-brand-row">
        <a class="login-brand" href="#login" aria-label="hy2panel 登录页">
          <img src="/logo.png" alt="" />
          <span>
            <strong>hy2panel</strong>
            <small>PRIVATE CONTROL PLANE</small>
          </span>
        </a>
        <ThemeToggle compact />
      </header>

      <div class="login-visual-copy">
        <span class="login-kicker"><i aria-hidden="true"></i> EDGE NODE CONSOLE</span>
        <h1 id="login-product-title">一个入口，掌握<br />节点的每次呼吸。</h1>
        <p>为 Reality 与 Hysteria2 设计的私有控制平面。状态、流量与维护操作集中呈现，不增加多余层级。</p>
      </div>

      <div class="login-figure-wrap">
        <div class="login-figure-meta" aria-hidden="true">
          <span>ROUTE / LIVE</span>
          <span>6U · 8 PORTS</span>
        </div>
        <HairlineFigure
          play
          :intensity="0.72"
          :theme="figureTheme"
          density="regular"
          label="自动播放的服务器机架路径：流量从入口经过配线端口和服务器刀片到达出口"
        />
      </div>

      <footer class="login-visual-footer">
        <span><ShieldCheck :size="14" /> HTTPS ONLY</span>
        <span>SESSION / STRICT</span>
        <span>CONTROL / LOCAL</span>
      </footer>
    </section>

    <section class="login-access" aria-labelledby="login-heading">
      <div class="login-access-inner">
        <div class="login-access-mark" aria-hidden="true"><LockKeyhole :size="22" /></div>
        <span class="login-access-kicker">AUTHORIZED ACCESS</span>
        <h2 id="login-heading">进入控制平面</h2>
        <p class="login-access-intro">使用管理员凭据继续。会话仅通过当前面板的安全 Cookie 保存。</p>

        <Alert v-if="error" variant="destructive" class="login-alert" aria-live="assertive">
          <KeyRound />
          <AlertTitle>无法登录</AlertTitle>
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>

        <form class="login-form-new" @submit.prevent="emit('submit')">
          <label for="admin-password">管理员密码</label>
          <div class="login-password-field">
            <KeyRound :size="17" aria-hidden="true" />
            <Input
              id="admin-password"
              v-model="passwordModel"
              type="password"
              autocomplete="current-password"
              placeholder="输入管理员密码"
              :aria-invalid="Boolean(error)"
              :disabled="busy"
              required
              autofocus
            />
          </div>
          <Button type="submit" size="lg" class="login-submit" :disabled="busy">
            <span>{{ busy ? '正在验证…' : '验证并进入' }}</span>
            <ArrowRight :size="17" />
          </Button>
        </form>

        <div class="login-security-note">
          <ShieldCheck :size="16" />
          <p><strong>受保护的管理入口</strong><span>登录限速 · HttpOnly · SameSite Strict</span></p>
        </div>
      </div>

      <footer class="login-access-footer">
        <span>hy2panel</span>
        <a href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">GitHub</a>
      </footer>
    </section>
  </main>
</template>

<style scoped>
.login-control-plane {
  --login-grid: color-mix(in oklab, var(--foreground) 7%, transparent);
  min-height: 100svh;
  display: grid;
  grid-template-columns: minmax(0, 1.42fr) minmax(25rem, 0.78fr);
  overflow: hidden;
  background: var(--background);
  color: var(--foreground);
}

.login-control-plane:focus {
  outline: none;
}

.login-visual {
  position: relative;
  min-height: 100svh;
  display: grid;
  grid-template-rows: auto minmax(0, auto) minmax(14rem, 1fr) auto;
  gap: clamp(1.25rem, 3vh, 2.75rem);
  overflow: hidden;
  padding: clamp(1.25rem, 3vw, 2.5rem) clamp(1.25rem, 4vw, 4.5rem) 1.5rem;
  border-right: 1px solid var(--border);
  background:
    linear-gradient(var(--login-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--login-grid) 1px, transparent 1px),
    radial-gradient(circle at 18% 18%, color-mix(in oklab, var(--primary) 16%, transparent), transparent 32%),
    color-mix(in oklab, var(--background) 90%, var(--primary) 10%);
  background-size: 2.5rem 2.5rem, 2.5rem 2.5rem, auto, auto;
}

.login-visual::before,
.login-visual::after {
  position: absolute;
  z-index: 0;
  content: '';
  pointer-events: none;
}

.login-visual::before {
  top: -9rem;
  right: -8rem;
  width: 25rem;
  height: 25rem;
  border: 1px solid color-mix(in oklab, var(--primary) 22%, transparent);
  border-radius: 50%;
}

.login-visual::after {
  right: 4.5rem;
  bottom: 4rem;
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--primary);
  box-shadow: 0 0 0 0.4rem color-mix(in oklab, var(--primary) 12%, transparent), 0 0 2.75rem 1.5rem color-mix(in oklab, var(--primary) 17%, transparent);
}

.login-brand-row,
.login-visual-copy,
.login-figure-wrap,
.login-visual-footer {
  position: relative;
  z-index: 1;
}

.login-brand-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.login-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  color: inherit;
  text-decoration: none;
}

.login-brand img {
  width: 2.5rem;
  height: 2.5rem;
  border: 1px solid color-mix(in oklab, var(--border) 78%, transparent);
  border-radius: 0.7rem;
  background: var(--card);
  object-fit: contain;
}

.login-brand strong,
.login-brand small {
  display: block;
}

.login-brand strong {
  font-size: 0.95rem;
  letter-spacing: -0.02em;
}

.login-brand small {
  margin-top: 0.15rem;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  letter-spacing: 0.14em;
}

.login-visual-copy {
  max-width: 46rem;
  align-self: end;
  padding-top: clamp(1rem, 4vh, 4rem);
}

.login-kicker,
.login-access-kicker {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.68rem;
  font-weight: 720;
  letter-spacing: 0.14em;
}

.login-kicker i {
  width: 0.38rem;
  height: 0.38rem;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 0.25rem color-mix(in oklab, currentColor 12%, transparent);
}

.login-visual-copy h1 {
  max-width: 44rem;
  margin: 1rem 0 1rem;
  color: var(--foreground);
  font-size: clamp(2.6rem, 5.2vw, 5.4rem);
  font-weight: 690;
  line-height: 0.98;
  letter-spacing: -0.065em;
}

.login-visual-copy p {
  max-width: 38rem;
  margin: 0;
  color: var(--muted-foreground);
  font-size: clamp(0.9rem, 1.3vw, 1.05rem);
  line-height: 1.8;
}

.login-figure-wrap {
  width: min(46rem, 100%);
  min-height: 0;
  align-self: end;
  justify-self: center;
  overflow: hidden;
  border: 1px solid color-mix(in oklab, var(--border) 84%, transparent);
  border-radius: 1rem;
  background: color-mix(in oklab, var(--card) 68%, transparent);
  box-shadow: 0 1.4rem 4rem color-mix(in oklab, var(--foreground) 8%, transparent);
  backdrop-filter: blur(12px);
}

.login-figure-meta {
  display: flex;
  justify-content: space-between;
  padding: 0.7rem 0.85rem;
  border-bottom: 1px solid var(--border);
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  letter-spacing: 0.1em;
}

.login-figure-wrap :deep(.hairline-figure) {
  min-height: 13rem;
  max-height: 22rem;
  aspect-ratio: 2 / 1;
}

.login-visual-footer {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.58rem;
  letter-spacing: 0.1em;
}

.login-visual-footer span {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.login-access {
  min-height: 100svh;
  display: flex;
  flex-direction: column;
  padding: clamp(1.25rem, 3vw, 2.5rem);
  background: var(--card);
}

.login-access-inner {
  width: min(23.5rem, 100%);
  margin: auto;
}

.login-access-mark {
  width: 3rem;
  height: 3rem;
  display: grid;
  place-items: center;
  margin-bottom: 2rem;
  border: 1px solid color-mix(in oklab, var(--primary) 25%, var(--border));
  border-radius: 0.85rem;
  background: color-mix(in oklab, var(--primary) 10%, var(--card));
  color: var(--primary);
}

.login-access h2 {
  margin: 0.8rem 0 0.65rem;
  color: var(--foreground);
  font-size: clamp(2rem, 3vw, 2.65rem);
  font-weight: 680;
  letter-spacing: -0.045em;
}

.login-access-intro {
  margin: 0 0 2rem;
  color: var(--muted-foreground);
  font-size: 0.875rem;
  line-height: 1.75;
}

.login-alert {
  margin-bottom: 1.25rem;
}

.login-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

.login-form-new {
  display: grid;
  gap: 0.75rem;
}

.login-form-new > label {
  margin: 0;
  color: var(--foreground);
  font-size: 0.76rem;
  font-weight: 650;
}

.login-password-field {
  position: relative;
}

.login-password-field > svg {
  position: absolute;
  z-index: 1;
  top: 50%;
  left: 0.85rem;
  transform: translateY(-50%);
  color: var(--muted-foreground);
  pointer-events: none;
}

.login-password-field :deep(input) {
  height: 3rem;
  padding-left: 2.65rem;
  background: var(--background);
  font-size: 0.875rem;
}

.login-submit {
  width: 100%;
  height: 3rem;
  justify-content: space-between;
  margin-top: 0.35rem;
  padding-inline: 1.05rem;
}

.login-security-note {
  display: flex;
  align-items: flex-start;
  gap: 0.7rem;
  margin-top: 1.75rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--border);
  color: var(--muted-foreground);
}

.login-security-note > svg {
  flex: 0 0 auto;
  margin-top: 0.1rem;
  color: var(--primary);
}

.login-security-note p,
.login-security-note strong,
.login-security-note span {
  display: block;
  margin: 0;
}

.login-security-note strong {
  color: var(--foreground);
  font-size: 0.72rem;
}

.login-security-note span {
  margin-top: 0.2rem;
  font-size: 0.66rem;
}

.login-access-footer {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  color: var(--muted-foreground);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.62rem;
  letter-spacing: 0.08em;
}

.login-access-footer a {
  color: inherit;
  text-underline-offset: 0.2rem;
}

@media (max-width: 900px) {
  .login-control-plane {
    display: block;
    overflow: visible;
  }

  .login-visual,
  .login-access {
    min-height: auto;
  }

  .login-visual {
    grid-template-rows: auto auto auto;
    padding-bottom: 2rem;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }

  .login-visual-copy {
    padding-top: 3rem;
  }

  .login-visual-copy h1 {
    font-size: clamp(2.8rem, 10vw, 4.5rem);
  }

  .login-visual-footer {
    display: none;
  }

  .login-figure-wrap {
    margin-top: 1rem;
  }

  .login-access {
    min-height: 34rem;
  }
}

@media (max-width: 560px) {
  .login-visual {
    gap: 1.5rem;
    padding: 1rem;
  }

  .login-brand small {
    display: none;
  }

  .login-visual-copy {
    padding-top: 2rem;
  }

  .login-visual-copy h1 {
    margin-top: 0.8rem;
    font-size: clamp(2.45rem, 13vw, 3.4rem);
  }

  .login-visual-copy p {
    font-size: 0.85rem;
  }

  .login-figure-wrap :deep(.hairline-figure) {
    min-height: 11rem;
  }

  .login-access {
    min-height: 31rem;
    padding: 2.5rem 1.25rem 1.25rem;
  }

  .login-access-footer {
    margin-top: 2.5rem;
  }
}
</style>
