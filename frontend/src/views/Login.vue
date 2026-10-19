<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { browserSupportsWebAuthn, startAuthentication } from '@simplewebauthn/browser'
import { api, errorMessage, type User } from '@/api/client.ts'
import AuthCard from '@/components/auth/AuthCard.vue'
import { isPasskeyCancellation, safeRedirect } from '@/components/auth/auth'
import { UiButton, UiInput, UiPasswordInput } from '@/components/ui'

const route = useRoute()
const username = ref(typeof route.query.username === 'string' ? route.query.username : '')
const password = ref('')
const error = ref('')
const busy = ref<'' | 'password' | 'passkey'>('')
const registrationEnabled = ref(false)
const passkeyAvailable = ref(false)
const passkeySupported = ref(false)
const passwordInput = ref<InstanceType<typeof UiPasswordInput> | null>(null)

// 登录信息存在 localStorage 的 userInfo 里，整页跳转以重新加载应用外壳
const finish = (user: User) => {
  localStorage.setItem('userInfo', JSON.stringify({
    id: user.id,
    username: user.username,
    role: user.role,
    permission: user.permission || { codes: [], bucket_ids: [] },
  }))
  window.location.replace(safeRedirect(route.query.redirect))
}

const login = async () => {
  if (busy.value) return
  if (!username.value.trim() || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  error.value = ''
  busy.value = 'password'
  try {
    finish(await api.login({ username: username.value.trim(), password: password.value }))
  } catch (err) {
    error.value = errorMessage(err, '登录失败，请检查网络连接')
    password.value = ''
    busy.value = ''
  }
}

const loginWithPasskey = async () => {
  if (busy.value) return
  error.value = ''
  busy.value = 'passkey'
  try {
    const { options } = await api.passkeyLoginOptions()
    const assertion = await startAuthentication({ optionsJSON: options as Parameters<typeof startAuthentication>[0]['optionsJSON'] })
    finish(await api.passkeyLoginVerify(assertion))
  } catch (err) {
    if (!isPasskeyCancellation(err)) error.value = errorMessage(err, 'Passkey 登录失败')
    busy.value = ''
  }
}

onMounted(async () => {
  passkeySupported.value = browserSupportsWebAuthn()
  try {
    const config = await api.getPublicConfig()
    registrationEnabled.value = config.registration_enabled === true
    passkeyAvailable.value = config.passkey_available === true
  } catch {
    // 读不到时只隐藏注册入口和 Passkey，不影响密码登录
  }
  if (username.value) (passwordInput.value?.$el as HTMLElement | undefined)?.querySelector('input')?.focus()
})
</script>

<template>
  <AuthCard title="登录" description="上传图片，获得可以直接引用的链接。">
    <form class="grid gap-3" novalidate @submit.prevent="login">
      <div class="flex flex-col gap-1.5">
        <label for="login-username" class="text-[12.5px] font-[550] text-oi-text">用户名</label>
        <UiInput id="login-username" v-model="username" autocomplete="username" autofocus :disabled="!!busy" />
      </div>
      <div class="flex flex-col gap-1.5">
        <label for="login-password" class="text-[12.5px] font-[550] text-oi-text">密码</label>
        <UiPasswordInput id="login-password" ref="passwordInput" v-model="password" autocomplete="current-password" :disabled="!!busy" />
      </div>
      <div v-if="error" class="flex items-center gap-1.5 rounded-[9px] bg-oi-danger-soft px-2.5 py-2 text-[12.5px] text-oi-danger" role="alert">
        <i class="ri-error-warning-line" aria-hidden="true" />{{ error }}
      </div>
      <UiButton variant="primary" type="submit" class="mt-1 !h-10" :disabled="!!busy">
        <i v-if="busy === 'password'" class="ri-loader-4-line animate-spin" aria-hidden="true" />
        {{ busy === 'password' ? '正在登录' : '登录' }}
      </UiButton>
    </form>

    <template v-if="passkeyAvailable">
      <div class="my-3.5 flex items-center gap-2.5 text-xs text-oi-faint before:h-px before:flex-1 before:bg-oi-line after:h-px after:flex-1 after:bg-oi-line">或</div>
      <UiButton
        class="!h-10 w-full"
        :disabled="!!busy || !passkeySupported"
        :title="passkeySupported ? undefined : '当前浏览器不支持 Passkey'"
        @click="loginWithPasskey"
      >
        <i :class="busy === 'passkey' ? 'ri-loader-4-line animate-spin' : 'ri-fingerprint-line'" aria-hidden="true" />
        {{ busy === 'passkey' ? '正在验证' : '使用 Passkey 登录' }}
      </UiButton>
    </template>

    <template #footer>
      还没有账号？<RouterLink v-if="registrationEnabled" to="/register" class="font-[550] text-oi-accent-2 hover:underline">注册账户</RouterLink><template v-else>请联系管理员创建</template>
    </template>
  </AuthCard>
</template>
