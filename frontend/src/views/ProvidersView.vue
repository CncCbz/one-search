<template>
  <div>
    <div class="page-actions">
      <el-button @click="load">刷新</el-button>
    </div>

    <div class="provider-grid">
      <el-card v-for="card in providerCards" :key="card.name" class="soft-card provider-card" shadow="never" @click="openProvider(card)">
        <div class="provider-card-header">
          <div style="display:flex;align-items:center;gap:12px;min-width:0">
            <div class="provider-icon">{{ providerShortName(card.display_name || card.name) }}</div>
            <div style="min-width:0">
              <div style="font-size:18px;font-weight:800;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{ card.display_name }}</div>
              <div class="muted" style="font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{ card.base_url }}</div>
            </div>
          </div>
          <el-switch :model-value="card.enabled" @click.stop @change="toggleProvider(card, Boolean($event))" />
        </div>

        <div class="metric-row">
          <div class="metric-pill"><div class="label">绑定密钥</div><div class="value">{{ card.enabledKeyCount }}/{{ card.keyCount }}</div></div>
          <div class="metric-pill"><div class="label">累计调用</div><div class="value">{{ card.totalCalls }}</div></div>
          <div class="metric-pill"><div class="label">成功</div><div class="value" style="color:var(--primary)">{{ card.totalSuccess }}</div></div>
          <div class="metric-pill"><div class="label">失败</div><div class="value" style="color:var(--danger)">{{ card.totalFailure }}</div></div>
        </div>

        <div style="display:flex;align-items:center;margin-top:14px">
          <span class="muted" style="font-size:12px">超时 {{ card.timeout_ms }} ms · 缓存 {{ card.default_cache_enabled ? card.cache_ttl_seconds + ' 秒' : '关闭' }}</span>
        </div>
      </el-card>
    </div>

    <el-dialog v-model="dialogVisible" width="760px" top="3vh" :title="dialogTitle" destroy-on-close class="provider-dialog channel-style-dialog">
      <template v-if="providerForm">
        <div class="channel-dialog-body">
          <section class="channel-section">
            <div class="section-line-header">
              <div>基础 URL</div>
              <el-button link :icon="CopyDocument" @click="copyText(providerForm.base_url)">复制</el-button>
            </div>
            <el-input v-model="providerForm.base_url" class="base-url-input" placeholder="https://example.com" />
          </section>

          <section class="channel-section">
            <div class="section-line-header">
              <div>API 密钥 ({{ selectedKeys.length }})</div>
              <el-button link :icon="Plus" :disabled="creatingRow" @click="startCreateKey">添加</el-button>
            </div>
            <div class="api-key-list">
              <div v-for="row in tableKeys" :key="row.isNew ? 'new-key' : row.id" class="api-key-row" :class="{ 'is-new': row.isNew }">
                <template v-if="row.isNew">
                  <el-input v-model="draftKey" class="key-input" type="password" show-password placeholder="API Key" />
                  <div class="new-key-actions">
                    <el-button link type="primary" :loading="creatingKey" @click="createKey">保存</el-button>
                    <el-button link @click="cancelCreateKey">取消</el-button>
                  </div>
                </template>
                <template v-else>
                  <div class="key-main">
                    <el-input class="key-input" :model-value="visibleKeyIds.has(row.id) ? row.key : maskKey(row.key)" readonly>
                      <template #suffix>
                        <div class="key-icon-actions">
                          <el-button link :icon="visibleKeyIds.has(row.id) ? Hide : View" title="显示/隐藏密钥" @click="toggleKeyVisible(row.id)" />
                        </div>
                      </template>
                    </el-input>
                    <div class="key-meta">
                      <span>成功 {{ row.total_successes }}</span>
                      <span>失败 {{ row.total_failures }}</span>
                      <span>成功率 {{ successRate(row) }}%</span>
                    </div>
                  </div>
                  <el-button link type="primary" :loading="testingKeyId === row.id" title="测试密钥" @click="testKey(row)">测试</el-button>
                  <el-button link type="danger" title="删除密钥" @click="removeKey(row.id)">删除</el-button>
                </template>
              </div>
              <div v-if="tableKeys.length === 0" class="empty-key-row">暂无密钥，点击右侧“添加”创建</div>
            </div>
          </section>

          <section class="channel-section">
            <div class="section-line-header">
              <div>运行状态</div>
            </div>
            <div class="status-summary">
              <div class="summary-card">
                <span class="summary-label">已启用密钥</span>
                <strong>{{ selectedKeys.filter((item) => item.status === 'enabled').length }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">总调用</span>
                <strong>{{ selectedKeys.reduce((sum, item) => sum + item.total_successes + item.total_failures, 0) }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">失败次数</span>
                <strong>{{ selectedKeys.reduce((sum, item) => sum + item.total_failures, 0) }}</strong>
              </div>
            </div>
          </section>

          <el-collapse v-model="advancedOpen" class="advanced-collapse">
            <el-collapse-item name="advanced">
              <template #title>
                <span class="advanced-title">高级设置</span>
              </template>
              <div class="advanced-form">
                <div class="advanced-field">
                  <div class="advanced-field-label">优先级</div>
                  <el-input-number v-model="providerForm.priority" :min="1" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">权重</div>
                  <el-input-number v-model="providerForm.weight" :min="1" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">请求超时</div>
                  <el-input-number v-model="providerForm.timeout_ms" :min="1000" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">请求结果数</div>
                  <el-input-number v-model="providerRequestLimit" :min="1" :max="50" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">启用缓存</div>
                  <el-switch v-model="providerForm.default_cache_enabled" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">缓存时长</div>
                  <el-input-number v-model="providerForm.cache_ttl_seconds" :min="0" controls-position="right" :disabled="!providerForm.default_cache_enabled" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">使用代理</div>
                  <el-switch :model-value="false" disabled />
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>
      <template #footer>
        <div class="dialog-action-bar">
          <el-button @click="dialogVisible=false">取消</el-button>
          <el-button type="primary" :loading="savingProvider" @click="saveProvider">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { CopyDocument, Hide, Plus, View } from '@element-plus/icons-vue'
import { api, ProviderConfig, ProviderKey } from '../api/client'

type EditableKey = ProviderKey & { isNew?: boolean }
type ProviderCard = ProviderConfig & { keyCount: number; enabledKeyCount: number; totalSuccess: number; totalFailure: number; totalCalls: number }

const providers = ref<ProviderConfig[]>([])
const keys = ref<EditableKey[]>([])
const dialogVisible = ref(false)
const providerForm = ref<ProviderConfig | null>(null)
const savingProvider = ref(false)
const creatingKey = ref(false)
const creatingRow = ref(false)
const draftKey = ref('')
const testingKeyId = ref<number | null>(null)
const visibleKeyIds = ref(new Set<number>())
const advancedOpen = ref<string[]>([])

const providerCards = computed<ProviderCard[]>(() => providers.value.map((provider) => {
  const ownedKeys = keys.value.filter((item) => item.provider_name === provider.name)
  const totalSuccess = ownedKeys.reduce((sum, item) => sum + item.total_successes, 0)
  const totalFailure = ownedKeys.reduce((sum, item) => sum + item.total_failures, 0)
  return { ...provider, keyCount: ownedKeys.length, enabledKeyCount: ownedKeys.filter((item) => item.status === 'enabled').length, totalSuccess, totalFailure, totalCalls: totalSuccess + totalFailure }
}))
const selectedKeys = computed(() => providerForm.value ? keys.value.filter((item) => item.provider_name === providerForm.value?.name) : [])
const tableKeys = computed<EditableKey[]>(() => creatingRow.value ? [{ id: 0, provider_id: 0, provider_name: providerForm.value?.name || '', alias: '', key_hint: '', key: '', status: 'enabled', weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0, max_concurrency: 1, current_failures: 0, total_successes: 0, total_failures: 0, daily_used: 0, monthly_used: 0, created_at: '', updated_at: '', isNew: true }, ...selectedKeys.value] : selectedKeys.value)
const dialogTitle = computed(() => providerForm.value ? `编辑 ${providerForm.value.display_name}` : '编辑平台')
const providerRequestLimit = computed({
  get() {
    const value = providerForm.value?.settings?.request_result_limit
    return typeof value === 'number' ? value : 10
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), request_result_limit: value }
  }
})

