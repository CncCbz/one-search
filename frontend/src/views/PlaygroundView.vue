<template>
  <div class="playground-page">
    <div class="playground-head">
      <h1>搜索调试</h1>
      <el-tag :type="ready ? 'success' : 'warning'" effect="light" round>
        {{ ready ? '可用' : '待配置' }}
      </el-tag>
    </div>

    <PageSkeleton v-if="loadingMeta" type="playground" />
    <template v-else>
    <div v-if="!ready" class="setup-banner">
      <div>
        <p>还没有可用的搜索平台</p>
        <span class="sub">启用 1 个平台并添加上游 Key 后再搜索</span>
      </div>
      <el-button type="primary" @click="$router.push('/providers')">去配置</el-button>
    </div>

    <el-card class="soft-card search-shell" :class="{ blocked: !ready }" shadow="never">
      <div class="search-row">
        <el-icon class="search-icon" :size="18"><Search /></el-icon>
        <el-input
          v-model="form.query"
          class="search-input"
          placeholder="输入搜索词"
          :disabled="!ready"
          @keyup.enter="run"
        />
        <el-button type="primary" :loading="loading" :disabled="!ready || !form.query.trim()" @click="run">
          搜索
        </el-button>
      </div>

      <div class="search-tools">
        <el-radio-group v-model="form.mode" size="small" :disabled="!ready">
          <el-radio-button value="parallel">并发</el-radio-button>
          <el-radio-button value="fallback">转移</el-radio-button>
          <el-radio-button value="single">单源</el-radio-button>
        </el-radio-group>

        <el-select
          v-model="form.providers"
          class="provider-select"
          multiple
          collapse-tags
          collapse-tags-tooltip
          placeholder="搜索平台"
          :disabled="!ready"
        >
          <el-option
            v-for="item in availableProviderOptions"
            :key="item.value"
            :value="item.value"
            :label="item.label"
          />
        </el-select>

        <div class="limit-field">
          <span>条数</span>
          <el-input-number v-model="form.limit" :min="1" :max="50" controls-position="right" :disabled="!ready" />
        </div>
      </div>
    </el-card>

    </template>

    <el-card v-if="result" class="soft-card result-card" shadow="never">
      <template #header>
        <div class="result-header">
          <div>
            <strong>搜索结果</strong>
            <span class="muted result-meta">
              {{ result.meta?.total_results || result.results.length }} 条 · 去重 {{ result.meta?.deduped_results || 0 }} ·
              {{ formatLatency(result.meta?.latency_ms || 0) }}
            </span>
          </div>
          <el-tag size="small" type="info" effect="plain">{{ result.meta?.request_id || '-' }}</el-tag>
        </div>
      </template>

      <div class="section-block">
        <div class="section-title">渠道调用</div>
        <el-table :data="providerRows" size="small" border row-key="key" max-height="360" class="provider-call-table">
          <el-table-column type="expand" width="46">
            <template #default="scope">
              <div class="provider-expanded-results">
                <div v-if="scope.row.results.length" class="search-result-list provider-result-list">
                  <div v-for="(item, index) in scope.row.results" :key="resultKey(scope.row.key, index, item)" class="search-result-card">
                    <div class="result-row">
                      <a v-if="item.url" class="result-title-link" :href="item.url" target="_blank" rel="noreferrer">{{ index + 1 }}. {{ item.title || item.url }}</a>
                      <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title || '无标题' }}</span>
                      <div class="result-row-meta">
                        <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score) }}</span>
                        <el-tag size="small">{{ resultProviderLabel(item, scope.row.provider) }}</el-tag>
                        <el-button
                          v-if="hasResultDetails(item)"
                          link
                          class="result-expand-button"
                          :icon="isResultOpen(resultKey(scope.row.key, index, item)) ? ArrowUp : ArrowDown"
                          :title="isResultOpen(resultKey(scope.row.key, index, item)) ? '收起内容' : '展开内容'"
                          @click.stop="toggleResultKey(resultKey(scope.row.key, index, item))"
                        />
                      </div>
                    </div>
                    <div v-if="hasResultDetails(item) && isResultOpen(resultKey(scope.row.key, index, item))" class="result-detail">
                      <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                      <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                    </div>
                  </div>
                </div>
                <el-empty v-else :description="callEmptyDescription(scope.row)" />
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="provider" label="渠道"><template #default="scope">{{ providerLabel(scope.row.provider) }}</template></el-table-column>
          <el-table-column prop="key_alias" label="密钥" />
          <el-table-column prop="attempt_index" label="尝试" width="86"><template #default="scope">第 {{ scope.row.attempt_index || 1 }} 次</template></el-table-column>
          <el-table-column prop="status" label="状态" width="132">
            <template #default="scope">
              <div class="call-status-cell">
                <el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ? '成功' : '失败' }}</el-tag>
                <el-tag v-if="scope.row.will_retry" size="small" type="warning">将重试</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="result_count" label="结果" width="80" align="right" />
          <el-table-column prop="latency_ms" label="延迟" width="100" align="right"><template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template></el-table-column>
          <el-table-column prop="error" label="错误"><template #default="scope">{{ scope.row.error || '-' }}</template></el-table-column>
        </el-table>
      </div>

      <div class="section-block">
        <div class="section-title">{{ hasProviderResults ? '合并结果' : '结果列表' }}</div>
        <div v-if="result.results.length" class="search-result-list merged-result-list">
          <div v-for="(item, index) in result.results" :key="resultKey('merged', index, item)" class="search-result-card">
            <div class="result-row">
              <a v-if="item.url" class="result-title-link" :href="item.url" target="_blank" rel="noreferrer">{{ index + 1 }}. {{ item.title || item.url }}</a>
              <span v-else class="result-title-link result-title-text">{{ index + 1 }}. {{ item.title || '无标题' }}</span>
              <div class="result-row-meta">
                <span v-if="item.score !== undefined" class="result-score">评分 {{ formatScore(item.score) }}</span>
                <el-tag size="small">{{ resultProviderLabel(item) }}</el-tag>
                <el-button
                  v-if="hasResultDetails(item)"
                  link
                  class="result-expand-button"
                  :icon="isResultOpen(resultKey('merged', index, item)) ? ArrowUp : ArrowDown"
                  :title="isResultOpen(resultKey('merged', index, item)) ? '收起内容' : '展开内容'"
                  @click.stop="toggleResultKey(resultKey('merged', index, item))"
                />
              </div>
            </div>
            <div v-if="hasResultDetails(item) && isResultOpen(resultKey('merged', index, item))" class="result-detail">
              <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
              <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
            </div>
          </div>
        </div>
        <el-empty v-else description="暂无搜索结果" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ArrowDown, ArrowUp, Search } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, ProviderCallLog, ProviderConfig, ProviderHealth } from '../api/client'
