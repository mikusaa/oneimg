<template>
  <div class="page-shell">
    <PageHeader title="存储管理" description="管理本地与远程存储及其访问地址">
      <template #actions>
        <button v-if="canCreateStorage" class="primary-button" @click="AddBucketModal">
          <i class="ri-add-line"></i>
          添加存储
        </button>
      </template>
    </PageHeader>

    <!-- 多存储卡片列表 -->
    <div class="grid grid-cols-[repeat(auto-fit,minmax(320px,1fr))] gap-6">
      <div
        v-for="storage in buckets"
        :key="storage.key"
        class="section-card relative"
      >
        <h3 class="section-title text-lg font-semibold mb-4 flex items-center gap-2">
          {{ storage.name }}
          <span class="text-xs bg-gray-100 dark:bg-dark-300 text-gray-500 dark:text-gray-300 px-2 py-0.5 rounded-full">
            {{ storage.type === 'default' ? '默认存储' : storage.type.toUpperCase() }}
          </span>
          <span v-if="storage.disabled" class="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
            已停用
          </span>
        </h3>

        <div class="mb-5 grid grid-cols-3 divide-x divide-slate-200/80 border-y border-slate-200/80 py-3 dark:divide-white/10 dark:border-white/10">
          <div class="px-3 first:pl-0">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ storage.type === 'default' ? '磁盘容量' : '配置容量' }}</p>
            <p class="mt-1 truncate text-base font-semibold text-gray-800 dark:text-white">{{ storage.total_readable }}</p>
          </div>
          <div class="px-3">
            <p class="text-xs text-gray-500 dark:text-gray-400">OneImg 占用</p>
            <p class="mt-1 flex min-w-0 items-center gap-1.5 text-base font-semibold text-gray-800 dark:text-white">
              <span class="truncate">{{ storage.usage_readable }}</span>
              <span v-if="!storage.usage_exact" class="shrink-0 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">估算</span>
            </p>
          </div>
          <div class="px-3 last:pr-0">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ storage.type === 'default' ? '图片目录可用' : '剩余配额' }}</p>
            <p class="mt-1 truncate text-base font-semibold text-gray-800 dark:text-white">{{ storage.usage_free }}</p>
          </div>
        </div>

        <div class="mb-5">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">{{ storage.type === 'default' ? '磁盘使用率' : '配额使用率' }}：{{ storage.usage_percent == null ? '--' : `${storage.usage_percent}%` }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ storage.progress_readable }}
            </p>
          </div>
          <div class="w-full h-2 bg-gray-200 dark:bg-dark-300 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full bg-blue-500 transition-[width] duration-500 dark:bg-blue-400"
              :style="{ width: `${storage.usage_percent || 0}%` }"
            ></div>
          </div>
        </div>

        <div v-if="storage.type === 'default' && canUpdateCDN" class="border-t border-slate-200/80 pt-4 dark:border-white/10">
          <label class="field-label" :for="`default-cdn-${storage.id}`">CDN 域名</label>
          <div class="settings-token-row">
            <input
              :id="`default-cdn-${storage.id}`"
              v-model="cdnDomains[storage.id]"
              type="text"
              class="input-modern min-w-0"
              placeholder="例如 https://img.example.com"
              @keyup.enter="saveDefaultStorageCDN(storage)"
            >
            <button
              type="button"
              class="soft-button shrink-0"
              :class="{ 'cursor-not-allowed opacity-50': !cdnDomainChanged(storage) || savingCDN === storage.id }"
              :disabled="!cdnDomainChanged(storage) || savingCDN === storage.id"
              @click="saveDefaultStorageCDN(storage)"
            >
              <i :class="savingCDN === storage.id ? 'ri-loader-4-line animate-spin' : 'ri-save-line'"></i>
              保存
            </button>
          </div>
          <p class="field-hint">CDN 站点根路径需指向本地 uploads 目录，返回链接会移除 /uploads 前缀。</p>
        </div>

        <div v-if="storage.type !== 'default'" class="flex items-center justify-end gap-3 pt-3 border-t border-gray-200 dark:border-dark-300">
          <button
          v-if="canUpdateStorage"
          @click="UpdateBucketModal(storage)"
          class="soft-button text-sm">
            <i class="ri-edit-fill"></i>
            编辑
          </button>
          <button
            v-if="canUpdateStorage"
            @click="storage.disabled ? EnableBucket(storage) : DisableBucketModal(storage)"
            class="soft-button text-sm">
            <i :class="storage.disabled ? 'ri-play-circle-line' : 'ri-pause-circle-line'"></i>
            {{ storage.disabled ? '启用' : '停用' }}
          </button>
          <button
            v-if="canDeleteStorage"
            @click="DeleteBucketModal(storage)"
            class="danger-button text-sm">
            <i class="ri-delete-bin-7-fill"></i>
            删除存储
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { apiFetch } from "@/api/client.ts"
import { onMounted, reactive, ref } from 'vue';
import PageHeader from '@/components/PageHeader.vue';
import message from '@/utils/message.ts';
import PopupModal from '@/utils/popupModal.ts';
import { getStoredUser, hasPermission } from '@/utils/permissions.ts';
import type { StorageBucket } from '@/api/generated';

