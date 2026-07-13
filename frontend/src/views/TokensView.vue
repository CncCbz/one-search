<template>
  <div>
    <div class="page-hd">
      <h1>接口令牌</h1>
      <div class="page-actions">
        <el-button type="primary" :icon="Plus" circle title="新增令牌" :disabled="loading && !loaded" @click="openCreate" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="table" :rows="5" />
    <template v-else>
    <el-alert v-if="rawToken" type="success" show-icon :closable="false" class="token-alert">
      <template #title>
        <div class="raw-token-row">
          <span>只显示一次</span>
          <code>{{ rawToken }}</code>
          <el-button :icon="CopyDocument" circle size="small" title="复制" @click="copyText(rawToken)" />
        </div>
      </template>
    </el-alert>

    <el-card class="soft-card" shadow="never" v-loading="loading">
      <el-table :data="tokens" stripe>
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column label="令牌" min-width="160">
          <template #default="scope">
            <span class="mono">{{ displayToken(scope.row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="渠道" min-width="140">
          <template #default="scope">
            <div class="provider-tags">
              <el-tag v-if="scope.row.allowed_providers.length === 0" size="small" type="info">全部</el-tag>
              <el-tag v-for="provider in scope.row.allowed_providers" :key="provider" size="small">{{ providerLabel(provider) }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'enabled' ? 'success' : 'info'">
              {{ scope.row.status === 'enabled' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rate_limit_per_min" label="RPM" width="80" align="right" />
        <el-table-column label="额度" min-width="120">
          <template #default="scope">
            <div class="quota-cell">
              <span>日 {{ formatQuota(scope.row.daily_quota) }}</span>
              <small>月 {{ formatQuota(scope.row.monthly_quota) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="usage_count" label="使用" width="90" align="right" />
        <el-table-column label="" width="132" align="right">
          <template #default="scope">
            <div class="row-actions">
              <el-button link :icon="Edit" title="编辑" @click="openEdit(scope.row)" />
              <el-button
                link
                :icon="scope.row.status === 'enabled' ? Remove : CircleCheck"
                :title="scope.row.status === 'enabled' ? '停用' : '启用'"
                @click="setStatus(scope.row, scope.row.status === 'enabled' ? 'disabled' : 'enabled')"
              />
              <el-button link type="danger" :icon="Delete" title="删除" @click="remove(scope.row.id)" />
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    </template>

    <el-dialog v-model="dialog" :title="editingToken ? '编辑接口令牌' : '新增接口令牌'" width="520px">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="允许请求渠道">
          <el-select v-model="form.allowed_providers" multiple collapse-tags collapse-tags-tooltip placeholder="不选择表示全部渠道">
            <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="每分钟限制（0 表示不限）"><el-input-number v-model="form.rate_limit_per_min" :min="0" /></el-form-item>
        <el-form-item label="日额度（0 表示不限）"><el-input-number v-model="form.daily_quota" :min="0" /></el-form-item>
        <el-form-item label="月额度（0 表示不限）"><el-input-number v-model="form.monthly_quota" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="saveToken">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { CircleCheck, CopyDocument, Delete, Edit, Plus, Remove } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, ApiToken } from '../api/client'
import { providerLabel, providerOptions } from '../utils/providers'

const loading = ref(true)
const loaded = ref(false)
const tokens = ref<ApiToken[]>([])
const dialog = ref(false)
const rawToken = ref('')
const editingToken = ref<ApiToken | null>(null)
const form = reactive({
  name: '默认客户端',
  scopes: ['search'],
  allowed_providers: [] as string[],
  rate_limit_per_min: 0,
  daily_quota: 0,
  monthly_quota: 0
})

async function load() {
  loading.value = true
  try {
    tokens.value = (await api.tokens()).tokens
    loaded.value = true
  } finally {
    loading.value = false
  }
}

function displayToken(token: ApiToken) {
  return token.token ? token.token : `${token.token_prefix}...`
}

function formatQuota(value: number) {
  if (!value) return '不限'
  return new Intl.NumberFormat('en-US').format(value)
}

function resetForm() {
  form.name = '默认客户端'
  form.scopes = ['search']
  form.allowed_providers = []
  form.rate_limit_per_min = 0
  form.daily_quota = 0
  form.monthly_quota = 0
}

function openCreate() {
  editingToken.value = null
  resetForm()
  dialog.value = true
}

function openEdit(token: ApiToken) {
  editingToken.value = token
  form.name = token.name
  form.scopes = token.scopes || ['search']
  form.allowed_providers = [...(token.allowed_providers || [])]
  form.rate_limit_per_min = token.rate_limit_per_min
  form.daily_quota = token.daily_quota
  form.monthly_quota = token.monthly_quota || 0
  dialog.value = true
}

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function saveToken() {
  if (editingToken.value) {
    await api.updateToken(editingToken.value.id, { ...form })
    ElMessage.success('令牌已保存')
  } else {
    const result = await api.createToken(form)
    rawToken.value = result.raw_token
    ElMessage.success('令牌已创建')
  }
  dialog.value = false
  await load()
}

async function setStatus(token: ApiToken, status: string) {
  await api.updateToken(token.id, { status })
  await load()
}

async function remove(id: number) {
  await api.deleteToken(id)
  await load()
}

onMounted(load)
</script>

<style scoped>
.token-alert { margin-bottom: 14px; }
.raw-token-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.raw-token-row code {
  font-family: var(--mono);
  word-break: break-all;
}
.provider-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.quota-cell {
  font-variant-numeric: tabular-nums;
  line-height: 1.3;
}
.quota-cell small {
  display: block;
  color: var(--faint);
  font-size: 11px;
  margin-top: 2px;
}
.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
</style>
