<template>
  <div class="logs-page">
    <div class="page-hd">
      <div>
        <h1>请求日志</h1>
        <p class="page-sub">活动流 · 卡片时间线 · 渠道结果可展开</p>
      </div>
      <div class="page-actions logs-actions">
        <el-switch v-model="autoRefresh" active-text="自动刷新" />
        <el-button :icon="Refresh" circle :loading="loading" title="刷新" @click="load" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="table" :rows="8" class="logs-skeleton" />
    <template v-else>
      <section class="kpi-row">
        <div class="kpi-card">
          <span>近窗请求</span>
          <b>{{ logs.length }}</b>
        </div>
        <div class="kpi-card">
          <span>成功</span>
          <b class="ok">{{ successCount }}</b>
        </div>
        <div class="kpi-card">
          <span>失败</span>
          <b class="bad">{{ failCount }}</b>
        </div>
        <div class="kpi-card">
          <span>缓存命中</span>
          <b>{{ cacheHitCount }}</b>
        </div>
      </section>

      <section class="filters">
        <el-input v-model="filterQ" clearable placeholder="搜索 query / request_id / 错误" :prefix-icon="Search" />
        <el-select v-model="filterStatus" clearable placeholder="全部状态" style="width: 120px">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-select v-model="filterMode" clearable placeholder="全部模式" style="width: 120px">
          <el-option label="并发" value="parallel" />
          <el-option label="转移" value="fallback" />
          <el-option label="单平台" value="single" />
        </el-select>
        <el-select v-model="filterCache" clearable placeholder="缓存：全部" style="width: 130px">
          <el-option label="命中" value="hit" />
          <el-option label="未命中" value="miss" />
        </el-select>
      </section>

      <div v-loading="loading" class="stream">
        <article
          v-for="row in filteredLogs"
          :key="row.id"
          class="log-card"
          :class="{ fail: row.status !== 'success', active: selectedLog?.id === row.id && drawerVisible }"
          @click="openDetail(row)"
        >
          <div class="rail" />
          <div class="body">
            <div class="q">{{ row.query || '-' }}</div>
            <div class="meta">
              <span>{{ formatTime(row.created_at) }}</span>
              <code>{{ shortRequestId(row.request_id) }}</code>
              <div class="tags">
                <span class="tag" :class="row.status === 'success' ? 'ok' : 'bad'">{{ row.status === 'success' ? '成功' : '失败' }}</span>
                <span class="tag">{{ modeLabel(row.mode) }}</span>
                <span class="tag">{{ row.compat_format || '-' }}</span>
                <span class="tag" :class="{ cache: row.cache_hit }">{{ row.cache_hit ? '缓存命中' : '未命中' }}</span>
              </div>
            </div>
            <div v-if="row.providers?.length" class="providers">
              <span v-for="p in row.providers" :key="p" class="pdot">{{ providerLabel(p) }}</span>
            </div>
            <div v-if="row.error_message" class="err-line">{{ row.error_message }}</div>
          </div>
          <div class="side">
            <div class="lat" :class="latencyClass(row)">{{ formatLatency(row.latency_ms) }}</div>
            <div class="cnt">{{ row.result_count }} 条结果</div>
          </div>
        </article>
        <div v-if="!filteredLogs.length" class="empty muted">暂无匹配日志</div>
      </div>
    </template>

    <Teleport to="body">
      <div v-if="drawerVisible" class="log-mask" @click="closeDrawer" />
      <aside
        class="log-drawer"
        :class="{ open: drawerVisible }"
        role="dialog"
        aria-modal="true"
        @keydown.esc="closeDrawer"
      >
        <template v-if="selectedLog">
          <div class="dhd">
            <div class="dhd-main">
              <h2>{{ selectedLog.query || '请求详情' }}</h2>
              <p>{{ selectedLog.request_id }} · {{ formatTime(selectedLog.created_at) }}</p>
            </div>
            <el-button circle :icon="Close" @click="closeDrawer" />
          </div>

          <div v-if="detailLoading" class="drawer-skel" aria-busy="true" aria-label="加载详情">
            <div class="sk-tabs">
              <span /><span /><span />
            </div>
            <div v-for="n in 4" :key="n" class="sk-call">
              <div class="sk-line w-40" />
              <div class="sk-line w-70" />
            </div>
          </div>

          <el-tabs v-else v-model="detailTab" class="drawer-tabs">
            <el-tab-pane label="请求参数" name="params">
              <div class="kv-grid">
                <div v-for="item in requestParams" :key="item.label" class="kv">
                  <span>{{ item.label }}</span>
                  <b>{{ item.value }}</b>
                </div>
              </div>
              <div v-if="selectedLog.error_message" class="err-box">{{ selectedLog.error_message }}</div>
            </el-tab-pane>

            <el-tab-pane name="calls">
              <template #label>
                渠道调用
                <em v-if="providerCallRows.length" class="tab-count">{{ providerCallRows.length }}</em>
              </template>

              <div v-if="providerCallRows.length" class="call-list">
                <article
                  v-for="call in providerCallRows"
                  :key="call.key"
                  class="call-card"
                  :class="{ fail: !callSuccess(call) }"
                >
                  <div class="call-top" @click="toggleCall(call.key)">
                    <div class="call-title">
                      <strong>{{ providerLabel(call.provider_name) }}</strong>
                      <small>
                        {{ call.key_alias || '—' }}
                        · 第 {{ call.attempt_index || 1 }} 次
                        · {{ formatLatency(call.latency_ms) }}
                        · {{ call.result_count || 0 }} 条
                        <template v-if="call.cached"> · 缓存</template>
                        <template v-if="call.will_retry"> · 将重试</template>
                      </small>
                    </div>
                    <div class="call-actions">
                      <el-tag size="small" :type="callSuccess(call) ? 'success' : 'danger'">
                        {{ callSuccess(call) ? '成功' : '失败' }}
                      </el-tag>
                      <el-button
                        link
                        :icon="isCallOpen(call.key) ? ArrowUp : ArrowDown"
                        :title="isCallOpen(call.key) ? '收起结果' : '展开结果'"
                      />
                    </div>
                  </div>

                  <div v-if="call.error_message" class="call-err">{{ call.error_message }}</div>

                  <div v-if="isCallOpen(call.key)" class="call-results">
                    <div v-if="call.results.length" class="result-list">
                      <div
                        v-for="(item, index) in call.results"
                        :key="resultKey(call.key, index, item)"
                        class="result-card"
                      >
                        <div class="result-row">
                          <a
                            v-if="item.url"
                            class="result-title-link"
                            :href="item.url"
                            target="_blank"
                            rel="noreferrer"
                            @click.stop
                          >{{ index + 1 }}. {{ item.title || item.url }}</a>
                          <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title || '无标题' }}</span>
                          <div class="result-row-meta">
                            <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score) }}</span>
                            <el-button
                              v-if="hasResultDetails(item)"
                              link
                              class="result-expand-button"
                              :icon="isResultOpen(resultKey(call.key, index, item)) ? ArrowUp : ArrowDown"
                              @click.stop="toggleResultKey(resultKey(call.key, index, item))"
                            />
                          </div>
                        </div>
                        <div
                          v-if="hasResultDetails(item) && isResultOpen(resultKey(call.key, index, item))"
                          class="result-detail"
                        >
                          <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                          <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                        </div>
                      </div>
                    </div>
                    <div v-else class="empty-call muted">{{ callEmptyDescription(call) }}</div>
                  </div>
                </article>
              </div>
              <el-empty v-else description="暂无渠道调用记录" :image-size="72" />
            </el-tab-pane>

            <el-tab-pane name="results">
              <template #label>
                {{ showProviderResults ? '合并结果' : '搜索结果' }}
                <em v-if="searchResults.length" class="tab-count">{{ searchResults.length }}</em>
              </template>

              <div v-if="searchResults.length" class="result-list merged">
                <div
                  v-for="(item, index) in searchResults"
                  :key="resultKey('merged', index, item)"
                  class="result-card"
                >
                  <div class="result-row">
                    <a
                      v-if="item.url"
                      class="result-title-link"
                      :href="item.url"
                      target="_blank"
                      rel="noreferrer"
                    >{{ index + 1 }}. {{ item.title || item.url }}</a>
                    <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title || '无标题' }}</span>
                    <div class="result-row-meta">
                      <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score) }}</span>
                      <el-tag size="small">{{ resultProviderLabel(item) }}</el-tag>
                      <el-button
                        v-if="hasResultDetails(item)"
                        link
                        class="result-expand-button"
                        :icon="isResultOpen(resultKey('merged', index, item)) ? ArrowUp : ArrowDown"
                        @click.stop="toggleResultKey(resultKey('merged', index, item))"
                      />
                    </div>
                  </div>
                  <div
                    v-if="hasResultDetails(item) && isResultOpen(resultKey('merged', index, item))"
                    class="result-detail"
                  >
                    <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                    <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                  </div>
                </div>
              </div>
              <el-empty v-else description="暂无搜索结果" :image-size="72" />
            </el-tab-pane>
          </el-tabs>
        </template>
      </aside>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowDown, ArrowUp, Close, Refresh, Search } from '@element-plus/icons-vue'
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
const drawerVisible = ref(false)
const detailLoading = ref(false)
const detailTab = ref('params')
const selectedLog = ref<SearchLog | null>(null)
const detailCalls = ref<ProviderCallLog[]>([])
const openResultKeys = ref<string[]>([])
const openCallKeys = ref<string[]>([])
let detailRequestSeq = 0