const currentUser = getStoredUser();
const canCreateStorage = hasPermission('storage:create', currentUser);
const canUpdateStorage = hasPermission('storage:update', currentUser);
const canDeleteStorage = hasPermission('storage:delete', currentUser);
const canUpdateCDN = hasPermission('setting:upload', currentUser);
type StorageView = StorageBucket & {
  key: number;
  total_readable: string;
  usage_readable: string;
  usage_free: string;
  usage_percent: number | null;
  progress_readable: string;
  cdn_domain?: string;
};

const buckets = ref<StorageView[]>([]);
const cdnDomains = reactive<Record<number, string>>({});
const savedCDNDomains = reactive<Record<number, string>>({});
const savingCDN = ref<number | null>(null);

const typeSpecificFields = {
  s3: [
    { name: 's3_endpoint', label: 'Endpoint', type: 'text', placeholder: '请输入 Endpoint', required: true},
    { name: 's3_access_key', label: 'AccessKey', type: 'password', placeholder: '请输入 AccessKey', required: true},
    { name: 's3_secret_key', label: 'SecretKey', type: 'password', placeholder: '请输入 SecretKey', required: true},
    { name: 's3_bucket', label: 'Bucket', type: 'text', placeholder: '请输入 Bucket', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  r2: [
    { name: 'r2_endpoint', label: 'Endpoint', type: 'text', placeholder: '请输入 Endpoint', required: true},
    { name: 'r2_access_key', label: 'AccessKey', type: 'password', placeholder: '请输入 AccessKey', required: true},
    { name: 'r2_secret_key', label: 'SecretKey', type: 'password', placeholder: '请输入 SecretKey', required: true},
    { name: 'r2_bucket', label: 'Bucket', type: 'text', placeholder: '请输入 Bucket', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  ftp: [
    { name: 'ftp_host', label: 'Host', type: 'text', placeholder: '请输入 Host', required: true, tip: '无需填写 ftp:// 或者 sftp://'},
    { name: 'ftp_port', label: 'Port', type: 'number', placeholder: 'FTP 默认端口号 21', required: false, defaultValue: 21 },
    { name: 'ftp_user', label: 'Username', type: 'password', placeholder: '请输入 Username', required: true},
    { name: 'ftp_pass', label: 'Password', type: 'password', placeholder: '请输入 Password', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  webdav: [
    { name: 'webdav_url', label: 'URL', type: 'text', placeholder: '请填写 WebDav 地址', required: true},
    { name: 'webdav_user', label: 'Username', type: 'password', placeholder: '请输入 Username', required: true},
    { name: 'webdav_pass', label: 'Password', type: 'password', placeholder: '请输入 Password', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ]
};

const sensitiveFields = ['s3_access_key', 's3_secret_key', 'r2_access_key', 'r2_secret_key', 'ftp_user', 'ftp_pass', 'webdav_user', 'webdav_pass'];

const storagePayload = (formData) => {
  const { id, name, type, capacity, ...config } = formData
  const payload: Record<string, unknown> = { name, type, capacity_bytes: Math.max(0, Number(capacity || 0)) * 1024 * 1024 * 1024, config }
  if (Number(id) > 0) payload.id = Number(id)
  return payload
}

const readableBytes = (value?: number) => {
  if (value == null || !Number.isFinite(value)) return '--'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** exponent).toFixed(exponent === 0 ? 0 : 2)} ${units[exponent]}`
}

const decorateBucket = (bucket: StorageBucket): StorageView => {
  const capacity = Number(bucket.capacity_bytes || 0)
  const usage = Number(bucket.usage_bytes || 0)
  if (bucket.type === 'default') {
    const total = bucket.filesystem?.total_bytes
    const filesystemUsed = bucket.filesystem?.used_bytes
    const available = bucket.filesystem?.available_bytes
    const percent = total && filesystemUsed != null ? Math.min(100, Math.round(filesystemUsed / total * 100)) : null
    return {
      ...bucket,
      key: bucket.id,
      total_readable: readableBytes(total),
      usage_readable: readableBytes(usage),
      usage_free: readableBytes(available),
      usage_percent: percent,
      progress_readable: total == null || filesystemUsed == null ? '--' : `${readableBytes(filesystemUsed)} / ${readableBytes(total)}`,
    }
  }
  const percent = capacity > 0 ? Math.min(100, Math.round(usage / capacity * 100)) : null
  return {
    ...bucket,
    key: bucket.id,
    total_readable: capacity > 0 ? readableBytes(capacity) : '无限制',
    usage_readable: readableBytes(usage),
    usage_free: capacity > 0 ? readableBytes(Math.max(0, capacity - usage)) : '无限制',
    usage_percent: percent,
    progress_readable: capacity > 0 ? `${readableBytes(usage)} / ${readableBytes(capacity)}` : '--',
  }
}

// 添加存储弹窗
const AddBucketModal = () => {
  const baseFields = [
    {
      name: 'name',
      label: '存储名称',
      type: 'text',
      placeholder: '请输入存储名称',
      required: true,
      tip: '存储名称不能超过10个字符',
    },
    {
      name: 'type',
      label: '存储类型',
      type: 'select',
      options: [
        { label: '请选择存储类型', value: '', disabled: true },
        { label: 'S3', value: 's3' },
        { label: 'R2', value: 'r2' },
        { label: 'FTP', value: 'ftp' },
        { label: 'WebDav', value: 'webdav' },
      ],
      required: true,
      onChange: (_, type) => {
        modal.appendFormFields(typeSpecificFields[type] || [], ['name', 'type']);
      }
    }
  ];

  const modal = new PopupModal({
    title: '添加存储',
    type: 'form',
    formFields: baseFields,
    formSubmit: async (modal, formData) => {
      try {
        if (!formData.name || !formData.type) {
          message.warning('请填写存储名称和选择存储类型');
          return;
        }
        const response = await apiFetch('/api/v1/storage-buckets', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(storagePayload(formData))
        });
        const result = await response.json();
        if (response.ok && result.data) {
          message.success('存储添加成功');
          modal.close();
          GetBuckets();
        } else {
          message.error(result.detail || '添加存储失败');
        }
      } catch (error) {
        console.error('添加存储失败:', error);
        message.error('添加存储失败，请稍后重试');
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: () => {
          modal.close();
        }
      },
      {
        text: '测试连接',
        type: 'default',
        callback: (_, formData) => testBucketConnection(formData)
      },
      {
        text: '确认添加',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          );
        }
      }
    ]
  });

  modal.open();
};

// 更新存储弹窗
const UpdateBucketModal = (bucket) => {
  const setValue = typeSpecificFields[bucket.type].map(field => ({
    ...field,
    defaultValue: field.name == 'capacity' ? formatCapacity(bucket.capacity_bytes) : (bucket.config[field.name] ?? ''),
    placeholder: sensitiveFields.includes(field.name) && bucket.config?.[`${field.name}_configured`] ? '已配置，留空表示不修改' : field.placeholder,
    tip: sensitiveFields.includes(field.name) && bucket.config?.[`${field.name}_configured`] ? '当前已配置，后端不会返回明文；留空表示继续使用原值' : field.tip,
    required: sensitiveFields.includes(field.name) ? !bucket.config?.[`${field.name}_configured`] : field.required,
  }));
  const modal = new PopupModal({
    title: '编辑存储',
    type: 'form',
    formFields: [
      { name: 'name', label: '存储名称', type: 'text', placeholder: '请输入存储名称', required: true, defaultValue: bucket.name },
      { name: 'type', label: '存储类型', type: 'select', disabled: true,
      tip: '存储类型不可修改；<br><b class="text-red-500">修改配置会导致已有的图片无法访问，请谨慎操作</b>',
      options: [
        { label: '请选择存储类型', value: '', disabled: true },
        { label: 'S3', value: 's3' },
        { label: 'R2', value: 'r2' },
        { label: 'FTP', value: 'ftp' },
        { label: 'WebDav', value: 'webdav' },
      ], required: true, defaultValue: bucket.type },
      ...setValue
    ],
    formSubmit: async (modal, formData) => {
      try {
        if (!formData.name || !formData.type) {
          message.warning('请填写存储名称和选择存储类型');
          return;
        }
        const response = await apiFetch(`/api/v1/storage-buckets/${bucket.id}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(storagePayload(formData))
        });
        const result = await response.json();
        if (response.ok && result.data) {
          message.success('存储更新成功');
          modal.close();
          GetBuckets();
        } else {
          message.error(result.detail || '更新存储失败');
        }
      } catch (error) {
        console.error('更新存储失败:', error);
        message.error('更新存储失败，请稍后重试');
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: () => {
          modal.close();
        }
      },
      {
        text: '测试连接',
        type: 'default',
        callback: (_, formData) => testBucketConnection({ ...formData, id: bucket.id, type: bucket.type })
      },
      {
        text: '确认更新',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          );
        }
      }
    ]
  });
  modal.open();
}

