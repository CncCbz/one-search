<template>
  <div>
    <div class="page-actions">
      <el-button type="primary" @click="dialog = true">新增令牌</el-button>
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
        <el-table-column prop="status" label="状态">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'enabled' ? 'success' : 'info'">{{ scope.row.status === 'enabled' ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rate_limit_per_min" label="RPM" />
        <el-table-column prop="daily_quota" label="日额度" />
        <el-table-column prop="usage_count" label="使用次数" />
        <el-table-column label="操作" width="180">
          <template #default="scope">
            <el-button link @click="setStatus(scope.row.id, scope.row.status === 'enabled' ? 'disabled' : 'enabled')">{{ scope.row.status === 'enabled' ? '停用' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(scope.row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialog" title="新增接口令牌">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="每分钟限制（0 表示不限）"><el-input-number v-model="form.rate_limit_per_min" :min="0" /></el-form-item>
        <el-form-item label="日额度（0 表示不限）"><el-input-number v-model="form.daily_quota" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="create">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, ApiToken } from '../api/client'

const tokens = ref<ApiToken[]>([])
const dialog = ref(false)
const rawToken = ref('')
const form = reactive({ name: '默认客户端', scopes: ['search'], rate_limit_per_min: 0, daily_quota: 0 })

async function load() {
  tokens.value = (await api.tokens()).tokens
}

function displayToken(token: ApiToken) {
  return token.token || `${token.token_prefix}...`
}

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function create() {
  const result = await api.createToken(form)
  rawToken.value = result.raw_token
  dialog.value = false
  await load()
}

async function setStatus(id: number, status: string) {
  await api.updateToken(id, status)
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
</style>