import { providerLabel, providerOptions } from '../utils/providers'

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

type ProviderSummary = {
  provider: string
  key_alias?: string
  status: string
  error?: string
  latency_ms: number
  result_count: number
  cached?: boolean
}

type ProviderResultGroup = ProviderSummary & {
  results?: SearchResultItem[]
}

type ProviderRow = ProviderSummary & {
  key: string
  attempt_index?: number
  will_retry?: boolean
  results: SearchResultItem[]
}

type SearchResponse = {
  results: SearchResultItem[]
  providers: ProviderSummary[]
  provider_results?: ProviderResultGroup[]
  provider_calls?: ProviderCallLog[]
  meta?: {
    request_id?: string
    total_results?: number
    deduped_results?: number
    latency_ms?: number
  }
}

const loading = ref(false)
const loadingMeta = ref(true)
const ready = ref(false)
const result = ref<SearchResponse | null>(null)
const openResultKeys = ref<string[]>([])
const providers = ref<ProviderConfig[]>([])
const providerHealth = ref<ProviderHealth[]>([])
const form = reactive({
  query: '',
  mode: 'fallback',
  providers: [] as string[],
  limit: 8,
  cache: 'default'
})

const availableProviderOptions = computed(() => {
  const readyNames = new Set(
    providers.value
      .filter((item) => item.enabled)
      .filter((item) => {
        const health = providerHealth.value.find((row) => row.provider_name === item.name)
        const keys = health?.available_keys ?? item.available_keys ?? 0
        return keys > 0
      })
      .map((item) => item.name)
  )
  const options = providerOptions.filter((item) => readyNames.has(item.value))
  return options.length ? options : providerOptions
})