const testBucketConnection = async (formData) => {
  try {
    if (!formData.type) {
      message.warning('请先选择存储类型');
      return;
    }
    const response = await apiFetch('/api/v1/storage-connection-tests', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(storagePayload(formData))
    });
    const result = await response.json();
    if (response.ok && result.data) {
      message.success(result.data?.detail || '连接测试成功');
    } else {
      message.error(result.detail || '连接测试失败');
    }
  } catch (error) {
    console.error('连接测试失败:', error);
    message.error('连接测试失败，请稍后重试');
  }
}

const escapeHTML = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, (char) => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
}[char] as string));

// 查询存储下的图片与视频记录数；无法获取时返回 null，弹窗仍提示删除范围。
const countBucketRecords = async (id: number): Promise<number | null> => {
  try {
    const response = await apiFetch(`/api/v1/images?bucket_id=${id}&page_size=1`);
    const result = await response.json();
    const total = result?.meta?.pagination?.total;
    return response.ok && typeof total === 'number' ? total : null;
  } catch {
    return null;
  }
};

const fetchDefaultStorageID = async (): Promise<number | null> => {
  try {
    const response = await apiFetch('/api/v1/upload-options');
    const result = await response.json();
    const id = result?.data?.default_storage;
    return response.ok && typeof id === 'number' ? id : null;
  } catch {
    return null;
  }
};

