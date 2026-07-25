<template>
  <div class="min-h-[calc(100vh-var(--app-header-height-mobile))] flex items-center justify-center px-4 py-8 md:min-h-[calc(100vh-var(--app-header-height))]">
    <div class="section-card w-full max-w-md p-6 md:p-8">
      <div class="mb-7 flex items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-slate-900 dark:text-white">创建账户</h1>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">注册后使用本地默认存储上传图片</p>
        </div>
        <router-link to="/login" class="text-sm text-primary hover:underline">返回登录</router-link>
      </div>

      <form class="space-y-5" @submit.prevent="handleSubmit">
        <div>
          <label class="field-label" for="register-username">用户名</label>
          <input id="register-username" v-model.trim="form.username" class="input-modern" maxlength="50" autocomplete="username" placeholder="3-50 个字符" :disabled="loading" />
        </div>
        <div>
          <label class="field-label" for="register-password">密码</label>
          <div class="relative">
            <input id="register-password" v-model="form.password" :type="showPassword ? 'text' : 'password'" class="input-modern pr-11" maxlength="100" autocomplete="new-password" placeholder="至少 6 个字符" :disabled="loading" />
            <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400" :title="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
              <i :class="showPassword ? 'ri-eye-off-line' : 'ri-eye-line'"></i>
            </button>
          </div>
        </div>
        <div>
          <label class="field-label" for="register-confirm-password">确认密码</label>
          <input id="register-confirm-password" v-model="form.confirmPassword" :type="showPassword ? 'text' : 'password'" class="input-modern" maxlength="100" autocomplete="new-password" placeholder="再次输入密码" :disabled="loading" />
        </div>
        <button type="submit" class="primary-button w-full justify-center py-3" :disabled="loading">
          <i :class="loading ? 'ri-loader-4-line animate-spin' : 'ri-user-add-line'"></i>
          {{ loading ? '注册中' : '注册' }}
        </button>
      </form>
    </div>

    <div v-if="showPow" class="fixed inset-0 z-[100] flex items-center justify-center bg-slate-950/60 px-4" @click.self="closePow">
      <div class="section-card w-full max-w-md p-6">
        <div class="mb-5 flex items-center justify-between">
          <h2 class="text-lg font-semibold text-slate-900 dark:text-white">安全验证</h2>
          <button type="button" class="icon-button" title="关闭" @click="closePow"><i class="ri-close-line"></i></button>
        </div>
        <div id="register-pow-container" class="flex min-h-24 items-center justify-center"></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import message from '@/utils/message.js'

const router = useRouter()
const form = reactive({ username: '', password: '', confirmPassword: '' })
const config = reactive({ start_register: false, pow_verify: false })
const loading = ref(false)
const showPassword = ref(false)
const showPow = ref(false)
let powWidget = null

const validate = () => {
  if (form.username.length < 3 || form.username.length > 50) return '用户名长度必须在 3-50 个字符之间'
  if (form.password.length < 6 || form.password.length > 100) return '密码长度必须在 6-100 个字符之间'
  if (form.password !== form.confirmPassword) return '两次输入的密码不一致'
  return ''
}

const submitRegistration = async (powToken = '') => {
  loading.value = true
  try {
    const response = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: form.username, password: form.password, powToken })
    })
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '注册失败')
    message.success(result.message || '注册成功，请登录')
    await router.replace('/login')
  } catch (error) {
    message.error(error.message || '注册失败')
  } finally {
    loading.value = false
  }
}

const createPowWidget = () => {
  const container = document.getElementById('register-pow-container')
  if (!container) return
  container.innerHTML = ''
  powWidget = document.createElement('pow-widget')
  powWidget.setAttribute('data-pow-api-endpoint', 'https://cha.eta.im/')
  powWidget.addEventListener('solve', event => {
    closePow()
    submitRegistration(event.detail.token)
  })
  powWidget.addEventListener('error', () => {
    message.error('安全验证失败，请重试')
    closePow()
  })
  container.appendChild(powWidget)
}

const openPow = () => {
  showPow.value = true
  requestAnimationFrame(createPowWidget)
}

const closePow = () => {
  powWidget?.remove()
  powWidget = null
  showPow.value = false
}

const handleSubmit = () => {
  const error = validate()
  if (error) return message.error(error)
  if (config.pow_verify) openPow()
  else submitRegistration()
}

const loadPowScript = () => {
  if (document.querySelector('script[src="https://cha.eta.im/static/js/pow.min.js"]')) return
  const script = document.createElement('script')
  script.src = 'https://cha.eta.im/static/js/pow.min.js'
  script.onerror = () => message.error('安全验证组件加载失败')
  document.head.appendChild(script)
}

onMounted(async () => {
  try {
    const response = await fetch('/api/settings/login')
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '读取注册设置失败')
    Object.assign(config, result.data)
    if (!config.start_register) {
      message.warning('暂未开放注册')
      return router.replace('/login')
    }
    if (config.pow_verify) loadPowScript()
  } catch (error) {
    message.error(error.message || '读取注册设置失败')
    router.replace('/login')
  }
})

onBeforeUnmount(closePow)
</script>
