<script setup lang="ts">
import { ref } from 'vue'
import { UiButton, UiDialog, UiDrawer, UiField, UiInput, UiMenu, UiMenuItem, UiMenuLabel, UiMenuRadioGroup, UiMenuRadioItem, UiMenuSegmented, UiMenuSeparator, UiPopover, UiSegmented, UiSelect, UiSwitch, confirm, toast } from '@/components/ui'
import { useTheme } from '@/composables/useTheme'

// 开发环境专用预览页（/_ui），用来核对基础组件在浅色 / 深色下的外观和键盘交互

const swatches = ['bg', 'sheet', 'raised', 'panel', 'hover', 'line', 'line-2', 'ink', 'text', 'muted', 'faint', 'accent', 'accent-2', 'accent-soft', 'sakura', 'sakura-soft', 'warn', 'warn-soft', 'danger', 'danger-soft', 'btn', 'btn-ink']
const variants = ['default', 'primary', 'accent', 'ghost', 'danger', 'danger-solid'] as const

const { isDark: dark, setTheme } = useTheme()
const toggleDark = (value: boolean) => setTheme(value ? 'dark' : 'light')
const menuView = ref('grid')
const storage = ref('2')
const popoverNote = ref('')

const name = ref('')
const domain = ref('img.example.com')
const note = ref('')
const enabled = ref(true)
const off = ref(false)
const format = ref('markdown')
const sort = ref('new')
const view = ref('grid')
const dialogOpen = ref(false)
const drawerOpen = ref(false)
const lastResult = ref('')

const askDelete = async () => {
  const ok = await confirm({
    title: '删除存储「S3 备份」？',
    description: '以下内容会被删除，且无法撤销：',
    points: ['2 条图片与视频记录及其标签关联，这些文件的外链立即失效', '1 个普通用户对它的「可用存储」授权'],
    danger: true,
    requireText: 'S3 备份',
    confirmText: '删除存储',
  })
  lastResult.value = ok ? '已确认删除' : '已取消'
}

const askNormal = async () => {
  const ok = await confirm({ title: '停用存储「NAS 归档」？', description: '停用后不再出现在上传选项中，已有文件和外链保持不变。', confirmText: '停用', icon: 'ri-pause-circle-line' })
  lastResult.value = ok ? '已确认停用' : '已取消'
}
</script>