const showDefaultStorageNotice = (name: string, action: '删除' | '停用') => {
  new PopupModal({
    title: `无法${action}默认存储`,
    content: `<p>「${escapeHTML(name)}」是当前系统默认存储，${action}后上传默认值将失效。</p><p class="mt-2">请先在“系统设置 → 存储设置”中切换系统默认存储，再${action}该存储。</p>`,
    type: 'confirm',
    buttons: [{ text: '知道了', type: 'primary', callback: (modal) => modal.close() }]
  }).open();
};

// 删除存储弹窗
const DeleteBucketModal = async (storage: StorageView) => {
  const { id, name } = storage;
  const [recordCount, defaultStorageID] = await Promise.all([countBucketRecords(id), fetchDefaultStorageID()]);
  if (defaultStorageID === id) {
    showDefaultStorageNotice(name, '删除');
    return;
  }
  const countText = recordCount === null
    ? '该存储下的全部图片与视频记录'
    : `该存储下的 <strong>${recordCount}</strong> 条图片与视频记录`;
  const inputID = `delete-storage-confirm-${id}`;
  const purgeID = `delete-storage-purge-${id}`;
  const modal = new PopupModal({
    title: '删除存储',
    content: `
      <p>即将删除存储「${escapeHTML(name)}」。此操作<strong>无法撤销</strong>：</p>
      <ul class="mt-2 list-disc space-y-1 pl-5 text-sm">
        <li>删除${countText}及相关标签关联，这些文件的外链将立即失效。</li>
        <li>所有用户对该存储的使用授权将被移除。</li>
        <li>默认保留存储端的源文件和缩略图，便于自行备份或迁移。</li>
      </ul>
      <p class="mt-3 text-sm text-slate-500 dark:text-slate-400">只想停止新上传、保留外链时，请改用“停用”。</p>
      <label class="mt-4 flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300" for="${purgeID}">
        <input id="${purgeID}" type="checkbox" class="mt-0.5">
        <span>同时删除存储端文件：清理这些记录对应、且未被其他记录引用的源文件和缩略图，无法恢复。</span>
      </label>
      <label class="field-label mt-4" for="${inputID}">请输入存储名称「${escapeHTML(name)}」以确认</label>
      <input id="${inputID}" type="text" class="input-modern w-full" autocomplete="off">
    `,
    type: 'confirm',
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close()
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          const input = document.getElementById(inputID) as HTMLInputElement | null;
          const purgeObjects = (document.getElementById(purgeID) as HTMLInputElement | null)?.checked === true;
          if (!input || input.value.trim() !== name) {
            message.error('存储名称不匹配，未执行删除');
            input?.focus();
            return;
          }
          modal.close();
          try {
            const query = purgeObjects ? '?purge_objects=true' : '';
            const response = await apiFetch(`/api/v1/storage-buckets/${id}${query}`, {
              method: 'DELETE',
              headers: {
                'Content-Type': 'application/json',
              }
            });
            const result = response.status === 204 ? null : await response.json();
            if (response.ok) {
              message.success(purgeObjects ? '存储及存储端文件已删除' : '存储已删除，存储端文件已保留');
              GetBuckets();
            } else {
              message.error(result?.detail || '删除存储失败');
            }
          } catch (error) {
            console.error('删除存储失败:', error);
            message.error('删除存储失败，请稍后重试');
          }
        }
      }
    ]
  });
  modal.open();
};

