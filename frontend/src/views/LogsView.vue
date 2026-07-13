<template>
  <div class="logs-page">
    <div class="page-hd">
      <h1>请求日志</h1>
      <div class="page-actions logs-actions">
        <el-switch v-model="autoRefresh" active-text="自动刷新" />
        <el-button :icon="Refresh" circle :loading="loading" title="刷新" @click="load" />
      </div>
    </div>
    <PageSkeleton v-if="loading && !loaded" type="table" :rows="8" class="logs-skeleton" />
    <el-card v-else class="soft-card logs-card" shadow="never" v-loading="loading">
      <el-table :data="logs" stripe row-key="id" class="logs-table" height="100%">
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
              <el-tag size="small" :type="scope.row.cache_hit ? 'success' : 'info'">{{ scope.row.cache_hit ? '缓存命中' :
                '未命中' }}</el-tag>
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
            <el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ?
              '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="120">
          <template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template>
        </el-table-column>
        <el-table-column label="" width="64" align="right">
          <template #default="scope">
            <el-button link type="primary" :icon="View" title="详情" @click="openDetail(scope.row)" />
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="请求详细日志" width="1180px" top="3vh" class="log-detail-dialog">
      <template v-if="selectedLog">
        <el-tabs v-model="detailTab" class="log-detail-tabs">
          <el-tab-pane label="请求参数" name="params">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="请求 ID">{{ selectedLog.request_id }}</el-descriptions-item>
              <el-descriptions-item label="时间">{{ formatTime(selectedLog.created_at) }}</el-descriptions-item>
              <el-descriptions-item label="模式">{{ modeLabel(selectedLog.mode) }}</el-descriptions-item>
              <el-descriptions-item label="格式">{{ selectedLog.compat_format }}</el-descriptions-item>
              <el-descriptions-item label="状态">{{ selectedLog.status === 'success' ? '成功' : '失败'
                }}</el-descriptions-item>
              <el-descriptions-item label="延迟">{{ formatLatency(selectedLog.latency_ms) }}</el-descriptions-item>
              <el-descriptions-item label="搜索词" :span="2">{{ selectedLog.query }}</el-descriptions-item>
              <el-descriptions-item v-if="selectedLog.error_message" label="错误" :span="2">{{ selectedLog.error_message
                }}</el-descriptions-item>
            </el-descriptions>

            <div class="detail-section">
              <div class="detail-title">请求参数</div>
              <div class="request-param-grid">
                <div v-for="item in requestParams" :key="item.label" class="request-param-item">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                </div>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="请求结果" name="results">
            <div class="detail-section first-section">
              <div class="detail-title">渠道调用</div>
              <el-table :data="providerCallRows" size="small" border row-key="key" max-height="360"
                class="provider-call-table">
                <el-table-column type="expand" width="46">
                  <template #default="scope">
                    <div class="provider-expanded-results">
                      <div v-if="scope.row.results.length" class="result-list provider-result-list">
                        <div v-for="(item, index) in scope.row.results" :key="resultKey(scope.row.key, index, item)"
                          class="result-card">
                          <div class="result-row">
                            <a v-if="item.url" class="result-title-link" :href="item.url" target="_blank"
                              rel="noreferrer">{{ index + 1 }}. {{ item.title || item.url }}</a>
                            <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title ||
                              '无标题' }}</span>
                            <div class="result-row-meta">
                              <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score)
                                }}</span>
                              <el-tag size="small">{{ resultProviderLabel(item, scope.row.provider_name) }}</el-tag>
                              <el-button v-if="hasResultDetails(item)" link class="result-expand-button"
                                :icon="isResultOpen(resultKey(scope.row.key, index, item)) ? ArrowUp : ArrowDown"
                                :title="isResultOpen(resultKey(scope.row.key, index, item)) ? '收起内容' : '展开内容'"
                                @click.stop="toggleResultKey(resultKey(scope.row.key, index, item))" />
                            </div>
                          </div>
                          <div v-if="hasResultDetails(item) && isResultOpen(resultKey(scope.row.key, index, item))"
                            class="result-detail">
                            <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                            <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                          </div>
                        </div>
                      </div>
                      <el-empty v-else :description="callEmptyDescription(scope.row)" />
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="provider_name" label="渠道" min-width="120">
                  <template #default="scope">{{ providerLabel(scope.row.provider_name) }}</template>
                </el-table-column>
                <el-table-column prop="key_alias" label="密钥" min-width="130" />
                <el-table-column prop="attempt_index" label="尝试" width="86">
                  <template #default="scope">第 {{ scope.row.attempt_index || 1 }} 次</template>
                </el-table-column>
                <el-table-column prop="status" label="状态" width="132">
                  <template #default="scope">
                    <div class="call-status-cell">
                      <el-tag :type="callSuccess(scope.row) ? 'success' : 'danger'">{{ callSuccess(scope.row) ? '成功' : '失败'
                        }}</el-tag>
                      <el-tag v-if="scope.row.will_retry" size="small" type="warning">将重试</el-tag>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="result_count" label="结果" width="80" />
                <el-table-column prop="latency_ms" label="延迟" width="100"><template #default="scope">{{
                  formatLatency(scope.row.latency_ms) }}</template></el-table-column>
                <el-table-column prop="cached" label="缓存" width="80"><template #default="scope">{{ scope.row.cached ?
                    '是' : '否' }}</template></el-table-column>
                <el-table-column prop="error_message" label="错误" min-width="220"><template #default="scope">{{
                  scope.row.error_message || '-' }}</template></el-table-column>
              </el-table>
            </div>

            <div class="detail-section">
              <div class="detail-title">{{ showProviderResults ? '合并结果' : '搜索结果' }}</div>
              <div v-if="searchResults.length" class="result-list merged-result-list">
                <div v-for="(item, index) in searchResults" :key="resultKey('merged', index, item)" class="result-card">
                  <div class="result-row">
                    <a v-if="item.url" class="result-title-link" :href="item.url" target="_blank" rel="noreferrer">{{
                      index + 1
                      }}. {{ item.title || item.url }}</a>
                    <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title || '无标题'
                      }}</span>
                    <div class="result-row-meta">
                      <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score) }}</span>
                      <el-tag size="small">{{ resultProviderLabel(item) }}</el-tag>
                      <el-button v-if="hasResultDetails(item)" link class="result-expand-button"
                        :icon="isResultOpen(resultKey('merged', index, item)) ? ArrowUp : ArrowDown"
                        :title="isResultOpen(resultKey('merged', index, item)) ? '收起内容' : '展开内容'"
                        @click.stop="toggleResultKey(resultKey('merged', index, item))" />
                    </div>
                  </div>
                  <div v-if="hasResultDetails(item) && isResultOpen(resultKey('merged', index, item))"
                    class="result-detail">
                    <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                    <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                  </div>
                </div>
              </div>
              <el-empty v-else description="暂无搜索结果" />
            </div>
          </el-tab-pane>
        </el-tabs>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowDown, ArrowUp, Refresh, View } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, ProviderCallLog, SearchLog } from '../api/client'
