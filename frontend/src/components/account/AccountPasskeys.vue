<script setup lang="ts">
import { computed, nextTick, reactive, ref } from 'vue'
import { startRegistration } from '@simplewebauthn/browser'
import { api, errorMessage, type PasskeyRecord } from '@/api/client.ts'
import { isPasskeyCancellation } from '@/components/auth/auth'
import SettingSection from '@/components/settings/SettingSection.vue'
import { toast, UiButton, UiInput, UiPasswordInput } from '@/components/ui'
import { formatDateTime } from '@/utils/format'

const MAX_PASSKEYS = 10

const props = defineProps<{
  passkeys: PasskeyRecord[]
  loading: boolean
  /** 浏览器和服务器都支持 */
  available: boolean
  /** 不可用的原因 */
  unavailableReason: string
}>()
const emit = defineEmits<{ changed: [] }>()

const full = computed(() => props.passkeys.length >= MAX_PASSKEYS)

/* ---------- 添加 ---------- */
const form = reactive({ name: '', password: '' })
const addErrors = reactive<Record<string, string>>({})
const adding = ref(false)
const add = async () => {
  if (adding.value || full.value) return
  for (const key of Object.keys(addErrors)) delete addErrors[key]
  const name = form.name.trim()
  if (!name) addErrors.name = '请填写设备名称'
  if (!form.password) addErrors.password = '请输入当前密码'
  if (Object.keys(addErrors).length) return
  adding.value = true
  try {
    const { options } = await api.passkeyRegistrationOptions({ name, current_password: form.password })
    const registration = await startRegistration({ optionsJSON: options as Parameters<typeof startRegistration>[0]['optionsJSON'] })
    await api.passkeyRegistrationVerify(registration)
    form.name = ''
    toast.success('Passkey 已添加')
    emit('changed')
  } catch (error) {
    if (!isPasskeyCancellation(error)) {
      const message = errorMessage(error, '添加 Passkey 失败')
      if (message.includes('密码')) addErrors.password = message
      else toast.error(message)
    }
  } finally {
    form.password = ''
    adding.value = false
  }
}

/* ---------- 重命名 ---------- */
const editingId = ref<number | null>(null)
const editingName = ref('')
const busyId = ref<number | null>(null)
const startRename = async (item: PasskeyRecord) => {
  deletingId.value = null
  editingId.value = item.id
  editingName.value = item.name
  await nextTick()
  const input = document.getElementById(`passkey-name-${item.id}`) as HTMLInputElement | null
  input?.focus()
  input?.select()
}
const rename = async (item: PasskeyRecord) => {
  const name = editingName.value.trim()
  if (!name) return toast.error('设备名称不能为空')
  if (name === item.name) { editingId.value = null; return }
  busyId.value = item.id
  try {
    await api.renamePasskey(item.id, name)
    editingId.value = null
    toast.success('名称已更新')
    emit('changed')
  } catch (error) {
    toast.error(errorMessage(error, '重命名失败'))
  } finally {
    busyId.value = null
  }
}

/* ---------- 删除：需要当前密码 ---------- */
const deletingId = ref<number | null>(null)
const deletePassword = ref('')
const deleteError = ref('')
const startDelete = async (item: PasskeyRecord) => {
  editingId.value = null
  deletingId.value = item.id
  deletePassword.value = ''
  deleteError.value = ''
  await nextTick()
  document.getElementById(`passkey-delete-${item.id}`)?.querySelector('input')?.focus()
}
const remove = async (item: PasskeyRecord) => {
  if (!deletePassword.value) { deleteError.value = '请输入当前密码'; return }
  busyId.value = item.id
  deleteError.value = ''
  try {
    await api.revokePasskey(item.id, deletePassword.value)
    deletingId.value = null
    toast.success(`已删除「${item.name}」`)
    emit('changed')
  } catch (error) {
    deleteError.value = errorMessage(error, '删除失败')
  } finally {
    deletePassword.value = ''
    busyId.value = null
  }
}
</script>

