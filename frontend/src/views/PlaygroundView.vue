<template>
  <div class="playground-page" :class="result ? 'has-result' : 'is-home'">
    <div class="hero">
      <div class="brand">
        <h1>One<em>Search</em></h1>
        <p>
          统一搜索调试
          <el-tag :type="ready ? 'success' : 'warning'" effect="light" round size="small">
            {{ ready ? '可用' : '待配置' }}
          </el-tag>
        </p>
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

        <div class="shell" :class="{ blocked: !ready }">
          <div class="search-row">
            <el-icon class="search-icon" :size="18"><Search /></el-icon>
            <el-input
              v-model="form.query"
              class="search-input"
              placeholder="输入搜索词"
              :disabled="!ready"
              @keyup.enter="run"
            />
            <el-button type="primary" class="search-btn" :loading="loading" :disabled="!ready || !form.query.trim()" @click="run">
              搜索
            </el-button>
          </div>
          <div class="filters">
            <label class="chip on">
              <span>模式</span>
              <el-select v-model="form.mode" size="small" :disabled="!ready" class="chip-select mode-select">
                <el-option value="fallback" label="转移" />
                <el-option value="parallel" label="并发" />
                <el-option value="single" label="单源" />
              </el-select>
            </label>
            <label class="chip">
              <span>平台</span>
              <el-select
                v-model="form.providers"
                class="chip-select provider-select"
                size="small"
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
            </label>
            <label class="chip">
              <span>条数</span>
              <el-input-number
                v-model="form.limit"
                class="chip-number"
                size="small"
                :min="1"
                :max="50"
                controls-position="right"
                :disabled="!ready"
              />
            </label>
          </div>
        </div>
      </template>
    </div>

    <div v-if="result" class="stage">
      <div class="stats">
        <span class="pill"><b>{{ result.meta?.total_results || result.results.length }}</b> 条结果</span>
        <span class="pill">去重 <b>{{ result.meta?.deduped_results || 0 }}</b></span>
        <span class="pill">耗时 <b>{{ formatLatency(result.meta?.latency_ms || 0) }}</b></span>
        <span v-if="result.meta?.request_id" class="pill mono">{{ result.meta.request_id }}</span>
      </div>

      <div class="results-wrap">
        <div v-if="result.results.length" ref="resultListEl" class="result-list">
          <article
            v-for="(item, index) in result.results"
            :key="resultKey('merged', index, item)"
            class="result"
            :style="{ animationDelay: `${0.08 + index * 0.05}s` }"
          >
            <div class="site">
              <span class="ico">{{ siteInitial(item.url, item.title) }}</span>
              <span class="site-text">{{ siteLabel(item.url) || '无链接' }}</span>
            </div>
            <h3>
              <a v-if="item.url" :href="item.url" target="_blank" rel="noreferrer">{{ item.title || item.url }}</a>
              <span v-else>{{ item.title || '无标题' }}</span>
            </h3>
            <p
              v-if="item.snippet || item.content"
              class="snip"
              :class="{ open: isResultOpen(resultKey('merged', index, item)) }"
            >
              {{ isResultOpen(resultKey('merged', index, item)) ? (item.content || item.snippet) : (item.snippet || item.content) }}
            </p>
            <div class="foot">
              <span class="tag">{{ resultProviderLabel(item) }}</span>
              <span v-if="item.score !== undefined" class="tag muted">评分 {{ formatScore(item.score) }}</span>
              <button
                v-if="hasResultDetails(item)"
                type="button"
                class="expand-btn"
                @click="toggleResultKey(resultKey('merged', index, item))"
              >
                {{ isResultOpen(resultKey('merged', index, item)) ? '收起' : '展开' }}
              </button>
            </div>
          </article>
        </div>
        <el-empty v-else description="暂无搜索结果" />

        <aside v-if="providerRows.length" class="panel">
          <h4>
            渠道调用
            <span class="live" :class="{ on: loading }"><i /><span>{{ loading ? 'searching' : 'done' }}</span></span>
          </h4>

          <div
            v-for="(row, index) in providerRows"
            :key="row.key"
            class="call"
            :style="{ animationDelay: `${0.16 + index * 0.06}s` }"
          >
            <div class="top">
              <span>{{ providerLabel(row.provider) }}</span>
              <span :class="row.status === 'success' ? 'ok' : 'fail'">
                {{ row.status === 'success' ? '成功' : '失败' }}
              </span>
            </div>
            <div class="sub">
              {{ formatLatency(row.latency_ms) }}
              <template v-if="row.key_alias"> · key {{ row.key_alias }}</template>
              <template v-if="row.status === 'success'"> · {{ row.result_count }} 条</template>
              <template v-if="row.attempt_index"> · 第 {{ row.attempt_index }} 次</template>
              <template v-if="row.cached"> · 缓存</template>
              <template v-if="row.will_retry"> · 将重试</template>
              <template v-if="row.error"> · {{ row.error }}</template>
            </div>
            <div class="latency" :class="row.status === 'success' ? 'ok' : 'fail'">
              <i :style="{ width: latencyWidth(row.latency_ms) + '%' }" />
            </div>
          </div>

          <div v-if="result.meta?.request_id" class="panel-meta">
            mode={{ form.mode }}<br />
            {{ result.meta.request_id }}
          </div>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Search } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, ProviderCallLog, ProviderConfig, ProviderHealth } from '../api/client'