const providerRows = computed<ProviderRow[]>(() => {
  if (!result.value) return []
  const groups = new Map((result.value.provider_results || []).map((group) => [group.provider, group]))
  const usedProviders = new Set<string>()
  const calls = result.value.provider_calls || []
  if (calls.length) {
    return calls.map((call, index) => {
      const group = groups.get(call.provider_name)
      const status = call.status || group?.status || 'success'
      const hasResults = status === 'success'
      usedProviders.add(call.provider_name)
      return {
        provider: call.provider_name,
        key: providerCallKey(call, index),
        key_alias: call.key_alias || group?.key_alias || '',
        attempt_index: call.attempt_index || 1,
        will_retry: Boolean(call.will_retry),
        status,
        error: call.error_message || (hasResults ? '' : group?.error || ''),
        latency_ms: call.latency_ms || group?.latency_ms || 0,
        result_count: call.result_count || (hasResults ? group?.result_count || group?.results?.length || 0 : 0),
        cached: call.cached || Boolean(group?.cached),
        results: hasResults ? group?.results || [] : []
      }
    })
  }
  const rows = result.value.providers.map((provider, index) => {
    const group = groups.get(provider.provider)
    const status = provider.status || group?.status || 'success'
    usedProviders.add(provider.provider)
    return {
      ...provider,
      key: `${provider.provider}-${index}`,
      attempt_index: 1,
      will_retry: false,
      key_alias: provider.key_alias || group?.key_alias || '',
      status,
      error: provider.error || group?.error || '',
      latency_ms: provider.latency_ms || group?.latency_ms || 0,
      result_count: provider.result_count || group?.result_count || group?.results?.length || 0,
      cached: provider.cached || Boolean(group?.cached),
      results: status === 'success' ? group?.results || [] : []
    }
  })
  for (const group of result.value.provider_results || []) {
    if (usedProviders.has(group.provider)) continue
    rows.push({
      ...group,
      key: `${group.provider}-${rows.length}`,
      attempt_index: 1,
      will_retry: false,
      key_alias: group.key_alias || '',
      status: group.status || 'success',
      error: group.error || '',
      latency_ms: group.latency_ms || 0,
      result_count: group.result_count || group.results?.length || 0,
      cached: Boolean(group.cached),
      results: group.status === 'success' ? group.results || [] : []
    })
  }
  return rows
})
const hasProviderResults = computed(() => providerRows.value.some((row) => row.results.length > 0))

function resultProviderLabel(item: SearchResultItem, fallback = '未知渠道') {
  if (item.providers?.length) return item.providers.map(providerLabel).join(', ')
  return providerLabel(item.provider || fallback)
}

function formatScore(value: number) {
  if (!Number.isFinite(value)) return '-'
  return Number.isInteger(value) ? String(value) : value.toFixed(2)
}

function formatLatency(value: number) {
  const latency = Number(value || 0)
  if (latency >= 1000) return `${(latency / 1000).toFixed(2)}s`
  return `${latency}ms`
}

function providerCallKey(call: ProviderCallLog, index: number) {
  return `${call.provider_name}-${call.provider_key_id || 'no-key'}-${call.attempt_index || 1}-${index}`
}

