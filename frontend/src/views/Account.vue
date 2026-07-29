<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <PageHeader title="账户设置" description="更新当前账户的用户名和登录密码" />

        <!-- 主要内容 -->
        <div class="pb-16">
            <div class="mx-auto grid max-w-2xl grid-cols-1 gap-6">

                <div class="section-card mx-auto w-full overflow-hidden">
                    <div class="panel-content p-6 md:p-8">
                        <!-- 账户修改表单 -->
                        <form @submit.prevent="updateAccount" class="account-form space-y-6">
                            <!-- 新用户名 -->
                            <div v-if="isAdmin" class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="newUsername"
                                >
                                    新用户名（留空则不修改）
                                </label>
                                <input 
                                    id="newUsername"
                                    v-model="accountForm.newUsername"
                                    type="text" 
                                    class="input-modern"
                                    placeholder="留空则不修改用户名"
                                    minlength="3"
                                    maxlength="64"
                                />
                            </div>
                            
                            <!-- 当前密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="currentPassword"
                                >
                                    当前密码 <span class="text-red-500">*</span>
                                </label>
                                <input 
                                    id="currentPassword"
                                    v-model="accountForm.currentPassword"
                                    type="password" 
                                    class="input-modern"
                                    placeholder="请输入当前密码以确认修改"
                                    required
                                />
                            </div>
                            
                            <!-- 新密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="newPassword"
                                >
                                    新密码（留空则不修改）
                                </label>
                                <input 
                                    id="newPassword"
                                    v-model="accountForm.newPassword"
                                    type="password" 
                                    class="input-modern"
                                    placeholder="留空则不修改密码（至少6位）"
                                    minlength="6"
                                />
                            </div>
                            
                            <!-- 确认新密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="confirmPassword"
                                >
                                    确认新密码
                                </label>
                                <input 
                                    id="confirmPassword"
                                    v-model="accountForm.confirmPassword"
                                    type="password" 
                                    class="input-modern"
                                    placeholder="请再次输入新密码"
                                />
                            </div>
                            
                            <!-- 提交按钮 -->
                            <div class="setting-group pt-2">
                                <button 
                                    type="submit" 
                                    :disabled="isUpdatingAccount"
                                    class="primary-button w-full px-6 py-3"
                                >
                                    <span v-if="isUpdatingAccount" class="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></span>
                                    <span>保存修改</span>
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import message from '@/utils/message.js'
import { getStoredUser, ROLE_ADMIN } from '@/utils/permissions.js'

const router = useRouter()
const isAdmin = Number(getStoredUser()?.role) === ROLE_ADMIN

// 表单数据
const accountForm = ref({
    newUsername: '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
})

// 加载状态
const isUpdatingAccount = ref(false)

// 更新账户信息
const updateAccount = async () => {
    const { newUsername, currentPassword, newPassword, confirmPassword } = accountForm.value
    
    // 检查是否有任何修改
    const hasUsernameChange = isAdmin && newUsername && newUsername.trim() !== ''
    const hasPasswordChange = newPassword && newPassword.trim() !== ''
    
    if (!hasUsernameChange && !hasPasswordChange) {
        message.error('请输入要修改的用户名或密码')
        return
    }
    
    // 验证用户名（如果要修改）
    if (hasUsernameChange) {
        if (newUsername.length < 3) {
            message.error('用户名长度至少为3位')
            return
        }
        
        if (newUsername.length > 64) {
            message.error('用户名长度不能超过64位')
            return
        }
    }
    
    // 验证密码（如果要修改）
    if (hasPasswordChange) {
        if (newPassword.length < 6) {
            message.error('新密码长度至少为6位')
            return
        }
        
        if (newPassword !== confirmPassword) {
            message.error('两次输入的新密码不一致')
            return
        }
    }
    
    // 验证当前密码
    if (!currentPassword) {
        message.error('请输入当前密码以确认修改')
        return
    }
    
    try {
        isUpdatingAccount.value = true
        
        const response = await fetch('/api/account/change', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: JSON.stringify({
                new_username: newUsername,
                current_password: currentPassword,
                new_password: newPassword
            })
        })
        
        const result = await response.json()
        
        if (!response.ok || !result.success) {
            // 未授权处理
            if (response.status === 401) {
                localStorage.removeItem('authToken')
                router.push('/login')
                return message.error('登录已过期，请重新登录')
            }
            throw new Error(result.message || '修改失败')
        }
        
        message.success('修改成功，请重新登录')

        // 清空表单
        accountForm.value = {
            newUsername: '',
            currentPassword: '',
            newPassword: '',
            confirmPassword: ''
        }
        
        localStorage.removeItem('userInfo')
        localStorage.removeItem('authToken')
        setTimeout(() => router.replace('/login'), 800)

    } catch (error) {
        message.error(error.message || '更新失败')
    } finally {
        isUpdatingAccount.value = false
    }
}
</script>