const filterQ = ref('')
const filterStatus = ref('')
const filterMode = ref('')
const filterCache = ref('')

const successCount = computed(() => logs.value.filter((item) => item.status === 'success').length)
const failCount = computed(() => logs.value.filter((item) => item.status !== 'success').length)
const cacheHitCount = computed(() => logs.value.filter((item) => item.cache_hit).length)

const filteredLogs = computed(() => {
  const q = filterQ.value.trim().toLowerCase()
  return logs.value.filter((row) => {
    if (filterStatus.value === 'success' && row.status !== 'success') return false
    if (filterStatus.value === 'failed' && row.status === 'success') return false
    if (filterMode.value && row.mode !== filterMode.value) return false
    if (filterCache.value === 'hit' && !row.cache_hit) return false
    if (filterCache.value === 'miss' && row.cache_hit) return false
    if (!q) return true
    return (
      row.query?.toLowerCase().includes(q) ||
      row.request_id?.toLowerCase().includes(q) ||
      row.error_message?.toLowerCase().includes(q)
    )
  })
})

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
    { label: '渠道', value: Array.isArray(request.providers) ? request.providers.map((item) => providerLabel(String(item))).join('、') : (selectedLog.value?.providers || []).map(providerLabel).join('、') || '-' },
    { label: '结果数', value: String(request.limit || selectedLog.value?.result_count || '-') },
    { label: '缓存策略', value: String(request.cache || selectedLog.value?.cache_policy || '-') },
    { label: '去重', value: request.dedupe === false ? '否' : '是' },
    { label: '状态', value: selectedLog.value?.status === 'success' ? '成功' : '失败' },
    { label: '延迟', value: formatLatency(selectedLog.value?.latency_ms || 0) },
    { label: '格式', value: selectedLog.value?.compat_format || '-' },
    { label: 'Request ID', value: selectedLog.value?.request_id || '-' }
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