function callEmptyDescription(call: ProviderRow) {
  if (call.error) return call.will_retry ? `${call.error}，将换 key 重试` : call.error
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

async function loadMeta() {
  loadingMeta.value = true
  try {
    const [providerRes, healthRes, settings] = await Promise.all([
      api.providers(),
      api.providerHealth().catch(() => ({ providers: [] as ProviderHealth[] })),
      api.settings().catch(() => null)
    ])
    providers.value = providerRes.providers || []
    providerHealth.value = healthRes.providers || []
    const readyProviders = providers.value.filter((item) => {
      if (!item.enabled) return false
      const health = providerHealth.value.find((row) => row.provider_name === item.name)
      return (health?.available_keys ?? item.available_keys ?? 0) > 0
    })
    ready.value = readyProviders.length > 0
    const preferred = settings?.default_providers?.filter((name) => readyProviders.some((item) => item.name === name)) || []
    form.providers = preferred.length ? preferred : readyProviders.map((item) => item.name)
    if (settings?.default_mode) form.mode = settings.default_mode
    if (settings?.default_limit) form.limit = settings.default_limit
  } catch (error) {
    ready.value = false
    ElMessage.error((error as Error).message)
  } finally {
    loadingMeta.value = false
  }
}

async function run() {
  if (!ready.value) {
    ElMessage.warning('请先配置可用平台')
    return
  }
  if (!form.query.trim()) {
    ElMessage.warning('请输入搜索词')
    return
  }
  loading.value = true
  openResultKeys.value = []
  try {
    result.value = await api.playgroundSearch({
      query: form.query,
      mode: form.mode,
      providers: form.providers,
      limit: form.limit
    }) as SearchResponse
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

onMounted(loadMeta)
</script>

<style scoped>
.playground-page { max-width: 1080px; margin: 0 auto; }
.playground-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.playground-head h1 {
  margin: 0;
  font-size: 22px;
  letter-spacing: -0.02em;
}
.search-shell { overflow: hidden; }
.search-shell.blocked { opacity: 0.55; }
.search-row {
  display: grid;
  grid-template-columns: 24px 1fr auto;
  gap: 10px;
  align-items: center;
}
.search-icon { color: var(--faint); }
.search-input :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent;
  padding-left: 0;
}
.search-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}
.provider-select { min-width: 220px; flex: 1; }
.limit-field {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 650;
}
.limit-field :deep(.el-input-number) { width: 110px; }
.result-card { margin-top: 16px; }
.result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.result-meta { margin-left: 10px; font-size: 12px; font-weight: 500; }
.section-block { margin-top: 16px; }
.section-title { margin-bottom: 8px; color: var(--text); font-weight: 800; }
.provider-call-table { width: 100%; }
.call-status-cell { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.provider-call-table :deep(.el-table__expanded-cell) { padding: 10px 12px; background: rgba(11, 110, 79, 0.04); }
.provider-expanded-results { max-height: 320px; overflow-y: auto; padding-right: 4px; }
.search-result-list { display: flex; flex-direction: column; gap: 8px; }
.merged-result-list { max-height: 560px; overflow-y: auto; }
.search-result-card {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
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
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-decoration: none;
}
.result-title-link:hover { color: var(--primary); }
.result-row-meta { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.result-score { color: var(--muted); font-size: 12px; white-space: nowrap; }
.result-expand-button { width: 24px; height: 24px; min-height: 24px; padding: 0; color: var(--primary); }
.result-detail { margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border); }
.result-snippet { margin: 0; color: var(--muted); line-height: 1.6; white-space: pre-wrap; }
.result-content { margin-top: 8px; color: var(--text); }
@media (max-width: 720px) {
  .search-row { grid-template-columns: 1fr; }
  .result-row { align-items: flex-start; flex-direction: column; }
}
</style>
