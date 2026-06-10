<template>
  <div>
    <el-card class="soft-card" shadow="never">
      <el-form label-position="top">
        <el-form-item label="搜索词"><el-input v-model="form.query" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="8"><el-form-item label="搜索模式"><el-select v-model="form.mode"><el-option value="parallel" label="并发聚合" /><el-option value="fallback" label="失败转移" /><el-option value="single" label="单平台" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="搜索平台"><el-select v-model="form.providers" multiple><el-option value="exa" label="Exa" /><el-option value="you" label="You.com" /><el-option value="jina" label="Jina" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="汇总返回结果数"><el-input-number v-model="form.limit" :min="1" :max="50" controls-position="right" /></el-form-item></el-col>
        </el-row>
        <div class="search-action-bar">
          <el-button type="primary" :loading="loading" @click="run">开始搜索</el-button>
        </div>
      </el-form>
    </el-card>

    <el-card v-if="result" class="soft-card" shadow="never" style="margin-top:16px">
      <template #header>搜索结果</template>
      <div class="result-summary">
        <div class="summary-item"><span>请求 ID</span><strong>{{ result.meta?.request_id || '-' }}</strong></div>
        <div class="summary-item"><span>总结果</span><strong>{{ result.meta?.total_results || result.results.length }}</strong></div>
        <div class="summary-item"><span>去重数量</span><strong>{{ result.meta?.deduped_results || 0 }}</strong></div>
        <div class="summary-item"><span>耗时</span><strong>{{ formatLatency(result.meta?.latency_ms || 0) }}</strong></div>
      </div>

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
                        <el-button v-if="hasResultDetails(item)" link class="result-expand-button" :icon="isResultOpen(resultKey(scope.row.key, index, item)) ? ArrowUp : ArrowDown" :title="isResultOpen(resultKey(scope.row.key, index, item)) ? '收起内容' : '展开内容'" @click.stop="toggleResultKey(resultKey(scope.row.key, index, item))" />
                      </div>
                    </div>
                    <div v-if="hasResultDetails(item) && isResultOpen(resultKey(scope.row.key, index, item))" class="result-detail">
                      <p v-if="item.snippet" class="result-snippet">{{ item.snippet }}</p>
                      <p v-if="item.content" class="result-snippet result-content">{{ item.content }}</p>
                    </div>
                  </div>
                </div>
                <el-empty v-else :description="scope.row.error || '该渠道暂无搜索结果'" />
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="provider" label="渠道"><template #default="scope">{{ providerLabel(scope.row.provider) }}</template></el-table-column>
          <el-table-column prop="key_alias" label="密钥" />
          <el-table-column prop="status" label="状态" width="90"><template #default="scope"><el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ? '成功' : '失败' }}</el-tag></template></el-table-column>
          <el-table-column prop="result_count" label="结果" width="80" />
          <el-table-column prop="latency_ms" label="延迟" width="100"><template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template></el-table-column>
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
                <el-button v-if="hasResultDetails(item)" link class="result-expand-button" :icon="isResultOpen(resultKey('merged', index, item)) ? ArrowUp : ArrowDown" :title="isResultOpen(resultKey('merged', index, item)) ? '收起内容' : '展开内容'" @click.stop="toggleResultKey(resultKey('merged', index, item))" />
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
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import { api } from '../api/client'

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
  results: SearchResultItem[]
}

type SearchResponse = {
  results: SearchResultItem[]
  providers: ProviderSummary[]
  provider_results?: ProviderResultGroup[]
  meta?: {
    request_id?: string
    total_results?: number
    deduped_results?: number
    latency_ms?: number
  }
}

const loading = ref(false)
const result = ref<SearchResponse | null>(null)
const openResultKeys = ref<string[]>([])
const form = reactive({ query: 'latest web search APIs', mode: 'parallel', providers: ['exa', 'you', 'jina'], limit: 10, cache: 'default' })

const providerRows = computed<ProviderRow[]>(() => {
  if (!result.value) return []
  const groups = new Map((result.value.provider_results || []).map((group) => [group.provider, group]))
  const usedProviders = new Set<string>()
  const rows = result.value.providers.map((provider, index) => {
    const group = groups.get(provider.provider)
    usedProviders.add(provider.provider)
    return {
      ...provider,
      key: `${provider.provider}-${index}`,
      key_alias: provider.key_alias || group?.key_alias || '',
      status: provider.status || group?.status || 'success',
      error: provider.error || group?.error || '',
      latency_ms: provider.latency_ms || group?.latency_ms || 0,
      result_count: provider.result_count || group?.result_count || group?.results?.length || 0,
      cached: provider.cached || Boolean(group?.cached),
      results: group?.results || []
    }
  })
  for (const group of result.value.provider_results || []) {
    if (usedProviders.has(group.provider)) continue
    rows.push({
      ...group,
      key: `${group.provider}-${rows.length}`,
      key_alias: group.key_alias || '',
      status: group.status || 'success',
      error: group.error || '',
      latency_ms: group.latency_ms || 0,
      result_count: group.result_count || group.results?.length || 0,
      cached: Boolean(group.cached),
      results: group.results || []
    })
  }
  return rows
})
const hasProviderResults = computed(() => providerRows.value.some((row) => row.results.length > 0))

function providerLabel(provider: string) {
  return ({ exa: 'Exa', you: 'You.com', jina: 'Jina' } as Record<string, string>)[provider] || provider
}

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

async function run() {
  loading.value = true
  openResultKeys.value = []
  try {
    result.value = await api.playgroundSearch(form) as SearchResponse
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.search-action-bar { display: flex; justify-content: flex-end; }
.result-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.summary-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: #fff; }
.summary-item span { color: var(--muted); }
.summary-item strong { min-width: 0; overflow: hidden; color: var(--text); text-overflow: ellipsis; white-space: nowrap; }
.section-block { margin-top: 16px; }
.section-title { margin-bottom: 8px; color: var(--text); font-weight: 800; }
.provider-call-table { width: 100%; }
.provider-call-table :deep(.el-table__expanded-cell) { padding: 10px 12px; background: rgba(47, 148, 97, .04); }
.provider-expanded-results { max-height: 320px; overflow-y: auto; padding-right: 4px; }
.search-result-list { display: flex; flex-direction: column; gap: 8px; padding-right: 2px; }
.provider-result-list { padding-bottom: 2px; }
.merged-result-list { max-height: 560px; overflow-y: auto; }
.search-result-card { padding: 8px 10px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: #fff; }
.result-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 28px; }
.result-title-link { min-width: 0; overflow: hidden; color: var(--text); font-weight: 800; text-overflow: ellipsis; white-space: nowrap; text-decoration: none; }
.result-title-link:hover { color: var(--primary); }
.result-title-text { display: block; }
.result-row-meta { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.result-score { color: var(--muted); font-size: 12px; white-space: nowrap; }
.result-expand-button { width: 24px; height: 24px; min-height: 24px; padding: 0; color: var(--primary); }
.result-detail { margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border); }
.result-snippet { margin: 0; color: var(--muted); line-height: 1.6; white-space: pre-wrap; }
.result-content { margin-top: 8px; color: var(--text); }
@media (max-width: 1100px) { .result-summary { grid-template-columns: 1fr; } }
@media (max-width: 720px) {
  .result-row { align-items: flex-start; flex-direction: column; }
  .result-row-meta { flex-wrap: wrap; }
}
</style>
