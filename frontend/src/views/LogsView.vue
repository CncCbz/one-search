<template>
  <div>
    <div class="page-actions logs-actions">
      <el-switch v-model="autoRefresh" active-text="自动刷新" />
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>
    <el-card class="soft-card logs-card" shadow="never">
      <el-table :data="logs" stripe row-key="id" class="logs-table" height="calc(100vh - 150px)">
        <el-table-column label="请求" min-width="360">
          <template #default="scope">
            <div class="log-main-cell">
              <div class="log-query">{{ scope.row.query || '-' }}</div>
              <div class="log-subline">
                <span>{{ formatTime(scope.row.created_at) }}</span>
                <span class="log-request-id">{{ shortRequestId(scope.row.request_id) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="220">
          <template #default="scope">
            <div class="log-tags">
              <el-tag size="small" type="info">{{ modeLabel(scope.row.mode) }}</el-tag>
              <el-tag size="small">{{ scope.row.compat_format }}</el-tag>
              <el-tag size="small" :type="scope.row.cache_hit ? 'success' : 'info'">{{ scope.row.cache_hit ? '缓存命中' : '未命中' }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="结果" width="110">
          <template #default="scope">
            <strong>{{ scope.row.result_count }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="120">
          <template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" align="right">
          <template #default="scope"><el-button link type="primary" @click="openDetail(scope.row)">详情</el-button></template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="请求详细日志" width="920px" class="log-detail-dialog">
      <template v-if="selectedLog">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="请求 ID">{{ selectedLog.request_id }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ formatTime(selectedLog.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="模式">{{ modeLabel(selectedLog.mode) }}</el-descriptions-item>
          <el-descriptions-item label="格式">{{ selectedLog.compat_format }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ selectedLog.status === 'success' ? '成功' : '失败' }}</el-descriptions-item>
          <el-descriptions-item label="延迟">{{ formatLatency(selectedLog.latency_ms) }}</el-descriptions-item>
          <el-descriptions-item label="搜索词" :span="2">{{ selectedLog.query }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedLog.error_message" label="错误" :span="2">{{ selectedLog.error_message }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-section">
          <div class="detail-title">渠道调用</div>
          <el-table :data="detailCalls" size="small" border>
            <el-table-column prop="provider_name" label="渠道" />
            <el-table-column prop="key_alias" label="密钥" />
            <el-table-column prop="status" label="状态" width="90"><template #default="scope"><el-tag :type="callSuccess(scope.row) ? 'success' : 'danger'">{{ callSuccess(scope.row) ? '成功' : '失败' }}</el-tag></template></el-table-column>
            <el-table-column prop="result_count" label="结果" width="80" />
            <el-table-column prop="latency_ms" label="延迟" width="100"><template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template></el-table-column>
            <el-table-column prop="cached" label="缓存" width="80"><template #default="scope">{{ scope.row.cached ? '是' : '否' }}</template></el-table-column>
            <el-table-column prop="error_message" label="错误" />
          </el-table>
        </div>

        <div class="detail-section">
          <div class="detail-title">请求参数</div>
          <div class="request-param-grid">
            <div v-for="item in requestParams" :key="item.label" class="request-param-item">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </div>

        <div class="detail-section">
          <div class="detail-title">搜索结果</div>
          <div v-if="searchResults.length" class="result-list">
            <div v-for="(item, index) in searchResults" :key="index" class="result-card">
              <div class="result-card-header">
                <div class="result-title">{{ index + 1 }}. {{ item.title || '无标题' }}</div>
                <el-tag size="small">{{ item.provider || item.providers?.join(', ') || '未知渠道' }}</el-tag>
              </div>
              <a v-if="item.url" class="result-url" :href="item.url" target="_blank" rel="noreferrer">{{ item.url }}</a>
              <div class="result-meta">
                <span v-if="item.score !== undefined">评分 {{ item.score }}</span>
                <span v-if="item.published_at">发布时间 {{ item.published_at }}</span>
              </div>
              <el-collapse v-if="item.snippet || item.content" v-model="openResultKeys" class="result-collapse">
                <el-collapse-item :name="String(index)">
                  <template #title>查看内容</template>
                  <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                  <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                </el-collapse-item>
              </el-collapse>
            </div>
          </div>
          <el-empty v-else description="暂无搜索结果" />
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, ProviderCallLog, SearchLog } from '../api/client'

type SearchResultItem = {
  title?: string
  url?: string
  snippet?: string
  content?: string
  provider?: string
  providers?: string[]
  score?: number
  published_at?: string
}

type SearchResponseLog = {
  results?: SearchResultItem[]
  meta?: Record<string, unknown>
}

const logs = ref<SearchLog[]>([])
const loading = ref(false)
const autoRefresh = ref(true)
let refreshTimer: ReturnType<typeof window.setInterval> | undefined
const detailVisible = ref(false)
const selectedLog = ref<SearchLog | null>(null)
const detailCalls = ref<ProviderCallLog[]>([])
const openResultKeys = ref<string[]>([])

const responseLog = computed(() => (selectedLog.value?.response_json || {}) as SearchResponseLog)
const searchResults = computed(() => responseLog.value.results || [])
const requestParams = computed(() => {
  const request = (selectedLog.value?.request_json || {}) as Record<string, unknown>
  return [
    { label: '搜索词', value: String(request.query || selectedLog.value?.query || '-') },
    { label: '模式', value: modeLabel(String(request.mode || selectedLog.value?.mode || '-')) },
    { label: '渠道', value: Array.isArray(request.providers) ? request.providers.join('、') : '-' },
    { label: '结果数', value: String(request.limit || selectedLog.value?.result_count || '-') },
    { label: '缓存策略', value: String(request.cache || selectedLog.value?.cache_policy || '-') },
    { label: '去重', value: request.dedupe === false ? '否' : '是' }
  ]
})

function modeLabel(mode: string) {
  return ({ parallel: '并发', fallback: '转移', single: '单平台' } as Record<string, string>)[mode] || mode
}

function formatTime(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function shortRequestId(id: string) {
  if (!id) return '-'
  return id.length > 14 ? `${id.slice(0, 8)}…${id.slice(-6)}` : id
}

function formatLatency(value: number) {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}s`
  return `${value}ms`
}

function callSuccess(call: ProviderCallLog) {
  return call.status === 'success'
}

async function load() {
  loading.value = true
  try {
    logs.value = (await api.logs()).logs
  } finally {
    loading.value = false
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  if (!autoRefresh.value) return
  refreshTimer = window.setInterval(() => {
    if (!detailVisible.value) void load()
  }, 10000)
}

function stopAutoRefresh() {
  if (!refreshTimer) return
  window.clearInterval(refreshTimer)
  refreshTimer = undefined
}

async function openDetail(row: SearchLog) {
  const result = await api.logDetail(row.id)
  selectedLog.value = result.log
  detailCalls.value = result.calls
  openResultKeys.value = []
  detailVisible.value = true
}

watch(autoRefresh, startAutoRefresh)

onMounted(() => {
  void load()
  startAutoRefresh()
})

onBeforeUnmount(stopAutoRefresh)
</script>

<style scoped>
.logs-actions { gap: 12px; }
.logs-card :deep(.el-card__body) { padding-bottom: 0; }
.logs-table :deep(.el-table__cell) { padding: 12px 0; }
.log-main-cell { min-width: 0; }
.log-query { overflow: hidden; color: var(--text); font-weight: 800; text-overflow: ellipsis; white-space: nowrap; }
.log-subline { display: flex; align-items: center; gap: 10px; margin-top: 6px; color: var(--muted); font-size: 12px; }
.log-request-id { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.log-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.detail-section { margin-top: 16px; }
.detail-title { margin-bottom: 8px; color: var(--text); font-weight: 800; }
.request-param-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-top: 1px solid var(--border); border-left: 1px solid var(--border); }
.request-param-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 44px; padding: 10px 12px; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.request-param-item span { color: var(--muted); }
.request-param-item strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }
.result-list { display: flex; flex-direction: column; gap: 10px; max-height: 420px; overflow-y: auto; padding-right: 2px; }
.result-card { padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: #fff; }
.result-card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.result-title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); font-weight: 800; }
.result-url { display: block; margin-top: 6px; overflow: hidden; color: var(--primary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; text-decoration: none; }
.result-snippet { margin: 0; color: var(--muted); line-height: 1.6; white-space: pre-wrap; }
.result-content { margin-top: 8px; color: var(--text); }
.result-meta { display: flex; gap: 12px; margin-top: 8px; color: var(--muted); font-size: 12px; }
.result-collapse { margin-top: 8px; border-top: 1px solid var(--border); border-bottom: 0; }
.result-collapse :deep(.el-collapse-item__header) { height: 36px; border-bottom: 0; color: var(--primary); font-weight: 700; }
.result-collapse :deep(.el-collapse-item__wrap) { border-bottom: 0; }
.result-collapse :deep(.el-collapse-item__content) { padding-bottom: 0; }
@media (max-width: 1100px) { .request-param-grid { grid-template-columns: 1fr; } }
</style>