const setBucketDisabled = async (storage: StorageView, disabled: boolean) => {
  try {
    const action = disabled ? 'disable' : 'enable';
    const response = await apiFetch(`/api/v1/storage-buckets/${storage.id}/${action}`, { method: 'POST' });
    const result = await response.json();
    if (response.ok) {
      message.success(disabled ? '存储已停用' : '存储已启用');
      GetBuckets();
    } else {
      message.error(result?.detail || (disabled ? '停用存储失败' : '启用存储失败'));
    }
  } catch (error) {
    console.error('切换存储状态失败:', error);
    message.error('操作失败，请稍后重试');
  }
};

const EnableBucket = (storage: StorageView) => setBucketDisabled(storage, false);

// 停用存储弹窗
const DisableBucketModal = async (storage: StorageView) => {
  if (await fetchDefaultStorageID() === storage.id) {
    showDefaultStorageNotice(storage.name, '停用');
    return;
  }
  new PopupModal({
    title: '停用存储',
    content: `
      <p>停用存储「${escapeHTML(storage.name)}」后：</p>
      <ul class="mt-2 list-disc space-y-1 pl-5 text-sm">
        <li>不再出现在上传选项中，上传和 URL 导入都会被拒绝。</li>
        <li>已有图片、外链、用户授权和存储端文件保持不变。</li>
        <li>可以随时重新启用。</li>
      </ul>
    `,
    type: 'confirm',
    buttons: [
      { text: '取消', type: 'default', callback: (modal) => modal.close() },
      {
        text: '停用',
        type: 'primary',
        callback: async (modal) => {
          modal.close();
          await setBucketDisabled(storage, true);
        }
      }
    ]
  }).open();
};

// 获取存储列表
const GetBuckets = async () => {
  try {
    const response = await apiFetch('/api/v1/storage-buckets', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      }
    });
    const result = await response.json();
    if (response.ok && Array.isArray(result.data)) {
      buckets.value = result.data.map(decorateBucket);
      let cdnDomain = '';
      if (canUpdateCDN) {
        const settingsResponse = await apiFetch('/api/v1/settings?groups=upload');
        const settingsResult = await settingsResponse.json();
        if (settingsResponse.ok && settingsResult.data) cdnDomain = settingsResult.data.cdn_domain || '';
      }
      for (const storage of buckets.value) {
        if (storage.type !== 'default') continue;
        const domain = cdnDomain;
        storage.cdn_domain = domain;
        cdnDomains[storage.id] = domain;
        savedCDNDomains[storage.id] = domain;
      }
    } else {
      message.error(result.detail || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    message.error('获取存储列表失败，请稍后重试');
  }
};

const cdnDomainChanged = (storage) => {
  return String(cdnDomains[storage.id] || '').trim() !== String(savedCDNDomains[storage.id] || '').trim();
};

const saveDefaultStorageCDN = async (storage) => {
  if (!canUpdateCDN || !cdnDomainChanged(storage) || savingCDN.value === storage.id) return;

  savingCDN.value = storage.id;
  try {
    const response = await apiFetch('/api/v1/settings', {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ cdn_domain: String(cdnDomains[storage.id] || '').trim() })
    });
    const result = await response.json();
    if (!response.ok || !result.data) {
      throw new Error(result.detail || '更新 CDN 域名失败');
    }

    const domain = result.data?.cdn_domain || '';
    cdnDomains[storage.id] = domain;
    savedCDNDomains[storage.id] = domain;
    storage.cdn_domain = domain;
    message.success('CDN 域名已更新');
  } catch (error) {
    console.error('更新 CDN 域名失败:', error);
    message.error(error.message || '更新 CDN 域名失败');
  } finally {
    savingCDN.value = null;
  }
};

// 辅助函数，存储容量转换 B -> GB
const formatCapacity = (value) => {
  if (!value) return '0';
  return (value / 1024 / 1024 / 1024).toFixed(2);
};

onMounted(() => {
  GetBuckets();
});
</script>