import { providerLabel, providerOptions } from '../utils/providers'

defineOptions({ name: 'PlaygroundView' })

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
const resultListEl = ref<HTMLElement | null>(null)
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

const maxLatency = computed(() => Math.max(1, ...providerRows.value.map((row) => Number(row.latency_ms || 0))))

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

function latencyWidth(value: number) {
  const ms = Number(value || 0)
  return Math.max(8, Math.min(100, Math.round((ms / maxLatency.value) * 100)))
}

function siteLabel(url?: string) {
  if (!url) return ''
  try {
    const parsed = new URL(url)
    const path = parsed.pathname === '/' ? '' : parsed.pathname.replace(/\/$/, '')
    return `${parsed.hostname}${path}`
  } catch {
    return url
  }
}

function siteInitial(url?: string, title?: string) {
  const raw = siteLabel(url) || title || '?'
  return raw.replace(/^www\./i, '').charAt(0).toUpperCase() || '?'
}

function providerCallKey(call: ProviderCallLog, index: number) {
  return `${call.provider_name}-${call.provider_key_id || 'no-key'}-${call.attempt_index || 1}-${index}`
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
    await nextTick()
    resultListEl.value?.scrollTo({ top: 0 })
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

onMounted(loadMeta)
</script>

<style scoped>
@keyframes rise {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: none; }
}
@keyframes fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes pulse-dot {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.35); opacity: .55; }
}
@keyframes bar-fill {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}
@keyframes float-in {
  from { opacity: 0; transform: translate(8px, 8px); }
  to { opacity: 1; transform: none; }
}

.playground-page {
  --content: 720px;
  --panel: 260px;
  --gap: 20px;
  --home-shift: max(96px, calc(42vh - 160px));
  /* 对齐 page-main 上下 padding，整页不滚 */
  --frame-h: calc(100dvh - 76px);
  width: 100%;
  max-width: var(--content);
  margin: 0 auto;
  height: var(--frame-h);
  /* 有结果时允许侧栏浮出；首页态再裁切，避免 translate 撑出滚动 */
  overflow: visible;
  display: flex;
  flex-direction: column;
}
.playground-page.is-home {
  overflow: hidden;
}

/* 无结果：搜索区垂直居中；有结果：平滑上移到顶部 */
.hero {
  width: 100%;
  flex: 0 0 auto;
  transform: translateY(0);
  transition: transform .5s cubic-bezier(.22, 1, .36, 1);
  will-change: transform;
}
.playground-page.is-home .hero {
  transform: translateY(var(--home-shift));
}
.playground-page.has-result .hero {
  transform: translateY(0);
}
.playground-page.has-result .brand {
  margin-bottom: 12px;
}
.playground-page.has-result .brand h1 {
  font-size: 22px;
}
.brand h1 {
  margin: 0;
  font-size: 32px;
  letter-spacing: -0.04em;
  font-weight: 800;
  transition: font-size .45s ease;
}