function latencyClass(row: SearchLog) {
  if (row.status !== 'success') return 'bad'
  if (Number(row.latency_ms || 0) > 2000) return 'slow'
  return ''
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
  if (Number(call.result_count || 0) > 0) return `调用摘要显示 ${call.result_count} 条结果，但正文未写入日志（常见于 seed/旧数据）`
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

function isCallOpen(key: string) {
  return openCallKeys.value.includes(key)
}

function toggleCall(key: string) {
  if (isCallOpen(key)) {
    openCallKeys.value = openCallKeys.value.filter((item) => item !== key)
    return
  }
  openCallKeys.value = [...openCallKeys.value, key]
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
    if (!drawerVisible.value) void load()
  }, 10000)
}

function stopAutoRefresh() {
  if (!refreshTimer) return
  window.clearInterval(refreshTimer)
  refreshTimer = undefined
}

function closeDrawer() {
  drawerVisible.value = false
  detailLoading.value = false
  detailRequestSeq += 1
}

async function openDetail(row: SearchLog) {
  const requestId = ++detailRequestSeq
  selectedLog.value = row
  detailCalls.value = []
  detailTab.value = 'calls'
  openResultKeys.value = []
  openCallKeys.value = []
  drawerVisible.value = true
  detailLoading.value = true
  try {
    const result = await api.logDetail(row.id)
    if (requestId !== detailRequestSeq) return
    selectedLog.value = result.log
    detailCalls.value = result.calls
  } finally {
    if (requestId === detailRequestSeq) detailLoading.value = false
  }
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
  gap: 12px;
}
.page-hd {
  flex: 0 0 auto;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 0;
}
.page-hd h1 { margin: 0; }
.page-sub {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}
.logs-actions { gap: 12px; }
.logs-skeleton {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.kpi-row {
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}
.kpi-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: var(--shadow);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.kpi-card span { color: var(--muted); font-size: 12px; }
.kpi-card b {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: -.03em;
  font-variant-numeric: tabular-nums;
}
.kpi-card b.ok { color: #12b76a; }
.kpi-card b.bad { color: #f04438; }

.filters {
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: var(--shadow);
}
.filters :deep(.el-input) { flex: 1 1 220px; min-width: 180px; }

.stream {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-right: 2px;
  padding-bottom: 8px;
}
.log-card {
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: 6px minmax(0, 1fr) auto;
  gap: 0 14px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: var(--shadow);
  overflow: hidden;
  cursor: pointer;
  transition: border-color .12s ease, transform .12s ease, box-shadow .12s ease;
}
.log-card:hover {
  border-color: #c9d2dc;
  transform: translateY(-1px);
}
.log-card.active {
  border-color: #9fd4be;
  box-shadow: 0 0 0 3px rgba(11, 110, 79, .08), var(--shadow);
}
.rail { background: #12b76a; min-height: 100%; }
.log-card.fail .rail { background: #f04438; }
.body { padding: 14px 0 14px 2px; min-width: 0; }
.q {
  font-weight: 800;
  font-size: 15px;
  letter-spacing: -.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  color: var(--muted);
  font-size: 12px;
  align-items: center;
}
.meta code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  color: #98a2b3;
}
.tags { display: flex; gap: 6px; flex-wrap: wrap; }
.tag {
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 650;
  background: #f2f4f7;
  color: #475467;
  display: inline-flex;
  align-items: center;
}
.tag.ok { background: #ecfdf3; color: #027a48; }
.tag.bad { background: #fef3f2; color: #b42318; }
.tag.cache { background: #e8f6f0; color: #085c42; }
.providers { display: flex; gap: 4px; margin-top: 8px; flex-wrap: wrap; }
.pdot {
  height: 20px;
  padding: 0 7px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  background: #f2f4f7;
  color: #475467;
}
.err-line {
  margin-top: 8px;
  color: #b42318;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.side {
  padding: 14px 16px 14px 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  gap: 6px;
}
.lat {
  font-variant-numeric: tabular-nums;
  font-weight: 800;
  font-size: 16px;
}
.lat.slow { color: #f79009; }
.lat.bad { color: #f04438; }
.cnt { color: var(--muted); font-size: 12px; }
.empty {
  padding: 40px;
  text-align: center;
  background: var(--card);
  border: 1px dashed var(--border);
  border-radius: 16px;
}
.muted { color: var(--muted); }

/* drawer chrome lives in unscoped block below (Teleport -> body) */
.tab-count {
  display: inline-flex;
  min-width: 16px;
  height: 16px;
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 999px;
  background: #e8f6f0;
  color: #085c42;
  font-size: 11px;
  font-style: normal;
  font-weight: 700;
  align-items: center;
  justify-content: center;
}
.kv-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.kv {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px 12px;
  background: #fbfcfd;
}
.kv span {
  display: block;
  color: var(--muted);
  font-size: 11px;
}
.kv b {
  display: block;
  margin-top: 4px;
  font-size: 13px;
  word-break: break-all;
  font-weight: 700;
}
.err-box {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #fef3f2;
  color: #b42318;
  font-size: 13px;
}

.call-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.call-card {
  border: 1px solid var(--border);
  border-radius: 14px;
  background: #fff;
  overflow: hidden;
}
.call-card.fail {
  border-color: #fecdca;
  background: linear-gradient(180deg, #fff8f7, #fff);
}
.call-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 12px;
  cursor: pointer;
}
.call-title strong { display: block; font-size: 14px; }
.call-title small {
  display: block;
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.45;
}
.call-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.call-err {
  margin: 0 12px 10px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #fef3f2;
  color: #b42318;
  font-size: 12px;
}
.call-results {
  border-top: 1px solid var(--border);
  background: rgba(47, 148, 97, .04);
  padding: 10px 12px 12px;
}
.empty-call {
  padding: 10px 4px;
  font-size: 13px;
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.result-list.merged { padding-top: 2px; }
.result-card {
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 10px;
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
.result-title-link:hover { color: var(--primary); }
.result-title-text { display: block; }
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

@media (max-width: 900px) {
  .kpi-row { grid-template-columns: 1fr 1fr; }
  .side { padding-right: 12px; }
  .kv-grid { grid-template-columns: 1fr; }
}
@media (max-width: 720px) {
  .result-row {
    align-items: flex-start;
    flex-direction: column;
  }
  .result-row-meta { flex-wrap: wrap; }
}
</style>

<style>
/* Teleport to body — must be unscoped */
.log-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(15, 20, 25, 0.28);
  backdrop-filter: blur(2px);
}
.log-drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2001;
  width: min(560px, 100vw);
  display: flex;
  flex-direction: column;
  background: #fff;
  border-left: 1px solid var(--border, #e6e8ec);
  box-shadow: -12px 0 40px rgba(16, 24, 40, 0.12);
  transform: translateX(100%);
  transition: transform 0.18s ease;
  pointer-events: none;
  padding: 16px 18px;
  box-sizing: border-box;
}
.log-drawer.open {
  transform: none;
  pointer-events: auto;
}
.log-drawer .dhd {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 0 0 12px;
  border-bottom: 1px solid var(--border, #e6e8ec);
  margin-bottom: 4px;
  flex: 0 0 auto;
}
.log-drawer .dhd h2 {
  margin: 0;
  font-size: 16px;
  line-height: 1.35;
  word-break: break-word;
}
.log-drawer .dhd p {
  margin: 4px 0 0;
  color: var(--muted, #667085);
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}
.log-drawer .drawer-skel {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 4px 20px;
}
.log-drawer .drawer-skel .sk-tabs {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}
.log-drawer .drawer-skel .sk-tabs span {
  width: 72px;
  height: 28px;
  border-radius: 999px;
  background: linear-gradient(90deg, #eef1f4 25%, #f7f8fa 50%, #eef1f4 75%);
  background-size: 200% 100%;
  animation: log-skel 1.2s ease infinite;
}
.log-drawer .drawer-skel .sk-call {
  padding: 14px;
  border: 1px solid var(--border, #e6e8ec);
  border-radius: 14px;
  background: #fbfcfd;
  margin-bottom: 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.log-drawer .drawer-skel .sk-line {
  height: 12px;
  border-radius: 999px;
  background: linear-gradient(90deg, #eef1f4 25%, #f7f8fa 50%, #eef1f4 75%);
  background-size: 200% 100%;
  animation: log-skel 1.2s ease infinite;
}
.log-drawer .drawer-skel .sk-line.w-40 { width: 40%; }
.log-drawer .drawer-skel .sk-line.w-70 { width: 70%; }
@keyframes log-skel {
  0% { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}
.log-drawer .drawer-tabs {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.log-drawer .drawer-tabs .el-tabs__header {
  flex: 0 0 auto;
}
.log-drawer .drawer-tabs .el-tabs__content {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}
.log-drawer .drawer-tabs .el-tab-pane {
  padding-bottom: 16px;
}
.log-drawer .tab-count {
  display: inline-flex;
  min-width: 16px;
  height: 16px;
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 999px;
  background: #e8f6f0;
  color: #085c42;
  font-size: 11px;
  font-style: normal;
  font-weight: 700;
  align-items: center;
  justify-content: center;
}
.log-drawer .kv-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.log-drawer .kv {
  border: 1px solid var(--border, #e6e8ec);
  border-radius: 12px;
  padding: 10px 12px;
  background: #fbfcfd;
}
.log-drawer .kv span {
  display: block;
  color: var(--muted, #667085);
  font-size: 11px;
}
.log-drawer .kv b {
  display: block;
  margin-top: 4px;
  font-size: 13px;
  word-break: break-all;
  font-weight: 700;
}
.log-drawer .err-box {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #fef3f2;
  color: #b42318;
  font-size: 13px;
}
.log-drawer .call-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.log-drawer .call-card {
  border: 1px solid var(--border, #e6e8ec);
  border-radius: 14px;
  background: #fff;
  overflow: hidden;
}
.log-drawer .call-card.fail {
  border-color: #fecdca;
  background: linear-gradient(180deg, #fff8f7, #fff);
}
.log-drawer .call-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 12px;
  cursor: pointer;
}
.log-drawer .call-title strong { display: block; font-size: 14px; }
.log-drawer .call-title small {
  display: block;
  margin-top: 4px;
  color: var(--muted, #667085);
  font-size: 12px;
  line-height: 1.45;
}
.log-drawer .call-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.log-drawer .call-err {
  margin: 0 12px 10px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #fef3f2;
  color: #b42318;
  font-size: 12px;
}
.log-drawer .call-results {
  border-top: 1px solid var(--border, #e6e8ec);
  background: rgba(47, 148, 97, 0.04);
  padding: 10px 12px 12px;
}
.log-drawer .empty-call {
  padding: 10px 4px;
  font-size: 13px;
  color: var(--muted, #667085);
}
.log-drawer .result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.log-drawer .result-card {
  padding: 8px 10px;
  border: 1px solid var(--border, #e6e8ec);
  border-radius: 10px;
  background: #fff;
}
.log-drawer .result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}
.log-drawer .result-title-link {
  min-width: 0;
  overflow: hidden;
  color: var(--text, #0f1419);
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-decoration: none;
}
.log-drawer .result-title-link:hover { color: var(--primary, #0b6e4f); }
.log-drawer .result-row-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.log-drawer .result-score {
  color: var(--muted, #667085);
  font-size: 12px;
  white-space: nowrap;
}
.log-drawer .result-detail {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border, #e6e8ec);
}
.log-drawer .result-snippet {
  margin: 0;
  color: var(--muted, #667085);
  line-height: 1.6;
  white-space: pre-wrap;
}
.log-drawer .result-content {
  margin-top: 8px;
  color: var(--text, #0f1419);
}
@media (max-width: 720px) {
  .log-drawer .kv-grid { grid-template-columns: 1fr; }
  .log-drawer .result-row {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
