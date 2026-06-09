<template>
  <div>
    <div class="page-actions">
      <el-button @click="load">刷新</el-button>
    </div>
    <el-row :gutter="16">
      <el-col :span="6" v-for="item in metrics" :key="item.label">
        <el-card class="metric-card" shadow="never"><div class="muted">{{ item.label }}</div><h2>{{ item.value }}</h2></el-card>
      </el-col>
    </el-row>
    <el-card class="soft-card" shadow="never" style="margin-top:16px">
      <template #header>平台健康概览</template>
      <el-table :data="providers" stripe>
        <el-table-column prop="display_name" label="平台" />
        <el-table-column prop="enabled" label="状态"><template #default="scope"><el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ scope.row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column prop="available_keys" label="可用密钥" />
        <el-table-column prop="timeout_ms" label="超时(ms)" />
        <el-table-column prop="cache_ttl_seconds" label="缓存时长(s)" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, ProviderConfig, UsageSummary } from '../api/client'

const usage = ref<UsageSummary>({ requests_total: 0, requests_success: 0, requests_failed: 0, cache_hits: 0, results_total: 0, average_latency_ms: 0 })
const providers = ref<ProviderConfig[]>([])
const metrics = computed(() => [
  { label: '总请求', value: usage.value.requests_total },
  { label: '成功请求', value: usage.value.requests_success },
  { label: '缓存命中', value: usage.value.cache_hits },
  { label: '平均延迟(ms)', value: usage.value.average_latency_ms.toFixed(1) }
])
async function load() {
  const result = await api.dashboard()
  usage.value = result.usage
  providers.value = result.providers
}
onMounted(load)
</script>