.brand {
  width: 100%;
  text-align: center;
  margin-bottom: 22px;
  transition: margin-bottom .45s ease;
}
.brand h1 em { font-style: normal; color: var(--primary); }
.brand p {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 13px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.setup-banner {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  padding: 14px 16px;
  border: 1px solid #f5d0a8;
  border-radius: 14px;
  background: #fffaf0;
  animation: rise .4s ease both;
}
.setup-banner p { margin: 0 0 2px; font-weight: 700; }
.setup-banner .sub { color: var(--muted); font-size: 12px; }

.shell {
  width: 100%;
  background: #fff;
  border-radius: 20px;
  box-shadow: var(--shadow);
  border: 1px solid var(--border);
  padding: 10px;
  transition: box-shadow .2s ease, border-color .2s ease;
}
.shell:focus-within {
  border-color: #b7e4d2;
  box-shadow: 0 1px 2px rgba(16,24,40,.04), 0 16px 48px rgba(11,110,79,.12);
}
.shell.blocked { opacity: .55; }

.search-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 6px 6px 14px;
}
.search-icon { color: var(--faint); flex: 0 0 auto; }
.search-input { flex: 1; min-width: 0; }
.search-input :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent;
  padding-left: 0;
}
.search-btn {
  height: 42px;
  border-radius: 14px !important;
  padding: 0 20px;
  font-weight: 650;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px 8px 4px;
  border-top: 1px solid #f0f2f5;
  margin-top: 6px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #f6f7f9;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 4px 10px 4px 12px;
  font-size: 12px;
  color: #475467;
  font-weight: 600;
  transition: background .15s, border-color .15s, transform .12s;
}
.chip:hover { transform: translateY(-1px); }
.chip.on {
  background: var(--primary-soft);
  border-color: #b7e4d2;
  color: var(--primary-ink);
}
.chip-select { width: auto; }
.chip-select :deep(.el-select__wrapper) {
  box-shadow: none !important;
  background: transparent;
  min-height: 28px;
  padding: 0 4px 0 0;
}
.mode-select { width: 84px; }
.provider-select { min-width: 140px; max-width: 220px; }
.chip-number { width: 96px; }
.chip-number :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent;
  padding-left: 0;
}

