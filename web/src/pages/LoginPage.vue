<script setup>
import { computed } from 'vue'
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
</script>

<template>
  <main id="main-content" class="login-page" tabindex="-1">
    <header class="login-header">
      <div class="brand-lockup" aria-label="hy2panel">
        <img src="/logo.png" alt="" />
        <span>
          <strong>hy2panel</strong>
          <small>PRIVATE CONTROL PLANE</small>
        </span>
      </div>
      <ThemeToggle compact />
    </header>

    <div class="login-layout">
      <section class="login-editorial" aria-labelledby="login-product-title">
        <div class="editorial-copy">
          <span class="section-index">SECURITY / 01</span>
          <h1 id="login-product-title">安静地管理<br />你的单节点服务。</h1>
          <p>Reality、Hysteria2 与主机维护汇入同一个私有控制平面，只保留管理员需要的上下文。</p>
        </div>

        <figure class="security-figure">
          <div class="figure-rail" aria-hidden="true">
            <span>ACCESS OBJECT</span>
            <span>POINTER RESPONSIVE</span>
          </div>
          <div class="figure-surface">
            <HairlineFigure
              class="security-drawing"
              figure="padlock"
              name="padlock"
              :play="false"
              :intensity="0.46"
              theme="auto"
              label="交互式安全插图：移动指针可查看挂锁开启动作，用于说明受保护的管理员入口"
            />
          </div>
          <figcaption>
            <strong>交互式安全插图</strong>
            <span>移动指针探索挂锁结构；它用于表达安全入口，不显示账户或会话数据。</span>
          </figcaption>
        </figure>
      </section>

      <section class="login-access" aria-labelledby="login-heading">
        <div class="access-inner">
          <span class="section-index">AUTHORIZED ACCESS / 02</span>
          <h2 id="login-heading">管理员登录</h2>
          <p class="access-intro">输入当前面板的管理员密码，继续进入控制台。</p>

          <Alert v-if="error" variant="destructive" class="login-alert" aria-live="assertive">
            <AlertTitle>无法登录</AlertTitle>
            <AlertDescription id="login-error">{{ error }}</AlertDescription>
          </Alert>

          <form class="login-form" :aria-busy="busy" @submit.prevent="emit('submit')">
            <label class="login-label" for="admin-password">管理员密码</label>
            <Input
              id="admin-password"
              v-model="passwordModel"
              type="password"
              autocomplete="current-password"
              placeholder="输入管理员密码"
              :aria-invalid="Boolean(error)"
              :aria-describedby="error ? 'login-error' : 'password-note'"
              :disabled="busy"
              required
              autofocus
            />
            <p id="password-note" class="field-note">凭据仅用于建立当前管理会话。</p>
            <Button type="submit" size="lg" class="login-submit" :disabled="busy">
              {{ busy ? '正在验证…' : '登录' }}
            </Button>
          </form>

          <div class="access-boundary" aria-label="安全边界说明">
            <span>SESSION</span>
            <p><strong>受保护的管理入口</strong><small>HttpOnly · SameSite Strict · 登录限速</small></p>
          </div>
        </div>
      </section>
    </div>

    <footer class="login-footer">
      <span>hy2panel / NODE CONTROL</span>
      <a href="https://github.com/LinyuFiee/hy2panel" target="_blank" rel="noopener noreferrer">GitHub</a>
    </footer>
  </main>
</template>

<style scoped>
.login-page {
  --login-rule: color-mix(in srgb, var(--foreground) 10%, transparent);
  min-height: 100svh;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  gap: clamp(1rem, 2.5vh, 1.75rem);
  padding: clamp(1rem, 2.4vw, 2rem);
  background: var(--background);
  color: var(--foreground);
}

.login-page:focus {
  outline: none;
}

.login-header,
.login-footer,
.login-layout {
  width: min(72rem, 100%);
  min-width: 0;
  margin-inline: auto;
}