function providerShortName(name: string) {
  if (/you/i.test(name)) return 'Y'
  if (/jina/i.test(name)) return 'J'
  if (/exa/i.test(name)) return 'E'
  return (name || 'S').slice(0, 1).toUpperCase()
}



function maskKey(key: string) {
  if (!key) return '-'
  if (key.length <= 16) return key
  return `${key.slice(0, 8)}...${key.slice(-8)}`
}

function successRate(row: EditableKey) {
  const total = row.total_successes + row.total_failures
  return total > 0 ? Math.round((row.total_successes / total) * 100) : 0
}

function toggleKeyVisible(id: number) {
  const next = new Set(visibleKeyIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  visibleKeyIds.value = next
}

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function load() {
  const [providerResult, keyResult] = await Promise.all([api.providers(), api.keys()])
  providers.value = providerResult.providers
  keys.value = keyResult.keys
}

function openProvider(provider: ProviderConfig) {
  const next = { ...JSON.parse(JSON.stringify(provider)), default_cache_enabled: false }
  next.settings = { ...(next.settings || {}), request_result_limit: Number(next.settings?.request_result_limit || 10) }
  providerForm.value = next
  creatingRow.value = false
  draftKey.value = ''
  visibleKeyIds.value = new Set()
  advancedOpen.value = []
  dialogVisible.value = true
}

async function toggleProvider(provider: ProviderConfig, enabled: boolean) {
  const next = { ...provider, enabled }
  await api.updateProvider(next)
  ElMessage.success(enabled ? '平台已启用' : '平台已停用')
  await load()
}

async function saveProvider() {
  if (!providerForm.value) return
  savingProvider.value = true
  try {
    await api.updateProvider(providerForm.value)
    ElMessage.success('平台配置已保存')
    await load()
    dialogVisible.value = false
  } finally {
    savingProvider.value = false
  }
}

function startCreateKey() {
  creatingRow.value = true
  draftKey.value = ''
}

function cancelCreateKey() {
  creatingRow.value = false
  draftKey.value = ''
}

async function createKey() {
  if (!providerForm.value) return
  if (!draftKey.value.trim()) { ElMessage.warning('请填写平台密钥'); return }
  creatingKey.value = true
  try {
    await api.createKey({ provider_name: providerForm.value.name, alias: `${providerForm.value.name}-${Date.now()}`, key: draftKey.value.trim(), weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0, max_concurrency: 1 })
    ElMessage.success('密钥已添加')
    cancelCreateKey()
    await load()
  } finally {
    creatingKey.value = false
  }
}

async function setKeyStatus(row: EditableKey, enabled: boolean) {
  const status = enabled ? 'enabled' : 'disabled'
  await api.updateKey(row.id, { status })
  ElMessage.success(enabled ? '密钥已启用' : '密钥已禁用')
  await load()
}

async function testKey(row: EditableKey) {
  testingKeyId.value = row.id
  try {
    const result = await api.testKey(row.id, { query: 'latest AI search API news', limit: 3 }) as { summary?: { status?: string; latency_ms?: number; result_count?: number; error?: string } }
    const summary = result.summary || {}
    if (summary.status === 'success') {
      ElNotification.success({ title: '测试成功', message: `返回 ${summary.result_count || 0} 条结果，耗时 ${summary.latency_ms || 0} ms`, position: 'top-right' })
    } else {
      ElNotification.error({ title: '测试失败', message: summary.error || '未知错误', position: 'top-right' })
    }
    await load()
  } catch (error) {
    ElNotification.error({ title: '测试失败', message: (error as Error).message, position: 'top-right' })
  } finally {
    testingKeyId.value = null
  }
}

async function removeKey(id: number) {
  await ElMessageBox.confirm('删除后不可恢复，确认删除这个密钥吗？', '删除密钥', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  await api.deleteKey(id)
  ElMessage.success('密钥已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.channel-dialog-body { box-sizing: border-box; overflow: visible; padding: 0 2px 2px; }
.channel-section { margin-bottom: 22px; }
.section-line-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; font-size: 16px; font-weight: 800; color: var(--text); }
.section-line-header :deep(.el-button) { margin-left: 0; }
.base-url-input :deep(.el-input__wrapper), .key-input :deep(.el-input__wrapper) { height: 44px; }
.api-key-list { display: flex; flex-direction: column; gap: 10px; overflow: visible; }
.api-key-row { display: grid; grid-template-columns: minmax(0, 1fr) 38px 38px; align-items: start; column-gap: 6px; width: 100%; min-width: 0; overflow: visible; }
.api-key-row.is-new { grid-template-columns: minmax(0, 1fr) auto; }
.api-key-row > * { min-width: 0; }
.api-key-row > .el-button { justify-self: center; align-self: start; height: 44px; padding: 0 4px; display: inline-flex; align-items: center; justify-content: center; }
.key-main { min-width: 0; overflow: hidden; }
.key-icon-actions { display: inline-flex; align-items: center; gap: 4px; flex-shrink: 0; }
.key-icon-actions :deep(.el-button) { width: 20px; height: 20px; padding: 0; margin-left: 0; font-size: 16px; }
.key-meta { display: flex; align-items: center; gap: 12px; margin-top: 5px; padding-left: 2px; color: var(--muted); font-size: 12px; line-height: 1; }
.key-meta span { white-space: nowrap; }
.new-key-actions { display: flex; justify-content: flex-end; gap: 8px; }
.empty-key-row { height: 44px; display: flex; align-items: center; justify-content: center; color: var(--muted); border: 1px dashed var(--border); border-radius: var(--el-border-radius-base); background: #fff; }
.status-summary { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.summary-card { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; background: #fff; border: 1px solid var(--border); border-radius: var(--el-border-radius-base);  }
.summary-label { color: var(--muted); }
.summary-card strong { color: var(--text); }
.advanced-collapse { margin-bottom: 22px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: #fff; overflow: hidden; }
.advanced-collapse :deep(.el-collapse-item__header) { height: 48px; padding: 0 16px; border-bottom: 1px solid var(--border); font-weight: 800; }
.advanced-collapse :deep(.el-collapse-item__content) { padding: 0; }
.advanced-collapse :deep(.el-collapse-item__wrap) { border-bottom: 0; }
.advanced-title { color: var(--text); }
.advanced-form { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); }
.advanced-field { display: flex; align-items: center; justify-content: space-between; gap: 10px; min-width: 0; min-height: 56px; padding: 10px 16px; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.advanced-field:nth-child(3n) { border-right: 0; }
.advanced-field:nth-last-child(-n + 3) { border-bottom: 0; }
.advanced-field-label { color: var(--text); font-weight: 700; line-height: 1.4; white-space: nowrap; }
.advanced-field :deep(.el-input-number) { width: 112px; flex-shrink: 0; }
.advanced-field :deep(.el-switch) { flex-shrink: 0; }
.dialog-action-bar { display: flex; justify-content: flex-end; gap: 12px; width: 100%; }
.dialog-action-bar :deep(.el-button) { margin-left: 0; }
:deep(.channel-style-dialog .el-dialog) { background: #fbfaf8; }
:deep(.provider-dialog .el-dialog__header) { padding: 20px 26px 8px; margin-right: 0; }
:deep(.provider-dialog .el-dialog__title) { font-size: 24px; font-weight: 900; color: var(--text); }
:deep(.provider-dialog .el-dialog__headerbtn) { top: 20px; right: 24px; font-size: 22px; }
:deep(.provider-dialog .el-dialog__body) { padding: 18px 26px 8px; overflow: visible; }
:deep(.provider-dialog .el-dialog__footer) { padding: 10px 26px 24px; }
</style>