import { providerLabel } from '../utils/providers'

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

type ProviderResultGroup = {
  provider: string
  key_alias?: string
  status?: string
  error_type?: string
  error?: string
  latency_ms?: number
  result_count?: number
  cached?: boolean
  results?: SearchResultItem[]
}

type ProviderResultGroupView = {
  key: string
  provider: string
  key_alias?: string
  status: string
  error_type?: string
  error?: string
  latency_ms: number
  result_count: number
  cached: boolean
  results: SearchResultItem[]
}

type ProviderCallRow = ProviderCallLog & {
  key: string
  results: SearchResultItem[]
}

type SearchResponseLog = {
  results?: SearchResultItem[]
  provider_results?: ProviderResultGroup[]
  provider_calls?: ProviderCallLog[]
  meta?: Record<string, unknown>
}

const logs = ref<SearchLog[]>([])
const loading = ref(true)
const loaded = ref(false)
const autoRefresh = ref(true)
let refreshTimer: ReturnType<typeof window.setInterval> | undefined
const detailVisible = ref(false)
const detailTab = ref('params')
const selectedLog = ref<SearchLog | null>(null)
const detailCalls = ref<ProviderCallLog[]>([])
const openResultKeys = ref<string[]>([])

const responseLog = computed(() => (selectedLog.value?.response_json || {}) as SearchResponseLog)
const searchResults = computed(() => responseLog.value.results || [])
const providerResultGroups = computed<ProviderResultGroupView[]>(() => {
  const groups = Array.isArray(responseLog.value.provider_results) ? responseLog.value.provider_results : []
  return groups.map((group, index) => {
    const results = Array.isArray(group.results) ? group.results : []
    const provider = group.provider || `provider-${index + 1}`
    return {
      key: `${provider}-${index}`,
      provider,
      key_alias: group.key_alias,
      status: group.status || 'success',
      error_type: group.error_type,
      error: group.error,
      latency_ms: Number(group.latency_ms || 0),
      result_count: Number(group.result_count ?? results.length),
      cached: Boolean(group.cached),
      results
    }
  })
})
const loggedProviderCalls = computed(() => detailCalls.value.length ? detailCalls.value : responseLog.value.provider_calls || [])
const providerCallRows = computed<ProviderCallRow[]>(() => {
  const groupsByProvider = new Map(providerResultGroups.value.map((group) => [group.provider, group]))
  const usedProviders = new Set<string>()
  const rows = loggedProviderCalls.value.map((call, index) => {
    const group = groupsByProvider.get(call.provider_name)
    const status = call.status || group?.status || 'success'
    const hasResults = status === 'success'
    usedProviders.add(call.provider_name)
    return {
      ...call,
      key: providerCallKey(call, index),
      key_alias: call.key_alias || group?.key_alias || '',
      attempt_index: call.attempt_index || 1,
      will_retry: Boolean(call.will_retry),
      status,
      error_type: call.error_type || group?.error_type || '',
      error_message: call.error_message || (hasResults ? '' : group?.error || ''),
      latency_ms: call.latency_ms || group?.latency_ms || 0,
      result_count: call.result_count || (hasResults ? group?.result_count || group?.results.length || 0 : 0),
      cached: call.cached || Boolean(group?.cached),
      results: hasResults ? group?.results || [] : []
    }
  })
  for (const group of providerResultGroups.value) {
    if (usedProviders.has(group.provider)) continue
    rows.push({
      key: group.key,
      provider_key_id: 0,
      provider_name: group.provider,
      key_alias: group.key_alias || '',
      attempt_index: 1,
      will_retry: false,
      status: group.status,
      error_type: group.error_type || '',
      error_message: group.error || '',
      latency_ms: group.latency_ms,
      result_count: group.result_count,
      cached: group.cached,
      results: group.status === 'success' ? group.results : []
    })
  }
  return rows
})
const showProviderResults = computed(() => selectedLog.value?.mode === 'parallel' && providerCallRows.value.length > 1 && providerCallRows.value.some((row) => row.results.length > 0))
const requestParams = computed(() => {
  const request = (selectedLog.value?.request_json || {}) as Record<string, unknown>
  return [
    { label: '搜索词', value: String(request.query || selectedLog.value?.query || '-') },
    { label: '模式', value: modeLabel(String(request.mode || selectedLog.value?.mode || '-')) },
    { label: '渠道', value: Array.isArray(request.providers) ? request.providers.map((item) => providerLabel(String(item))).join('、') : '-' },
    { label: '结果数', value: String(request.limit || selectedLog.value?.result_count || '-') },
    { label: '缓存策略', value: String(request.cache || selectedLog.value?.cache_policy || '-') },
    { label: '去重', value: request.dedupe === false ? '否' : '是' }
  ]
})

