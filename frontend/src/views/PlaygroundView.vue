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
        <el-table :data="result.providers" size="small" border>
          <el-table-column prop="provider" label="渠道" />
          <el-table-column prop="key_alias" label="密钥" />
          <el-table-column prop="status" label="状态" width="90"><template #default="scope"><el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ? '成功' : '失败' }}</el-tag></template></el-table-column>
          <el-table-column prop="result_count" label="结果" width="80" />
          <el-table-column prop="latency_ms" label="延迟" width="100"><template #default="scope">{{ formatLatency(scope.row.latency_ms) }}</template></el-table-column>
          <el-table-column prop="error" label="错误" />
        </el-table>
      </div>

      <div class="section-block">
        <div class="section-title">结果列表</div>
        <div v-if="result.results.length" class="search-result-list">
          <div v-for="(item, index) in result.results" :key="index" class="search-result-card">
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
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
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
}

type SearchResponse = {
  results: SearchResultItem[]
  providers: ProviderSummary[]
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

function formatLatency(value: number) {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}s`
  return `${value}ms`
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
.search-result-list { display: flex; flex-direction: column; gap: 10px; max-height: 560px; overflow-y: auto; padding-right: 2px; }
.search-result-card { padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: #fff; }
.result-card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.result-title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); font-weight: 800; }
.result-url { display: block; margin-top: 6px; overflow: hidden; color: var(--primary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; text-decoration: none; }
.result-meta { display: flex; gap: 12px; margin-top: 8px; color: var(--muted); font-size: 12px; }
.result-snippet { margin: 0; color: var(--muted); line-height: 1.6; white-space: pre-wrap; }
.result-content { margin-top: 8px; color: var(--text); }
.result-collapse { margin-top: 8px; border-top: 1px solid var(--border); border-bottom: 0; }
.result-collapse :deep(.el-collapse-item__header) { height: 36px; border-bottom: 0; color: var(--primary); font-weight: 700; }
.result-collapse :deep(.el-collapse-item__wrap) { border-bottom: 0; }
.result-collapse :deep(.el-collapse-item__content) { padding-bottom: 0; }
@media (max-width: 1100px) { .result-summary { grid-template-columns: 1fr; } }
</style>
