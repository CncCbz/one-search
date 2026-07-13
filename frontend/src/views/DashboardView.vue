<template>
  <div class="dashboard-page">
    <div class="page-hd">
      <h1>仪表盘</h1>
      <div class="page-actions">
        <el-button :icon="Refresh" circle title="刷新" :loading="loading" @click="load" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="dashboard" />
    <template v-else>
      <el-row :gutter="12">
        <el-col :xs="12" :sm="8" :md="4" v-for="item in metrics" :key="item.label" class="metric-col">
          <el-card class="metric-card" shadow="never">
            <div class="muted">{{ item.label }}</div>
            <div class="metric-value">{{ item.value }}</div>
          </el-card>
        </el-col>
      </el-row>

      <div class="dashboard-grid">
        <el-card class="soft-card dashboard-card" shadow="never" v-loading="loading">
          <template #header>
            <div class="card-hd"><span>平台健康</span><span class="muted">最近窗口</span></div>
          </template>
          <el-table :data="providerRows" stripe height="100%">
            <el-table-column prop="display_name" label="平台" min-width="120" />
            <el-table-column label="状态" width="100">
              <template #default="scope">
                <el-tag :type="healthTagType(scope.row.health_status)">{{ healthLabel(scope.row.health_status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="密钥" width="90" align="right">
              <template #default="scope">{{ scope.row.available_keys }}/{{ scope.row.total_keys || scope.row.available_keys || 0 }}</template>
            </el-table-column>
            <el-table-column label="成功率" width="90" align="right">
              <template #default="scope">{{ formatPercent(scope.row.success_rate) }}</template>
            </el-table-column>
            <el-table-column prop="requests_failed" label="失败" width="80" align="right" />
            <el-table-column prop="timeout_ms" label="超时" width="90" align="right" />
          </el-table>
        </el-card>

        <el-card class="soft-card dashboard-card" shadow="never" v-loading="loading">
          <template #header>
            <div class="card-hd"><span>用量</span><span class="muted">{{ billing.days || 30 }} 天</span></div>
          </template>
          <el-table :data="billing.units" stripe height="100%">
            <el-table-column prop="provider_name" label="平台">
              <template #default="scope">{{ providerLabel(scope.row.provider_name) }}</template>
            </el-table-column>
            <el-table-column prop="unit" label="单位">
              <template #default="scope">{{ usageUnitLabel(scope.row.unit) }}</template>
            </el-table-column>
            <el-table-column label="数量" align="right">
              <template #default="scope">{{ formatNumber(scope.row.quantity_total) }}</template>
            </el-table-column>
            <el-table-column label="USD" align="right">
              <template #default="scope">{{ formatCurrency(scope.row.cost_usd_total) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, BillingSummary, ProviderConfig, ProviderHealth, UsageSummary } from '../api/client'
import { providerLabel } from '../utils/providers'

const loading = ref(true)
const loaded = ref(false)
const usage = ref<UsageSummary>({
  requests_total: 0,
  requests_success: 0,
  requests_failed: 0,
  cache_hits: 0,
  results_total: 0,
  average_latency_ms: 0
})
const providers = ref<ProviderConfig[]>([])
const providerHealth = ref<ProviderHealth[]>([])
const billing = ref<BillingSummary>({ days: 30, units: [] })

const metrics = computed(() => [
  { label: '总请求', value: usage.value.requests_total },
  { label: '成功', value: usage.value.requests_success },
  { label: '失败', value: usage.value.requests_failed },
  { label: '结果', value: usage.value.results_total },
  { label: '缓存命中', value: usage.value.cache_hits },
  { label: '平均延迟', value: `${usage.value.average_latency_ms.toFixed(1)}ms` }
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
  loading.value = true
  try {
    const result = await api.dashboard()
    usage.value = result.usage
    providers.value = result.providers
    providerHealth.value = result.provider_health || []
    billing.value = result.billing || { days: 30, units: [] }
    loaded.value = true
  } finally {
    loading.value = false
  }
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
  const labels: Record<string, string> = {
    healthy: '健康',
    degraded: '降级',
    down: '不可用',
    disabled: '停用',
    no_keys: '无密钥',
    unknown: '未知'
  }
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
.dashboard-page { display: flex; flex-direction: column; gap: 14px; }
.metric-col { margin-bottom: 0; }
.metric-value {
  margin-top: 8px;
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
}
.dashboard-grid {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: 14px;
  min-height: 420px;
}
.dashboard-card {
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.dashboard-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.card-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
@media (max-width: 980px) {
  .dashboard-grid { grid-template-columns: 1fr; }
}
</style>