function modeLabel(mode: string) {
  return ({ parallel: '并发', fallback: '转移', single: '单平台' } as Record<string, string>)[mode] || mode
}

function resultProviderLabel(item: SearchResultItem, fallback = '未知渠道') {
  if (item.providers?.length) return item.providers.map(providerLabel).join(', ')
  return providerLabel(item.provider || fallback)
}

function formatScore(value: number) {
  if (!Number.isFinite(value)) return '-'
  return Number.isInteger(value) ? String(value) : value.toFixed(2)
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
  const latency = Number(value || 0)
  if (latency >= 1000) return `${(latency / 1000).toFixed(2)}s`
  return `${latency}ms`
}

function callSuccess(call: ProviderCallLog) {
  return call.status === 'success'
}

function providerCallKey(call: ProviderCallLog, index: number) {
  return `${call.provider_name}-${call.provider_key_id || 'no-key'}-${call.attempt_index || 1}-${index}`
}

function callEmptyDescription(call: ProviderCallRow) {
  if (call.error_message) return call.will_retry ? `${call.error_message}，将换 key 重试` : call.error_message
  if (call.will_retry) return '本次失败，已换 key 重试'
  return '该渠道暂无搜索结果'
}

function hasResultDetails(item: SearchResultItem) {
  return Boolean(item.snippet || item.content)
}

