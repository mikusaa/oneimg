<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { browserSupportsWebAuthn } from '@simplewebauthn/browser'
import { api, errorMessage, type PasskeyRecord, type PersonalAccessToken } from '@/api/client.ts'
import AccountPasskeys from '@/components/account/AccountPasskeys.vue'
import AccountProfile from '@/components/account/AccountProfile.vue'
import AccountTokens from '@/components/account/AccountTokens.vue'
import SiteFooter from '@/components/shell/SiteFooter.vue'
import { useLogout } from '@/components/shell/useLogout'
import { toast, UiAvatar, UiButton, UiSegmented } from '@/components/ui'
import { setSessionUser, useSession } from '@/composables/useSession'
import { THEME_OPTIONS, useTheme, type ThemePreference } from '@/composables/useTheme'
import { ROLE_ADMIN } from '@/utils/permissions.ts'

const { user, roleLabel, mustChangePassword } = useSession()
const bootstrap = computed(() => !!user.value.bootstrap_verification_required)
const isAdmin = computed(() => Number(user.value.role) === ROLE_ADMIN)
const username = computed(() => user.value.username ?? '')
const joined = ref('')

/* ---------- 头像：服务器裁成正方形并转为 WebP，不计入存储配额 ---------- */
const AVATAR_MAX_BYTES = 10 * 1024 * 1024
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarBusy = ref(false)
const hasAvatar = computed(() => !!user.value.avatar_url)
const uploadAvatar = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > AVATAR_MAX_BYTES) {
    toast.error('头像文件不能超过 10 MiB')
    return
  }
  avatarBusy.value = true
  try {
    setSessionUser(await api.updateMyAvatar(file))
    toast.success('头像已更新')
  } catch (error) {
    toast.error(errorMessage(error, '上传头像失败'))
  } finally {
    avatarBusy.value = false
  }
}
const removeAvatar = async () => {
  avatarBusy.value = true
  try {
    setSessionUser(await api.deleteMyAvatar())
    toast.success('已移除头像')
  } catch (error) {
    toast.error(errorMessage(error, '移除头像失败'))
  } finally {
    avatarBusy.value = false
  }
}

/* ---------- Passkey 与令牌 ---------- */
const passkeys = ref<PasskeyRecord[]>([])
const tokens = ref<PersonalAccessToken[]>([])
const loadingPasskeys = ref(true)
const loadingTokens = ref(true)
const browserPasskeys = browserSupportsWebAuthn()
const serverPasskeys = ref(false)
const passkeyAvailable = computed(() => browserPasskeys && serverPasskeys.value)
const passkeyReason = computed(() => !browserPasskeys ? '当前浏览器不支持 Passkey。' : '服务器未启用 Passkey（需要 HTTPS 和正确的站点地址）。')
const activeTokens = computed(() => tokens.value.filter(token => !token.revoked_at && (!token.expires_at || new Date(token.expires_at).getTime() > Date.now())).length)

const loadPasskeys = async () => {
  try {
    passkeys.value = await api.listPasskeys() ?? []
  } catch (error) {
    toast.error(errorMessage(error, '读取 Passkey 失败'))
  } finally {
    loadingPasskeys.value = false
  }
}
const loadTokens = async () => {
  try {
    tokens.value = await api.listTokens() ?? []
  } catch (error) {
    toast.error(errorMessage(error, '读取令牌失败'))
  } finally {
    loadingTokens.value = false
  }
}

onMounted(async () => {
  // 需要改密的账户只能调用 /me，其他接口都会被拒绝
  if (mustChangePassword.value) return
  void api.me().then(me => { joined.value = me.created_at }).catch(() => undefined)
  void api.getPublicConfig().then(config => { serverPasskeys.value = config.passkey_available === true }).catch(() => undefined)
  await Promise.all([loadPasskeys(), loadTokens()])
})

/* ---------- 分区导航：点击滚动，滚动时高亮当前分区 ---------- */
const SECTIONS = [
  { id: 'account-profile', label: '资料与密码', icon: 'ri-user-3-line' },
  { id: 'account-passkey', label: 'Passkey', icon: 'ri-fingerprint-line' },
  { id: 'account-tokens', label: 'API 令牌', icon: 'ri-key-2-line' },
]
const current = ref(SECTIONS[0].id)
const jump = (id: string) => {
  current.value = id
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  document.getElementById(id)?.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' })
}
let observer: IntersectionObserver | null = null
onMounted(() => {
  if (mustChangePassword.value || !('IntersectionObserver' in window)) return
  observer = new IntersectionObserver(entries => {
    const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0]
    if (visible) current.value = visible.target.id
  }, { rootMargin: '0px 0px -60% 0px' })
  for (const section of SECTIONS) {
    const element = document.getElementById(section.id)
    if (element) observer.observe(element)
  }
})
onBeforeUnmount(() => observer?.disconnect())

/* ---------- 手机端「我的」：外观和退出登录（桌面端在侧栏底部） ---------- */
const { preference, setTheme } = useTheme()
const { logout, pending: loggingOut } = useLogout()
const theme = computed({
  get: () => preference.value,
  set: (value: string) => setTheme(value as ThemePreference),
})
const themeOptions = THEME_OPTIONS.map(option => ({ value: option.value, label: option.label }))
</script>

