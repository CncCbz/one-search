<template>
  <div>
    <div class="page-actions">
      <el-button type="primary" @click="openCreate">新增令牌</el-button>
    </div>

    <el-alert v-if="rawToken" type="success" show-icon :closable="false" style="margin-bottom:16px">
      <template #title>
        <span>新令牌只显示一次：</span>
        <code>{{ rawToken }}</code>
        <el-button link type="primary" @click="copyText(rawToken)">复制</el-button>
      </template>
    </el-alert>

    <el-card class="soft-card" shadow="never">
      <el-table :data="tokens" stripe>
        <el-table-column prop="name" label="名称" />
        <el-table-column label="令牌">
          <template #default="scope">
            <div class="token-cell">
              <span>{{ displayToken(scope.row) }}</span>
              <el-button link type="primary" @click="copyText(scope.row.token || scope.row.token_prefix)">复制</el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="允许渠道" width="220">
          <template #default="scope">
            <div class="provider-tags">
              <el-tag v-if="scope.row.allowed_providers.length === 0" size="small" type="info">全部渠道</el-tag>
              <el-tag v-for="provider in scope.row.allowed_providers" :key="provider" size="small">{{ providerLabel(provider) }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'enabled' ? 'success' : 'info'">{{ scope.row.status === 'enabled' ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rate_limit_per_min" label="RPM" width="90" />
        <el-table-column prop="daily_quota" label="日额度" width="100" />
        <el-table-column prop="usage_count" label="使用次数" width="100" />
        <el-table-column label="操作" width="220">
          <template #default="scope">
            <el-button link type="primary" @click="openEdit(scope.row)">编辑</el-button>
            <el-button link @click="setStatus(scope.row, scope.row.status === 'enabled' ? 'disabled' : 'enabled')">{{ scope.row.status === 'enabled' ? '停用' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(scope.row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialog" :title="editingToken ? '编辑接口令牌' : '新增接口令牌'">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="允许请求渠道">
          <el-select v-model="form.allowed_providers" multiple collapse-tags collapse-tags-tooltip placeholder="不选择表示全部渠道">
            <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="每分钟限制（0 表示不限）"><el-input-number v-model="form.rate_limit_per_min" :min="0" /></el-form-item>
        <el-form-item label="日额度（0 表示不限）"><el-input-number v-model="form.daily_quota" :min="0" /></el-form-item>
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
import { ElMessage } from 'element-plus'
import { api, ApiToken } from '../api/client'

const providerOptions = [
  { label: 'Exa', value: 'exa' },
  { label: 'You.com', value: 'you' },
  { label: 'Jina', value: 'jina' }
]
const tokens = ref<ApiToken[]>([])
const dialog = ref(false)
const rawToken = ref('')
const editingToken = ref<ApiToken | null>(null)
const form = reactive({ name: '默认客户端', scopes: ['search'], allowed_providers: [] as string[], rate_limit_per_min: 0, daily_quota: 0 })

async function load() {
  tokens.value = (await api.tokens()).tokens
}

function providerLabel(provider: string) {
  return providerOptions.find((item) => item.value === provider)?.label || provider
}

function displayToken(token: ApiToken) {
  return token.token || `${token.token_prefix}...`
}

function resetForm() {
  form.name = '默认客户端'
  form.scopes = ['search']
  form.allowed_providers = []
  form.rate_limit_per_min = 0
  form.daily_quota = 0
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
.token-cell { display: flex; align-items: center; gap: 8px; min-width: 0; }
.token-cell span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.provider-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
</style>