function resultKey(prefix: string, index: number, item: SearchResultItem) {
  return `${prefix}-${index}-${item.url || item.title || 'result'}`
}

function isResultOpen(key: string) {
  return openResultKeys.value.includes(key)
}

function toggleResultKey(key: string) {
  if (isResultOpen(key)) {
    openResultKeys.value = openResultKeys.value.filter((item) => item !== key)
    return
  }
  openResultKeys.value = [...openResultKeys.value, key]
}

async function load() {
  loading.value = true
  try {
    logs.value = (await api.logs()).logs
  } finally {
    loaded.value = true
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
  detailTab.value = 'params'
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
.logs-page {
  height: calc(100dvh - 76px);
  max-height: calc(100dvh - 76px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}
.logs-page .page-hd {
  flex: 0 0 auto;
  margin-bottom: 12px;
}
.logs-skeleton {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
.logs-card {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.logs-card :deep(.el-card__body) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding-bottom: 0;
}
.logs-table {
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
}
.logs-actions {
  gap: 12px;
}

.logs-table :deep(.el-table__cell) {
  padding: 12px 0;
}

.log-main-cell {
  min-width: 0;
}

.log-query {
  overflow: hidden;
  color: var(--text);
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-subline {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  color: var(--muted);
  font-size: 12px;
}

.log-request-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.log-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.log-detail-dialog :deep(.el-dialog) {
  max-width: calc(100vw - 32px);
}

.log-detail-dialog :deep(.el-dialog__body) {
  max-height: calc(100vh - 116px);
  overflow: hidden;
}

.log-detail-tabs :deep(.el-tabs__content) {
  max-height: calc(100vh - 220px);
  overflow-y: auto;
  padding-right: 4px;
}

.detail-section {
  margin-top: 16px;
}

.first-section {
  margin-top: 2px;
}

.detail-title {
  margin-bottom: 8px;
  color: var(--text);
  font-weight: 800;
}

.request-param-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid var(--border);
  border-left: 1px solid var(--border);
}

.request-param-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 10px 12px;
  border-right: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

.request-param-item span {
  color: var(--muted);
}

.request-param-item strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
}

.provider-call-table {
  width: 100%;
}

.call-status-cell {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.provider-call-table :deep(.el-table__expanded-cell) {
  padding: 10px 12px;
  background: rgba(47, 148, 97, .04);
}

.provider-expanded-results {
  max-height: 320px;
  overflow-y: auto;
  padding-right: 4px;
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-right: 2px;
}

.provider-result-list {
  padding-bottom: 2px;
}

.merged-result-list {
  max-height: 420px;
  overflow-y: auto;
}

.result-card {
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--el-border-radius-base);
  background: #fff;
}

.result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}

.result-title-link {
  min-width: 0;
  overflow: hidden;
  color: var(--text);
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-decoration: none;
}

.result-title-link:hover {
  color: var(--primary);
}

.result-title-text {
  display: block;
}

.result-row-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.result-score {
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.result-expand-button {
  width: 24px;
  height: 24px;
  min-height: 24px;
  padding: 0;
  color: var(--primary);
}

.result-detail {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.result-snippet {
  margin: 0;
  color: var(--muted);
  line-height: 1.6;
  white-space: pre-wrap;
}

.result-content {
  margin-top: 8px;
  color: var(--text);
}

@media (max-width: 1100px) {
  .request-param-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .result-row {
    align-items: flex-start;
    flex-direction: column;
  }

  .result-row-meta {
    flex-wrap: wrap;
  }
}
</style>