<template>
  <div class="mx-auto w-full max-w-[1100px] font-oi text-sm text-oi-text">
    <h1 class="sr-only">账户设置</h1>
    <div
      class="relative mb-5 flex flex-wrap items-center gap-[18px] overflow-hidden rounded-[22px] border border-oi-line bg-oi-panel bg-[radial-gradient(80%_140%_at_100%_0%,color-mix(in_srgb,var(--oi-sakura)_16%,transparent),transparent_60%),radial-gradient(80%_140%_at_0%_100%,color-mix(in_srgb,var(--oi-accent)_14%,transparent),transparent_60%)] p-[22px] min-[900px]:flex-nowrap"
    >
      <UiAvatar :name="username" :src="user.avatar_url" size="lg" />
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-xl font-[680] tracking-[-0.02em] text-oi-ink">{{ username }}</h2>
        <p class="mt-0.5 text-[13px] text-oi-muted">
          {{ roleLabel }}<template v-if="user.id"> · ID {{ user.id }}</template><template v-if="joined"> · 加入于 {{ new Date(joined).toLocaleDateString('zh-CN') }}</template>
        </p>
        <div v-if="!mustChangePassword" class="mt-2.5 flex flex-wrap items-center gap-2">
          <UiButton size="sm" :disabled="avatarBusy" @click="avatarInput?.click()">
            <i :class="avatarBusy ? 'ri-loader-4-line animate-spin' : 'ri-image-edit-line'" aria-hidden="true" />{{ hasAvatar ? '更换头像' : '上传头像' }}
          </UiButton>
          <UiButton v-if="hasAvatar" size="sm" variant="ghost" :disabled="avatarBusy" @click="removeAvatar">移除</UiButton>
          <span class="text-xs text-oi-muted">JPEG、PNG、GIF、WebP 静态图，不超过 10 MiB，居中裁成正方形</span>
          <input ref="avatarInput" type="file" accept="image/jpeg,image/png,image/gif,image/webp" class="hidden" aria-label="选择头像图片" @change="uploadAvatar">
        </div>
      </div>
      <dl v-if="!mustChangePassword" class="flex w-full gap-7 min-[900px]:ml-auto min-[900px]:w-auto">
        <div class="flex flex-col-reverse">
          <dt class="text-xs text-oi-muted">Passkey</dt>
          <dd class="text-[19px] font-[650] tabular-nums text-oi-ink">{{ loadingPasskeys ? '—' : passkeys.length }}</dd>
        </div>
        <div class="flex flex-col-reverse">
          <dt class="text-xs text-oi-muted">有效令牌</dt>
          <dd class="text-[19px] font-[650] tabular-nums text-oi-ink">{{ loadingTokens ? '—' : activeTokens }}</dd>
        </div>
      </dl>
    </div>

    <div v-if="mustChangePassword" class="mb-5 flex items-start gap-3 rounded-[14px] bg-oi-warn-soft px-4 py-3.5 text-oi-warn" role="alert">
      <i class="ri-lock-password-line text-[22px]" aria-hidden="true" />
      <div>
        <b class="text-oi-ink">请先修改初始密码</b>
        <p v-if="bootstrap" class="mt-0.5 text-[12.5px] text-oi-text">
          此账户存在旧默认密码风险，需要输入服务器数据目录中 <code class="font-oi-mono text-[11.5px]">.admin_recovery_code</code> 里的一次性校验码。完成后，全部历史 Passkey 和 API 令牌都会失效。
        </p>
        <p v-else class="mt-0.5 text-[12.5px] text-oi-text">改密前不能使用其他功能，改密后请用新密码重新登录。</p>
      </div>
    </div>

    <div :class="['grid items-start gap-4 min-[900px]:gap-8', mustChangePassword ? 'grid-cols-[minmax(0,720px)]' : 'grid-cols-1 min-[900px]:grid-cols-[200px_minmax(0,1fr)]']">
      <nav v-if="!mustChangePassword" class="-mx-4 flex gap-0.5 overflow-x-auto px-4 [scrollbar-width:none] min-[900px]:sticky min-[900px]:top-4 min-[900px]:mx-0 min-[900px]:flex-col min-[900px]:px-0" aria-label="账户设置分区">
        <a
          v-for="section in SECTIONS"
          :key="section.id"
          :href="`#${section.id}`"
          :aria-current="current === section.id ? 'location' : undefined"
          :class="[
            'flex h-9 flex-none items-center gap-2.5 rounded-[9px] px-3 font-[520] transition-colors [&>i]:text-[17px] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring',
            current === section.id ? 'bg-oi-panel text-oi-ink shadow-[inset_0_0_0_1px_var(--oi-line)] [&>i]:text-oi-accent' : 'text-oi-text hover:bg-oi-hover [&>i]:text-oi-muted',
          ]"
          @click.prevent="jump(section.id)"
        >
          <i :class="section.icon" aria-hidden="true" />{{ section.label }}
        </a>
      </nav>

      <div class="min-w-0">
        <section class="mb-7 min-[900px]:hidden" aria-label="外观与登录">
          <div class="divide-y divide-oi-line rounded-oi-lg border border-oi-line bg-oi-raised">
            <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
              <span id="account-theme-label" class="text-[13.5px] font-[560] text-oi-ink">外观</span>
              <UiSegmented v-model="theme" :options="themeOptions" aria-labelledby="account-theme-label" />
            </div>
            <div class="px-4 py-3">
              <UiButton variant="danger" :disabled="loggingOut" @click="logout">
                <i class="ri-logout-box-r-line" aria-hidden="true" />退出登录
              </UiButton>
            </div>
          </div>
          <SiteFooter class="mt-3.5" />
        </section>

        <AccountProfile :username="username" :can-rename="isAdmin" :must-change="mustChangePassword" :bootstrap="bootstrap" />
        <template v-if="!mustChangePassword">
          <AccountPasskeys :passkeys="passkeys" :loading="loadingPasskeys" :available="passkeyAvailable" :unavailable-reason="passkeyReason" @changed="loadPasskeys" />
          <AccountTokens :tokens="tokens" :loading="loadingTokens" :is-admin="isAdmin" @changed="loadTokens" />
        </template>
      </div>
    </div>
  </div>
</template>
