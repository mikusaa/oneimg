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
        </h3>

        <div class="mb-5 grid grid-cols-3 divide-x divide-slate-200/80 border-y border-slate-200/80 py-3 dark:divide-white/10 dark:border-white/10">
          <div class="px-3 first:pl-0">
            <p class="text-xs text-gray-500 dark:text-gray-400">总容量</p>
            <p class="mt-1 truncate text-base font-semibold text-gray-800 dark:text-white">{{ storage.total_readable || '--' }}</p>
          </div>
          <div class="px-3">
            <p class="text-xs text-gray-500 dark:text-gray-400">已使用</p>
            <p class="mt-1 truncate text-base font-semibold text-gray-800 dark:text-white">{{ storage.usage_readable }}</p>
          </div>
          <div class="px-3 last:pr-0">
            <p class="text-xs text-gray-500 dark:text-gray-400">剩余容量</p>
            <p class="mt-1 truncate text-base font-semibold text-gray-800 dark:text-white">{{ storage.usage_free || '--' }}</p>
          </div>
        </div>

        <div class="mb-5">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">使用率：{{ storage.usage_percent }}%</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ storage.usage_readable }} / {{ storage.total_readable }}
            </p>
          </div>
          <div class="w-full h-2 bg-gray-200 dark:bg-dark-300 rounded-full overflow-hidden">
            <div
              class="h-full rounded-full bg-blue-500 transition-[width] duration-500 dark:bg-blue-400"
              :style="{ width: `${storage.usage_percent}%` }"
            ></div>
          </div>
        </div>

        <div v-if="storage.type === 'default'" class="border-t border-slate-200/80 pt-4 dark:border-white/10">
          <label class="field-label" :for="`default-cdn-${storage.id}`">CDN 域名</label>
          <div v-if="canUpdateCDN" class="settings-token-row">
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
          <p v-else class="text-sm text-slate-700 dark:text-slate-200">
            {{ storage.cdn_domain || '未配置' }}
          </p>
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
            v-if="canDeleteStorage"
            @click="DeleteBucketModal(storage.id)"
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

const currentUser = getStoredUser();
const canCreateStorage = hasPermission('storage:create', currentUser);
const canUpdateStorage = hasPermission('storage:update', currentUser);
const canDeleteStorage = hasPermission('storage:delete', currentUser);
const canUpdateCDN = hasPermission('setting:upload', currentUser);
const buckets = ref([]);
const cdnDomains = reactive({});
const savedCDNDomains = reactive({});
const savingCDN = ref(null);

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
  const { name, type, capacity, ...config } = formData
  return { name, type, capacity_bytes: Math.max(0, Number(capacity || 0)) * 1024 * 1024 * 1024, config }
}

const decorateBucket = (bucket) => {
  const capacity = Number(bucket.capacity_bytes || 0)
  const usage = Number(bucket.usage_bytes || 0)
  const readable = value => value ? `${(value / 1024 / 1024 / 1024).toFixed(2)} GB` : '0 B'
  return { ...bucket, key: bucket.id, total_readable: readable(capacity), usage_readable: readable(usage), usage_free: readable(Math.max(0, capacity - usage)), usage_percent: capacity ? Math.min(100, Math.round(usage / capacity * 100)) : 0 }
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
    defaultValue: field.name == 'capacity' ? formatCapacity(bucket[field.name]) : (bucket.config[field.name] ?? ''),
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

// 删除存储弹窗
const DeleteBucketModal = (id) => {
  const modal = new PopupModal({
    title: '删除存储',
    content: `
      <p>确定要删除该存储吗？</p>
      <p class="text-red-500">注意：存储下的图片存储信息也会一并删除（源文件除外），请谨慎操作！</p>
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
          modal.close();
          try {
            const response = await apiFetch(`/api/v1/storage-buckets/${id}`, {
              method: 'DELETE',
              headers: {
                'Content-Type': 'application/json',
              }
            });
            const result = response.status === 204 ? null : await response.json();
            if (response.ok) {
              message.success('存储删除成功');
              modal.close();
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