.stage {
  width: 100%;
  margin-top: 14px;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  animation: rise .45s .12s ease both;
}
.stats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--muted);
  animation: fade-in .4s .1s ease both;
}
.pill {
  background: #fff;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 4px 10px;
  transition: border-color .15s, transform .12s;
}
.pill:hover { border-color: #cfe8dc; transform: translateY(-1px); }
.stats b { color: var(--text); font-weight: 700; }
.mono { font-family: var(--mono); }

.results-wrap {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  /* 不可 hidden，否则 absolute 侧栏会被裁掉 */
  overflow: visible;
}
.result-list {
  height: 100%;
  max-height: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  padding-right: 2px;
  padding-bottom: 8px;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}
.result-list::-webkit-scrollbar { width: 8px; }
.result-list::-webkit-scrollbar-thumb {
  background: #d0d5dd;
  border-radius: 99px;
}
.result-list::-webkit-scrollbar-track { background: transparent; }

.result {
  background: #fff;
  border: 1px solid #e8eaed;
  border-radius: 16px;
  padding: 16px 18px;
  margin-bottom: 12px;
  box-shadow: 0 1px 2px rgba(16,24,40,.03);
  transition: box-shadow .2s, border-color .2s, transform .2s;
  animation: rise .45s ease both;
}
.result:last-child { margin-bottom: 4px; }
.result:hover {
  border-color: #cfe8dc;
  box-shadow: 0 10px 28px rgba(11,110,79,.1);
  transform: translateY(-2px);
}
.site {
  font-size: 12px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.site-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ico {
  width: 22px;
  height: 22px;
  border-radius: 8px;
  background: linear-gradient(135deg, #e8f6f0, #d4f0e4);
  display: grid;
  place-items: center;
  font-size: 11px;
  font-weight: 700;
  color: var(--primary);
  flex: 0 0 auto;
}
.result h3 {
  margin: 8px 0 6px;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.result h3 a,
.result h3 span {
  color: var(--text);
  text-decoration: none;
  transition: color .15s;
}
.result h3 a:hover { color: var(--primary); }
.snip {
  margin: 0;
  color: #475467;
  font-size: 14px;
  line-height: 1.55;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}
.snip.open {
  display: block;
  -webkit-line-clamp: unset;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
}
.foot {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  align-items: center;
}
.tag {
  font-size: 11px;
  font-weight: 650;
  color: var(--primary-ink);
  background: var(--primary-soft);
  border-radius: 999px;
  padding: 3px 8px;
}
.tag.muted {
  color: var(--muted);
  background: #f2f4f7;
}
.expand-btn {
  border: 0;
  background: transparent;
  color: var(--primary);
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
  padding: 0;
}
.expand-btn:hover { text-decoration: underline; }

.panel {
  position: absolute;
  top: 0;
  left: calc(100% + var(--gap));
  width: var(--panel);
  max-height: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  background: #fff;
  border: 1px solid #e8eaed;
  border-radius: 16px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(16,24,40,.04), 0 12px 32px rgba(16,24,40,.08);
  animation: float-in .45s .15s ease both;
  z-index: 2;
}
.panel h4 {
  margin: 0 0 12px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: .08em;
  color: var(--muted);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  letter-spacing: .04em;
  color: var(--faint);
  text-transform: uppercase;
}
.live.on { color: var(--primary); }
.live i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  display: block;
}
.live.on i { animation: pulse-dot 1.4s ease infinite; }

.call {
  background: #f8faf9;
  border: 1px solid #e8eaed;
  border-radius: 12px;
  padding: 11px 12px;
  margin-bottom: 8px;
  transition: background .15s, border-color .15s, transform .15s, box-shadow .15s;
  animation: rise .4s ease both;
}
.call:hover {
  background: #fff;
  border-color: #cfe8dc;
  box-shadow: 0 6px 16px rgba(11,110,79,.08);
  transform: translateY(-1px);
}
.call .top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 650;
}
.call .sub {
  margin-top: 5px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.4;
  word-break: break-word;
}
.ok { color: var(--primary); }
.fail { color: var(--danger); }
.latency {
  margin-top: 10px;
  height: 3px;
  background: #eef1f4;
  border-radius: 99px;
  overflow: hidden;
}
.latency > i {
  display: block;
  height: 100%;
  border-radius: inherit;
  transform-origin: left center;
  animation: bar-fill .7s ease both;
}
.latency.ok > i { background: linear-gradient(90deg, #0b6e4f, #34d399); }
.latency.fail > i { background: linear-gradient(90deg, #b42318, #f97066); }

.panel-meta {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #eef1f4;
  font-size: 11px;
  color: var(--faint);
  line-height: 1.5;
  font-family: var(--mono);
  word-break: break-all;
}

@media (max-width: 1180px) {
  .results-wrap {
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
  }
  .result-list {
    height: auto;
    max-height: none;
    overflow: visible;
    padding-bottom: 0;
  }
  .panel {
    position: static;
    width: 100%;
    max-height: none;
    margin-top: 8px;
    animation-name: rise;
  }
}
@media (max-width: 640px) {
  .playground-page {
    --home-shift: max(48px, calc(28vh - 120px));
    --frame-h: calc(100dvh - 48px);
  }
  .search-row { flex-direction: column; align-items: stretch; padding: 10px; }
  .search-btn { width: 100%; }
  .setup-banner { flex-direction: column; align-items: flex-start; }
}

@media (prefers-reduced-motion: reduce) {
  .hero,
  .brand h1,
  .stage,
  .result,
  .call { transition: none !important; animation: none !important; }
}
</style>