<template>
  <SettingSection id="account-passkey" title="Passkey">
    <template #badge>
      <span
        :class="['inline-flex h-[22px] items-center gap-1 rounded-full px-2 text-[11.5px] font-[550]', available ? 'bg-oi-accent-soft text-oi-accent-2' : 'bg-oi-panel text-oi-muted']"
        :title="available ? undefined : unavailableReason"
      >
        <i :class="available ? 'ri-checkbox-circle-line' : 'ri-forbid-line'" aria-hidden="true" />{{ available ? '此浏览器可用' : '不可用' }}
      </span>
    </template>
    <template #description>
      用 Touch ID、Face ID、Windows Hello 或安全密钥登录，不需要输入密码。已绑定 {{ passkeys.length }} / {{ MAX_PASSKEYS }}。
      <template v-if="!available">{{ unavailableReason }}</template>
    </template>

    <form
      v-if="available"
      class="grid grid-cols-1 items-start gap-3 rounded-t-oi-lg bg-oi-panel px-4 py-4 min-[900px]:grid-cols-[1fr_1fr_auto] min-[900px]:px-[18px]"
      novalidate
      @submit.prevent="add"
    >
      <div class="flex flex-col gap-1.5">
        <label for="passkey-new-name" class="text-[12.5px] font-[550] text-oi-text">设备名称</label>
        <UiInput id="passkey-new-name" v-model="form.name" maxlength="50" placeholder="例如 MacBook Touch ID" :disabled="adding || full" :invalid="!!addErrors.name" />
        <p v-if="addErrors.name" class="text-xs text-oi-danger" role="alert">{{ addErrors.name }}</p>
      </div>
      <div class="flex flex-col gap-1.5">
        <label for="passkey-new-password" class="text-[12.5px] font-[550] text-oi-text">当前密码</label>
        <UiPasswordInput id="passkey-new-password" v-model="form.password" autocomplete="current-password" placeholder="确认身份" :disabled="adding || full" :invalid="!!addErrors.password" />
        <p v-if="addErrors.password" class="text-xs text-oi-danger" role="alert">{{ addErrors.password }}</p>
      </div>
      <UiButton variant="primary" type="submit" class="min-[900px]:mt-[23px]" :disabled="adding || full" :title="full ? `最多绑定 ${MAX_PASSKEYS} 个` : undefined">
        <i :class="adding ? 'ri-loader-4-line animate-spin' : 'ri-add-line'" aria-hidden="true" />{{ adding ? '等待验证' : '添加 Passkey' }}
      </UiButton>
    </form>

    <div v-if="loading && !passkeys.length" class="flex h-20 items-center justify-center text-oi-muted" aria-busy="true">
      <i class="ri-loader-4-line animate-spin text-lg" aria-hidden="true" /><span class="sr-only">正在读取</span>
    </div>
    <div v-else-if="!passkeys.length" class="px-4 py-7 text-center text-[13px] text-oi-muted">尚未绑定 Passkey</div>
    <div
      v-for="item in passkeys"
      :key="item.id"
      class="flex flex-wrap items-center gap-3.5 px-4 py-3.5 min-[900px]:px-[18px]"
    >
      <span class="grid h-[38px] w-[38px] flex-none place-items-center rounded-[11px] border border-oi-line bg-oi-panel text-lg text-oi-text" aria-hidden="true"><i class="ri-fingerprint-line" /></span>
      <div class="min-w-0 flex-1">
        <form v-if="editingId === item.id" class="flex flex-wrap items-center gap-2" @submit.prevent="rename(item)">
          <UiInput
            :id="`passkey-name-${item.id}`"
            v-model="editingName"
            maxlength="50"
            class="!h-8 min-w-0 flex-1"
            aria-label="设备名称"
            @keydown.esc.prevent="editingId = null"
          />
          <UiButton size="sm" variant="primary" type="submit" :disabled="busyId === item.id">保存</UiButton>
          <UiButton size="sm" variant="ghost" @click="editingId = null">取消</UiButton>
        </form>
        <template v-else>
          <b class="block truncate font-[580] text-oi-ink" :title="item.name">{{ item.name }}</b>
          <div class="mt-0.5 text-[12.5px] text-oi-muted">
            添加于 {{ formatDateTime(item.created_at) }} · {{ item.last_used_at ? `最近使用 ${formatDateTime(item.last_used_at)}` : '尚未使用' }}
          </div>
        </template>
      </div>
      <template v-if="editingId !== item.id">
        <UiButton size="sm" variant="ghost" icon title="重命名" :aria-label="`重命名 ${item.name}`" @click="startRename(item)"><i class="ri-edit-line" aria-hidden="true" /></UiButton>
        <UiButton size="sm" variant="ghost" icon class="hover:!text-oi-danger" title="删除" :aria-label="`删除 ${item.name}`" @click="startDelete(item)"><i class="ri-delete-bin-6-line" aria-hidden="true" /></UiButton>
      </template>
      <form
        v-if="deletingId === item.id"
        :id="`passkey-delete-${item.id}`"
        class="flex basis-full flex-wrap items-start gap-2 min-[900px]:ml-[52px]"
        novalidate
        @submit.prevent="remove(item)"
      >
        <div class="min-w-[200px] max-w-[280px] flex-1">
          <UiPasswordInput
            v-model="deletePassword"
            class="[&_input]:!h-8 [&_button]:!top-px"
            :revealable="false"
            autocomplete="current-password"
            placeholder="输入当前密码确认删除"
            :aria-label="`输入当前密码确认删除 ${item.name}`"
            :invalid="!!deleteError"
            @keydown.esc.prevent="deletingId = null"
          />
          <p v-if="deleteError" class="mt-1 text-xs text-oi-danger" role="alert">{{ deleteError }}</p>
        </div>
        <UiButton size="sm" variant="ghost" @click="deletingId = null">取消</UiButton>
        <UiButton size="sm" variant="danger-solid" type="submit" :disabled="busyId === item.id">确认删除</UiButton>
      </form>
    </div>
  </SettingSection>
</template>
