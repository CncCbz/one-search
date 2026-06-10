<template>
  <div>
    <div class="page-actions">
      <el-button @click="load">刷新</el-button>
    </div>
    <el-card class="soft-card" shadow="never">
      <el-table :data="logs" stripe>
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
import { api, AuditLog } from '../api/client'

const logs = ref<AuditLog[]>([])

async function load() {
  logs.value = (await api.auditLogs()).logs
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
.muted-id { margin-left: 6px; color: var(--muted); }
code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; white-space: normal; word-break: break-all; }
</style>
