<template>
  <div>
    <div class="page-actions"><el-button @click="load">刷新</el-button></div>
    <el-card class="soft-card" shadow="never">
      <el-table :data="logs" stripe>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="request_id" label="请求 ID" width="210" />
        <el-table-column prop="query" label="搜索词" />
        <el-table-column prop="mode" label="模式" width="100"><template #default="scope">{{ modeLabel(scope.row.mode) }}</template></el-table-column>
        <el-table-column prop="compat_format" label="格式" width="100" />
        <el-table-column prop="cache_hit" label="缓存" width="90"><template #default="scope">{{ scope.row.cache_hit ? '命中' : '未命中' }}</template></el-table-column>
        <el-table-column prop="result_count" label="结果" width="80" />
        <el-table-column prop="status" label="状态" width="100"><template #default="scope"><el-tag :type="scope.row.status === 'success' ? 'success' : 'danger'">{{ scope.row.status === 'success' ? '成功' : '失败' }}</el-tag></template></el-table-column>
        <el-table-column prop="latency_ms" label="延迟(ms)" width="110" />
        <el-table-column label="操作" width="90"><template #default="scope"><el-button link type="primary" @click="openDetail(scope.row)">查看</el-button></template></el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="请求详细日志" width="920px" class="log-detail-dialog">
      <template v-if="selectedLog">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="请求 ID">{{ selectedLog.request_id }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ selectedLog.created_at }}</el-descriptions-item>
          <el-descriptions-item label="模式">{{ modeLabel(selectedLog.mode) }}</el-descriptions-item>
          <el-descriptions-item label="格式">{{ selectedLog.compat_format }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ selectedLog.status === 'success' ? '成功' : '失败' }}</el-descriptions-item>
          <el-descriptions-item label="延迟">{{ selectedLog.latency_ms }} ms</el-descriptions-item>
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
            <el-table-column prop="latency_ms" label="延迟(ms)" width="100" />
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
import { computed, onMounted, ref } from 'vue'
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

function callSuccess(call: ProviderCallLog) {
  return call.status === 'success'
}

async function load() {
  logs.value = (await api.logs()).logs
}

async function openDetail(row: SearchLog) {
  const result = await api.logDetail(row.id)
  selectedLog.value = result.log
  detailCalls.value = result.calls
  openResultKeys.value = []
  detailVisible.value = true
}

onMounted(load)
</script>

<style scoped>
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
