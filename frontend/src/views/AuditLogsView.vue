<template>
  <div class="audit-page">
    <div class="page-hd">
      <h1>审计日志</h1>
      <div class="page-actions">
        <el-button :icon="Refresh" circle title="刷新" :loading="loading" @click="load" />
      </div>
    </div>
    <PageSkeleton v-if="loading && !loaded" type="table" :rows="8" class="audit-skeleton" />
    <el-card v-else class="soft-card audit-card" shadow="never" v-loading="loading">
      <el-table :data="logs" stripe class="audit-table" height="100%">
        <el-table-column label="时间" width="180">
          <template #default="scope">{{ formatTime(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="actor" label="操作者" width="120" />
        <el-table-column prop="action" label="动作" width="180" />
        <el-table-column label="对象" width="190">
          <template #default="scope">
            <span>{{ scope.row.resource_type || '-' }}</span>
            <span v-if="scope.row.resource_id" class="muted-id">#{{ scope.row.resource_id }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="request_id" label="请求 ID" width="220" />
        <el-table-column label="元数据">
          <template #default="scope"><code>{{ compactMetadata(scope.row.metadata) }}</code></template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, AuditLog } from '../api/client'

const loading = ref(true)
const loaded = ref(false)
const logs = ref<AuditLog[]>([])

async function load() {
  loading.value = true
  try {
    logs.value = (await api.auditLogs()).logs
    loaded.value = true
  } finally {
    loading.value = false
  }
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function compactMetadata(value: Record<string, unknown>) {
  const text = JSON.stringify(value || {})
  return text.length > 160 ? `${text.slice(0, 157)}...` : text
}

onMounted(load)
</script>

<style scoped>
.audit-page {
  height: calc(100dvh - 76px);
  max-height: calc(100dvh - 76px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}
.audit-page .page-hd {
  flex: 0 0 auto;
  margin-bottom: 12px;
}
.audit-skeleton {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
.audit-card {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.audit-card :deep(.el-card__body) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.audit-table {
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
}
.muted-id { margin-left: 6px; color: var(--muted); }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; white-space: normal; word-break: break-all; }
</style>