.login-header,
.login-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.brand-lockup {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.brand-lockup img {
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  border: 1px solid var(--border);
  border-radius: 0.45rem;
  background: var(--card);
  object-fit: contain;
}

.brand-lockup strong,
.brand-lockup small {
  display: block;
}

.brand-lockup strong {
  font-size: 0.83rem;
  font-weight: 650;
  letter-spacing: -0.015em;
}

.brand-lockup small,
.section-index,
.figure-rail,
.login-footer,
.access-boundary > span {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.brand-lockup small {
  margin-top: 0.1rem;
  color: var(--muted-foreground);
  font-size: 0.52rem;
  letter-spacing: 0.11em;
}

.login-layout {
  align-self: center;
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(20rem, 0.88fr);
  min-height: clamp(32rem, calc(100svh - 10rem), 39rem);
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.9rem;
  background: var(--card);
  box-shadow: 0 1rem 3rem color-mix(in srgb, var(--foreground) 6%, transparent);
}

.login-editorial,
.login-access {
  min-width: 0;
  padding: clamp(1.4rem, 3vw, 2.75rem);
}

.login-editorial {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: clamp(1.5rem, 3vh, 2.5rem);
  border-right: 1px solid var(--border);
  background:
    linear-gradient(var(--login-rule) 1px, transparent 1px),
    linear-gradient(90deg, var(--login-rule) 1px, transparent 1px),
    var(--background);
  background-size: 2.5rem 2.5rem;
}

.editorial-copy {
  position: relative;
  z-index: 1;
  max-width: 35rem;
}

.section-index {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--muted-foreground);
  font-size: 0.6rem;
  font-weight: 650;
  letter-spacing: 0.12em;
}

.section-index::before {
  width: 1.5rem;
  height: 1px;
  content: '';
  background: currentColor;
  opacity: 0.7;
}

.editorial-copy h1 {
  margin: 0.85rem 0 0.8rem;
  font-size: clamp(2.25rem, 4.4vw, 4.25rem);
  font-weight: 640;
  line-height: 1.02;
  letter-spacing: -0.055em;
}

.editorial-copy p {
  max-width: 31rem;
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.82rem;
  line-height: 1.75;
}

.security-figure {
  width: min(27rem, 100%);
  min-width: 0;
  align-self: end;
  justify-self: center;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 0.72rem;
  background: var(--background);
}

.figure-rail {
  min-height: 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.55rem 0.75rem;
  border-bottom: 1px solid var(--border);
  color: var(--muted-foreground);
  font-size: 0.52rem;
  letter-spacing: 0.08em;
}

.figure-surface {
  min-width: 0;
  padding: 0.25rem;
  background: var(--background);
  --hairline-plate: var(--background);
}

.security-drawing {
  display: block;
  width: 100%;
  min-width: 0;
}

.security-figure figcaption {
  display: grid;
  gap: 0.22rem;
  padding: 0.7rem 0.8rem 0.75rem;
  border-top: 1px solid var(--border);
}

.security-figure figcaption strong {
  font-size: 0.7rem;
  font-weight: 650;
}

.security-figure figcaption span {
  color: var(--muted-foreground);
  font-size: 0.64rem;
  line-height: 1.55;
}

.login-access {
  display: grid;
  place-items: center;
  background: var(--card);
}

.access-inner {
  width: min(22.5rem, 100%);
  min-width: 0;
}

.access-inner h2 {
  margin: 0.85rem 0 0.45rem;
  font-size: clamp(1.75rem, 3vw, 2.35rem);
  font-weight: 650;
  letter-spacing: -0.04em;
}

.access-intro {
  margin: 0 0 1.75rem;
  color: var(--muted-foreground);
  font-size: 0.8rem;
  line-height: 1.65;
}

.login-alert {
  width: 100%;
  margin-bottom: 0.95rem;
}

.login-alert :deep([data-slot='alert-description']) {
  color: inherit;
}

.login-form {
  width: 100%;
  display: grid;
  gap: 0.6rem;
}

.login-label {
  color: var(--foreground);
  font-size: 0.74rem;
  font-weight: 650;
}

.login-form :deep(input) {
  height: 2.85rem;
  border-radius: 0.55rem;
  background: var(--background);
  font-size: 0.9rem;
}

.field-note {
  margin: 0;
  color: var(--muted-foreground);
  font-size: 0.66rem;
  line-height: 1.5;
}

.login-submit {
  width: 100%;
  height: 2.85rem;
  margin-top: 0.35rem;
  border-radius: 0.55rem;
  font-size: 0.86rem;
}

.access-boundary {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.75rem;
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}

.access-boundary > span {
  padding-top: 0.15rem;
  color: var(--muted-foreground);
  font-size: 0.52rem;
  letter-spacing: 0.09em;
}

.access-boundary p,
.access-boundary strong,
.access-boundary small {
  display: block;
  margin: 0;
}

.access-boundary strong {
  color: var(--foreground);
  font-size: 0.7rem;
  font-weight: 650;
}

.access-boundary small {
  margin-top: 0.2rem;
  color: var(--muted-foreground);
  font-size: 0.62rem;
  line-height: 1.45;
}

.login-footer {
  color: var(--muted-foreground);
  font-size: 0.55rem;
  letter-spacing: 0.08em;
}

.login-footer a {
  color: inherit;
  text-decoration: none;
}

.login-footer a:hover {
  color: var(--foreground);
  text-decoration: underline;
  text-underline-offset: 0.2rem;
}

@media (max-width: 780px) {
  .login-page {
    display: block;
    padding: 0.9rem;
  }

  .login-header {
    margin-bottom: 1rem;
  }

  .login-layout {
    grid-template-columns: minmax(0, 1fr);
    min-height: 0;
  }

  .login-access {
    grid-row: 1;
    min-height: 29rem;
    border-bottom: 1px solid var(--border);
  }

  .login-editorial {
    grid-row: 2;
    border-right: 0;
  }

  .editorial-copy h1 {
    font-size: clamp(2.1rem, 10vw, 3.25rem);
  }

  .security-figure {
    width: min(24rem, 100%);
  }

  .login-footer {
    margin-top: 1rem;
  }
}

@media (max-width: 420px) {
  .login-page {
    padding: 0.75rem;
  }

  .brand-lockup small {
    display: none;
  }

  .login-editorial,
  .login-access {
    padding: 1.15rem;
  }

  .login-access {
    min-height: 27rem;
  }

  .editorial-copy h1 {
    font-size: clamp(1.95rem, 12vw, 2.7rem);
  }

  .figure-rail span:last-child {
    display: none;
  }

  .login-footer {
    padding-inline: 0.15rem;
    letter-spacing: 0.04em;
  }
}
</style>
