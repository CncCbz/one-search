<template>
  <div class="dashboard-page">
    <div class="page-actions">
      <el-button @click="load">刷新</el-button>
    </div>
    <el-row :gutter="16">
      <el-col :span="8" v-for="item in metrics" :key="item.label" class="metric-col">
        <el-card class="metric-card" shadow="never"><div class="muted">{{ item.label }}</div><h2>{{ item.value }}</h2></el-card>
      </el-col>
    </el-row>
    <el-card class="soft-card dashboard-card" shadow="never">
      <template #header>平台健康概览</template>
      <el-table :data="providerRows" stripe height="100%">
        <el-table-column prop="display_name" label="平台" />
        <el-table-column label="健康状态" width="120"><template #default="scope"><el-tag :type="healthTagType(scope.row.health_status)">{{ healthLabel(scope.row.health_status) }}</el-tag></template></el-table-column>
        <el-table-column label="可用密钥" width="110"><template #default="scope">{{ scope.row.available_keys }}/{{ scope.row.total_keys || scope.row.available_keys || 0 }}</template></el-table-column>
        <el-table-column label="最近成功率" width="120"><template #default="scope">{{ formatPercent(scope.row.success_rate) }}</template></el-table-column>
        <el-table-column prop="requests_failed" label="最近失败" width="100" />
        <el-table-column prop="timeout_ms" label="超时(ms)" width="110" />
        <el-table-column prop="cache_ttl_seconds" label="缓存(s)" width="100" />
      </el-table>
    </el-card>

    <el-card class="soft-card dashboard-card" shadow="never">
      <template #header>用量计量</template>
      <el-table :data="billing.units" stripe height="100%">
        <el-table-column prop="provider_name" label="平台"><template #default="scope">{{ providerLabel(scope.row.provider_name) }}</template></el-table-column>
        <el-table-column prop="unit" label="单位"><template #default="scope">{{ usageUnitLabel(scope.row.unit) }}</template></el-table-column>
        <el-table-column label="数量"><template #default="scope">{{ formatNumber(scope.row.quantity_total) }}</template></el-table-column>
        <el-table-column label="USD"><template #default="scope">{{ formatCurrency(scope.row.cost_usd_total) }}</template></el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, BillingSummary, ProviderConfig, ProviderHealth, UsageSummary } from '../api/client'
import { providerLabel } from '../utils/providers'

const usage = ref<UsageSummary>({ requests_total: 0, requests_success: 0, requests_failed: 0, cache_hits: 0, results_total: 0, average_latency_ms: 0 })
const providers = ref<ProviderConfig[]>([])
const providerHealth = ref<ProviderHealth[]>([])
const billing = ref<BillingSummary>({ days: 30, units: [] })
const metrics = computed(() => [
  { label: '总请求', value: usage.value.requests_total },
  { label: '成功请求', value: usage.value.requests_success },
  { label: '失败请求', value: usage.value.requests_failed },
  { label: '结果总数', value: usage.value.results_total },
  { label: '缓存命中', value: usage.value.cache_hits },
  { label: '平均延迟(ms)', value: usage.value.average_latency_ms.toFixed(1) }
])
const providerRows = computed(() => providers.value.map((provider) => {
  const health = providerHealth.value.find((item) => item.provider_name === provider.name)
  return {
    ...provider,
    health_status: health?.status || (provider.enabled ? 'unknown' : 'disabled'),
    total_keys: health?.total_keys || provider.available_keys || 0,
    available_keys: health?.available_keys ?? provider.available_keys ?? 0,
    success_rate: health?.success_rate ?? 0,
    requests_failed: health?.requests_failed ?? 0
  }
}))
async function load() {
  const result = await api.dashboard()
  usage.value = result.usage
  providers.value = result.providers
  providerHealth.value = result.provider_health || []
  billing.value = result.billing || { days: 30, units: [] }
}
function formatPercent(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `${Math.round(value * 100)}%`
}

function usageUnitLabel(unit: string) {
  return ({ requests: '请求', credits: 'Credits', tokens: 'Tokens', usd: 'USD' } as Record<string, string>)[unit] || unit
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value || 0)
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value || 0)
}

function healthLabel(status: string) {
  const labels: Record<string, string> = { healthy: '健康', degraded: '降级', down: '不可用', disabled: '停用', no_keys: '无密钥', unknown: '未知' }
  return labels[status] || status
}

function healthTagType(status: string) {
  if (status === 'healthy') return 'success'
  if (status === 'degraded') return 'warning'
  if (status === 'down') return 'danger'
  return 'info'
}

onMounted(load)
</script>

<style scoped>
.dashboard-page { height: calc(100vh - 56px); display: flex; flex-direction: column; overflow: hidden; }
.metric-col { margin-bottom: 16px; }
.dashboard-card { flex: 1 1 0; min-height: 0; margin-top: 16px; display: flex; flex-direction: column; }
.dashboard-card :deep(.el-card__body) { flex: 1; min-height: 0; overflow: hidden; }
</style>