<template>
  <div class="min-h-screen rounded-oi-lg bg-oi-bg p-6 font-oi text-oi-text max-md:p-4">
    <header class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-[22px] font-[680] text-oi-ink">基础组件预览</h1>
        <p class="mt-1 text-[13px] text-oi-muted">仅开发环境可见。交互由 Reka UI 提供，外观来自 tokens.css 的 --oi- 变量。</p>
      </div>
      <UiSegmented
        :model-value="dark ? 'dark' : 'light'"
        aria-label="预览主题"
        :options="[{ value: 'light', label: '浅色', icon: 'ri-sun-line' }, { value: 'dark', label: '深色', icon: 'ri-moon-line' }]"
        @update:model-value="v => toggleDark(v === 'dark')"
      />
    </header>

    <section class="mb-6 rounded-oi-lg border border-oi-line bg-oi-sheet p-5">
      <h2 class="mb-3 text-[15px] font-[650] text-oi-ink">颜色变量</h2>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-2.5">
        <div v-for="s in swatches" :key="s" class="flex items-center gap-2 text-xs">
          <span class="h-7 w-7 flex-none rounded-lg border border-oi-line" :style="{ background: `var(--oi-${s})` }" />
          <code class="font-oi-mono text-oi-muted">{{ s }}</code>
        </div>
      </div>
    </section>

    <section class="mb-6 rounded-oi-lg border border-oi-line bg-oi-sheet p-5">
      <h2 class="mb-3 text-[15px] font-[650] text-oi-ink">按钮</h2>
      <div class="flex flex-col gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <UiButton v-for="v in variants" :key="v" :variant="v">{{ v }}</UiButton>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton v-for="v in variants" :key="v" :variant="v" size="sm"><i class="ri-add-line" />{{ v }}</UiButton>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiButton icon aria-label="复制"><i class="ri-file-copy-line" /></UiButton>
          <UiButton icon variant="ghost" aria-label="设置"><i class="ri-settings-3-line" /></UiButton>
          <UiButton icon size="sm" variant="danger" aria-label="删除"><i class="ri-delete-bin-6-line" /></UiButton>
          <UiButton disabled>不可用</UiButton>
          <UiButton variant="primary" disabled>不可用</UiButton>
          <UiButton as="a" href="#/_ui" variant="ghost"><i class="ri-link" />作为链接</UiButton>
        </div>
      </div>
    </section>

    <section class="mb-6 grid gap-5 rounded-oi-lg border border-oi-line bg-oi-sheet p-5 md:grid-cols-2">
      <h2 class="text-[15px] font-[650] text-oi-ink md:col-span-2">输入框</h2>
      <UiField label="存储名称" for="sc-name" hint="最多 10 个字符">
        <UiInput id="sc-name" v-model="name" placeholder="例如 R2 主存储" maxlength="10" />
      </UiField>
      <UiField label="直链域名" for="sc-domain" :error="domain.includes(' ') ? '域名不能包含空格' : ''">
        <UiInput id="sc-domain" v-model="domain" mono :invalid="domain.includes(' ')" />
      </UiField>
      <UiField label="说明" for="sc-note" class="md:col-span-2">
        <UiInput id="sc-note" v-model="note" multiline placeholder="多行输入" />
      </UiField>
      <UiField label="禁用状态" for="sc-dis">
        <UiInput id="sc-dis" model-value="不可编辑" disabled />
      </UiField>
      <UiField label="下拉选择" for="sc-sort">
        <div class="flex flex-wrap items-center gap-2">
          <UiSelect id="sc-sort" v-model="sort">
            <option value="new">最新上传</option>
            <option value="old">最早上传</option>
            <option value="size">文件最大</option>
          </UiSelect>
          <UiSelect v-model="sort" size="sm" aria-label="小尺寸下拉选择">
            <option value="new">最新上传</option>
            <option value="old">最早上传</option>
            <option value="size">文件最大</option>
          </UiSelect>
          <UiSelect model-value="new" disabled aria-label="禁用的下拉选择">
            <option value="new">不可选择</option>
          </UiSelect>
        </div>
      </UiField>
    </section>

    <section class="mb-6 grid gap-5 rounded-oi-lg border border-oi-line bg-oi-sheet p-5 md:grid-cols-2">
      <h2 class="text-[15px] font-[650] text-oi-ink md:col-span-2">开关与分段控件</h2>
      <div class="flex flex-col gap-3 text-[13.5px]">
        <label class="flex items-center justify-between gap-3">允许上传视频 <UiSwitch v-model="enabled" /></label>
        <label class="flex items-center justify-between gap-3">开放注册（默认关）<UiSwitch v-model="off" /></label>
        <label class="flex items-center justify-between gap-3 opacity-80">不可用 <UiSwitch :model-value="true" disabled /></label>
      </div>
      <div class="flex flex-col items-start gap-3">
        <UiSegmented
          v-model="format"
          aria-label="链接格式"
          :options="[{ value: 'url', label: 'URL' }, { value: 'markdown', label: 'Markdown' }, { value: 'html', label: 'HTML' }, { value: 'bbcode', label: 'BBCode' }]"
        />
        <UiSegmented
          v-model="view"
          aria-label="视图"
          :options="[{ value: 'grid', icon: 'ri-layout-grid-line', title: '网格' }, { value: 'list', icon: 'ri-list-check', title: '列表' }]"
        />
        <p class="text-xs text-oi-muted">当前：{{ format }} · {{ view }}（方向键切换）</p>
      </div>
    </section>

    <section class="mb-6 rounded-oi-lg border border-oi-line bg-oi-sheet p-5">
      <h2 class="mb-3 text-[15px] font-[650] text-oi-ink">弹窗、抽屉与提示</h2>
      <div class="flex flex-wrap items-center gap-2">
        <UiDialog v-model:open="dialogOpen" title="从 URL 导入" description="由服务器下载该图片，再按当前存储和标签保存。" icon="ri-link" close-button>
          <template #trigger><UiButton>普通弹窗</UiButton></template>
          <UiField label="图片地址" for="sc-url"><UiInput id="sc-url" placeholder="https://" /></UiField>
          <template #footer="{ close }">
            <UiButton variant="ghost" @click="close">取消</UiButton>
            <UiButton variant="primary" @click="close(); toast.success('已加入待上传列表')">导入</UiButton>
          </template>
        </UiDialog>
        <UiButton variant="danger" @click="askDelete">危险确认（需输入名称）</UiButton>
        <UiButton @click="askNormal">普通确认</UiButton>
        <UiDrawer v-model:open="drawerOpen" title="lin 的权限" description="普通用户 · ID 3">
          <template #trigger><UiButton>抽屉</UiButton></template>
          <UiField label="用户名" for="sc-user"><UiInput id="sc-user" model-value="lin" /></UiField>
          <label class="flex items-center justify-between gap-3 text-[13.5px]">可以使用 S3 备份 <UiSwitch :model-value="true" /></label>
          <p v-for="n in 12" :key="n" class="text-[13px] text-oi-muted">内容较多时抽屉主体单独滚动（第 {{ n }} 行）</p>
          <template #footer="{ close }">
            <UiButton variant="ghost" @click="close">取消</UiButton>
            <UiButton variant="primary" @click="close(); toast.success('已保存 lin 的权限')">保存</UiButton>
          </template>
        </UiDrawer>
        <UiButton variant="ghost" @click="toast.success('已复制链接')">成功提示</UiButton>
        <UiButton variant="ghost" @click="toast.error('上传失败：超过图片 10 MB 上限')">错误提示</UiButton>
        <UiButton variant="ghost" @click="toast.info('正在读取视频信息…')">普通提示</UiButton>
      </div>
      <p v-if="lastResult" class="mt-3 text-xs text-oi-muted">确认结果：{{ lastResult }}</p>
    </section>

    <section class="mb-6 rounded-oi-lg border border-oi-line bg-oi-sheet p-5">
      <h2 class="mb-3 text-[15px] font-[650] text-oi-ink">下拉菜单与气泡</h2>
      <div class="flex flex-wrap gap-2.5">
        <UiMenu>
          <template #trigger><UiButton><i class="ri-more-fill" />更多操作</UiButton></template>
          <UiMenuItem icon="ri-key-2-line" @select="toast.info('重置密码')">重置密码</UiMenuItem>
          <UiMenuItem icon="ri-fingerprint-line" @select="toast.info('撤销 Passkey')">撤销全部 Passkey</UiMenuItem>
          <UiMenuItem icon="ri-forbid-line" disabled>不可用的操作</UiMenuItem>
          <UiMenuSeparator />
          <UiMenuItem icon="ri-delete-bin-6-line" danger @select="toast.error('删除用户')">删除用户</UiMenuItem>
        </UiMenu>
        <UiMenu side="top" align="end">
          <template #trigger><UiButton variant="ghost">向上展开</UiButton></template>
          <UiMenuItem to="/_ui" icon="ri-links-line">站内链接<template #end><i class="ri-check-line text-oi-accent" /></template></UiMenuItem>
          <UiMenuSeparator />
          <UiMenuLabel>视图（选择后不关闭）</UiMenuLabel>
          <UiMenuSegmented
            v-model="menuView"
            aria-label="视图"
            :options="[{ value: 'grid', icon: 'ri-grid-line', title: '网格' }, { value: 'list', icon: 'ri-list-check', title: '列表' }]"
          />
        </UiMenu>
        <UiMenu>
          <template #trigger><UiButton><i class="ri-database-2-line" />单选：存储 {{ storage }}</UiButton></template>
          <UiMenuLabel>选择存储</UiMenuLabel>
          <UiMenuRadioGroup v-model="storage">
            <UiMenuRadioItem value="1">本地存储</UiMenuRadioItem>
            <UiMenuRadioItem value="2">Cloudflare R2</UiMenuRadioItem>
            <UiMenuRadioItem value="3" disabled>FTP 旧站（已停用）</UiMenuRadioItem>
          </UiMenuRadioGroup>
        </UiMenu>
        <UiPopover label="气泡示例">
          <template #trigger><UiButton variant="ghost"><i class="ri-chat-3-line" />气泡</UiButton></template>
          <template #default="{ close }">
            <div class="flex w-[260px] flex-col gap-2.5 p-3.5">
              <UiField label="备注" for="popover-note"><UiInput id="popover-note" v-model="popoverNote" placeholder="Esc 或点外部关闭" /></UiField>
              <UiButton size="sm" variant="primary" class="self-end" @click="close">完成</UiButton>
            </div>
          </template>
        </UiPopover>
      </div>
    </section>
  </div>
</template>
