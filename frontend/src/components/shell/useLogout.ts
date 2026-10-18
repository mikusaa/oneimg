import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '@/api/client.ts'
import { confirm } from '@/components/ui/confirm'
import { toast } from '@/components/ui/toast'
import { clearSession } from '@/composables/useSession'
import { hasPendingUploads } from '@/composables/useUploadQueue'

export const useLogout = () => {
  const router = useRouter()
  const pending = ref(false)

  const logout = async () => {
    if (pending.value) return
    // 退出后上传队列会清空
    if (hasPendingUploads() && !await confirm({
      title: '退出登录？',
      description: '待上传列表里还有文件，退出后会清空，正在上传的文件可能没有保存。',
      danger: true,
      confirmText: '退出登录',
    })) return
    pending.value = true
    try {
      const response = await apiFetch('/api/v1/auth/logout', { method: 'POST' })
      // 401 说明会话已经失效，同样按已退出处理
      if (!response.ok && response.status !== 401) throw new Error(`退出登录失败：${response.status}`)
      clearSession()
      toast.success('已退出登录')
      await router.push('/login')
    } catch {
      toast.error('退出登录失败，请稍后重试')
    } finally {
      pending.value = false
    }
  }

  return { logout, pending }
}
