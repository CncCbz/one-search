<template>
  <div>
    <div class="page-actions">
      <el-button @click="load">刷新</el-button>
    </div>

    <div class="provider-grid">
      <el-card v-for="card in providerCards" :key="card.name" class="soft-card provider-card" shadow="never"
        @click="openProvider(card)">
        <div class="provider-card-header">
          <div style="display:flex;align-items:center;gap:12px;min-width:0">
            <div class="provider-icon">{{ providerShortName(card.display_name || card.name) }}</div>
            <div style="min-width:0">
              <div style="font-size:18px;font-weight:800;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{
                card.display_name }}</div>
              <div class="muted" style="font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{
                card.base_url }}</div>
            </div>
          </div>
          <el-switch :model-value="card.enabled" @click.stop @change="toggleProvider(card, Boolean($event))" />
        </div>

        <div class="metric-row">
          <div class="metric-pill">
            <div class="label">绑定密钥</div>
            <div class="value">{{ card.enabledKeyCount }}/{{ card.keyCount }}</div>
          </div>
          <div class="metric-pill">
            <div class="label">累计调用</div>
            <div class="value">{{ card.totalCalls }}</div>
          </div>
          <div class="metric-pill">
            <div class="label">成功</div>
            <div class="value" style="color:var(--primary)">{{ card.totalSuccess }}</div>
          </div>
          <div class="metric-pill">
            <div class="label">失败</div>
            <div class="value" style="color:var(--danger)">{{ card.totalFailure }}</div>
          </div>
        </div>

        <div style="display:flex;align-items:center;margin-top:14px">
          <span class="muted" style="font-size:12px">超时 {{ card.timeout_ms }} ms · 缓存 {{ card.default_cache_enabled ?
            card.cache_ttl_seconds + ' 秒' : '关闭' }}</span>
        </div>
      </el-card>
    </div>

    <el-dialog v-model="dialogVisible" width="760px" top="3vh" :title="dialogTitle" destroy-on-close
      class="provider-dialog channel-style-dialog">
      <template v-if="providerForm">
        <div class="channel-dialog-body">
          <section class="channel-section">
            <div class="section-line-header">
              <div>基础 URL</div>
              <el-tooltip content="复制基础 URL" placement="top">
                <el-button link :icon="CopyDocument" aria-label="复制基础 URL" @click="copyText(providerForm.base_url)" />
              </el-tooltip>
            </div>
            <el-input v-model="providerForm.base_url" class="base-url-input" placeholder="https://example.com" />
          </section>

          <section class="channel-section">
            <div class="section-line-header">
              <div>API 密钥 ({{ selectedKeys.length }})</div>
              <el-tooltip content="添加密钥" placement="top">
                <el-button link :icon="Plus" :disabled="creatingRow" aria-label="添加密钥" @click="startCreateKey" />
              </el-tooltip>
            </div>
            <div class="api-key-list">
              <div v-for="row in tableKeys" :key="row.isNew ? 'new-key' : row.id" class="api-key-row"
                :class="{ 'is-new': row.isNew }">
                <template v-if="row.isNew">
                  <div class="new-key-fields">
                    <el-input v-model="draftKey" class="key-input" type="password" show-password
                      placeholder="API Key" />
                    <el-input v-if="providerForm?.name === 'exa'" v-model="draftExaServiceKey" class="key-input"
                      type="password" show-password placeholder="Exa x-api-key / 管理密钥" />
                  </div>
                  <div class="new-key-actions">
                    <el-tooltip content="保存密钥" placement="top">
                      <el-button link type="primary" :icon="Check" :loading="creatingKey" aria-label="保存密钥"
                        @click="createKey" />
                    </el-tooltip>
                    <el-tooltip content="取消添加" placement="top">
                      <el-button link :icon="Close" aria-label="取消添加" @click="cancelCreateKey" />
                    </el-tooltip>
                  </div>
                </template>
                <template v-else>
                  <div class="key-main">
                    <el-input class="key-input" :model-value="visibleKeyIds.has(row.id) ? row.key : maskKey(row.key)"
                      readonly>
                      <template #suffix>
                        <div class="key-icon-actions">
                          <el-tooltip :content="visibleKeyIds.has(row.id) ? '隐藏密钥' : '显示密钥'" placement="top">
                            <el-button link :icon="visibleKeyIds.has(row.id) ? Hide : View" aria-label="显示或隐藏密钥"
                              @click="toggleKeyVisible(row.id)" />
                          </el-tooltip>
                        </div>
                      </template>
                    </el-input>
                    <div class="key-meta">
                      <span>成功 {{ row.total_successes }}</span>
                      <span>失败 {{ row.total_failures }}</span>
                      <span>成功率 {{ successRate(row) }}%</span>
                      <span :class="quotaMetaClass(row)">{{ quotaMetaText(row) }}</span>
                      <el-tooltip v-if="row.status !== 'enabled'" :content="keyDisabledReason(row)" placement="top">
                        <span class="key-status-reason" aria-label="停用原因"><el-icon>
                            <WarnTriangleFilled />
                          </el-icon></span>
                      </el-tooltip>
                      <span v-if="row.provider_name === 'exa' && row.exa_service_key_hint">x-api-key {{
                        row.exa_service_key_hint }}</span>
                    </div>
                  </div>
                  <el-tooltip content="测试密钥" placement="top">
                    <el-button link class="row-icon-button" type="primary" :icon="Refresh"
                      :loading="testingKeyId === row.id" aria-label="测试密钥" @click="testKey(row)" />
                  </el-tooltip>
                  <el-tooltip content="查询官方额度" placement="top">
                    <el-button link class="row-icon-button" :icon="Clock" :loading="quotaLoadingKeyId === row.id"
                      aria-label="查询官方额度" @click="queryQuota(row)" />
                  </el-tooltip>
                  <el-tooltip :content="row.status === 'enabled' ? '停用密钥' : '启用密钥'" placement="top">
                    <el-button link class="row-icon-button" :type="row.status === 'enabled' ? 'warning' : 'success'"
                      :icon="row.status === 'enabled' ? Remove : CircleCheck"
                      :aria-label="row.status === 'enabled' ? '停用密钥' : '启用密钥'"
                      @click="setKeyStatus(row, row.status !== 'enabled')" />
                  </el-tooltip>
                  <el-tooltip content="删除密钥" placement="top">
                    <el-button link class="row-icon-button" type="danger" :icon="Delete" aria-label="删除密钥"
                      @click="removeKey(row.id)" />
                  </el-tooltip>
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
                <strong>{{selectedKeys.filter((item) => item.status === 'enabled').length}}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">总调用</span>
                <strong>{{selectedKeys.reduce((sum, item) => sum + item.total_successes + item.total_failures, 0)
                  }}</strong>
              </div>
              <div class="summary-card">
                <span class="summary-label">失败次数</span>
                <strong>{{selectedKeys.reduce((sum, item) => sum + item.total_failures, 0)}}</strong>
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
                  <div class="advanced-field-label">换 key 重试</div>
                  <el-input-number v-model="providerKeyRetryCount" :min="0" :max="20" controls-position="right" />
                </div>
                <div class="advanced-field advanced-field-wide">
                  <div class="advanced-field-label">Key 路由策略</div>
                  <el-select v-model="providerKeyRoutingStrategy">
                    <el-option value="" label="权重优先轮询" />
                    <el-option value="least_used" label="最少使用优先" />
                    <el-option value="random" label="随机" />
                    <el-option value="weighted_random" label="按权重随机" />
                  </el-select>
                </div>
                <div class="advanced-field advanced-field-wide">
                  <div class="advanced-field-label">可重试错误</div>
                  <el-select v-model="providerRetryErrorTypes" multiple collapse-tags collapse-tags-tooltip>
                    <el-option label="鉴权失败" value="auth" />
                    <el-option label="额度耗尽" value="quota_exhausted" />
                    <el-option label="限流" value="rate_limited" />
                    <el-option label="上游错误" value="upstream" />
                    <el-option label="响应异常" value="invalid_response" />
                  </el-select>
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">启用缓存</div>
                  <el-switch v-model="providerForm.default_cache_enabled" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">缓存时长</div>
                  <el-input-number v-model="providerForm.cache_ttl_seconds" :min="0" controls-position="right"
                    :disabled="!providerForm.default_cache_enabled" />
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
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="savingProvider" @click="saveProvider">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { Check, CircleCheck, Clock, Close, CopyDocument, Delete, Hide, Plus, Refresh, Remove, View, WarnTriangleFilled } from '@element-plus/icons-vue'
import { api, OfficialQuotaResult, ProviderConfig, ProviderKey } from '../api/client'

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
const draftExaServiceKey = ref('')
const testingKeyId = ref<number | null>(null)
const quotaLoadingKeyId = ref<number | null>(null)
const visibleKeyIds = ref(new Set<number>())
const advancedOpen = ref<string[]>([])

const providerCards = computed<ProviderCard[]>(() => providers.value.map((provider) => {
  const ownedKeys = keys.value.filter((item) => item.provider_name === provider.name)
  const totalSuccess = ownedKeys.reduce((sum, item) => sum + item.total_successes, 0)
  const totalFailure = ownedKeys.reduce((sum, item) => sum + item.total_failures, 0)
  return { ...provider, keyCount: ownedKeys.length, enabledKeyCount: ownedKeys.filter((item) => item.status === 'enabled').length, totalSuccess, totalFailure, totalCalls: totalSuccess + totalFailure }
}))
const selectedKeys = computed(() => providerForm.value ? keys.value.filter((item) => item.provider_name === providerForm.value?.name) : [])
const tableKeys = computed<EditableKey[]>(() => creatingRow.value ? [{ id: 0, provider_id: 0, provider_name: providerForm.value?.name || '', alias: '', key_hint: '', key: '', exa_service_key_hint: '', status: 'enabled', weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0, max_concurrency: 1, current_failures: 0, total_successes: 0, total_failures: 0, daily_used: 0, monthly_used: 0, official_quota_status: '', official_quota_message: '', official_quota_unit: '', created_at: '', updated_at: '', isNew: true }, ...selectedKeys.value] : selectedKeys.value)
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
const providerKeyRetryCount = computed({
  get() {
    const value = providerForm.value?.settings?.key_retry_count
    return typeof value === 'number' ? value : 3
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), key_retry_count: value }
  }
})
const providerRetryErrorTypes = computed<string[]>({
  get() {
    const value = providerForm.value?.settings?.retry_error_types
    if (Array.isArray(value)) return value.filter((item): item is string => typeof item === 'string')
    return ['auth', 'quota_exhausted', 'rate_limited']
  },
  set(value: string[]) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), retry_error_types: value }
  }
})
const providerKeyRoutingStrategy = computed<string>({
  get() {
    const value = providerForm.value?.settings?.key_routing_strategy
    return typeof value === 'string' ? value : ''
  },
  set(value: string) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), key_routing_strategy: value }
  }
})

function providerShortName(name: string) {
  if (/you/i.test(name)) return 'Y'
  if (/jina/i.test(name)) return 'J'
  if (/exa/i.test(name)) return 'E'
  return (name || 'S').slice(0, 1).toUpperCase()
}

function formatNumber(value?: number) {
  if (value === undefined || value === null || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value)
}

function formatCurrency(value?: number) {
  if (value === undefined || value === null || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value)
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

function quotaMetaText(row: EditableKey) {
  if (!row.official_quota_checked_at) return '额度 未同步'
  if (row.official_quota_status !== 'success') return `额度 ${row.official_quota_message || '同步失败'}`
  if (row.provider_name === 'you') return `额度 ${formatCurrency(row.official_quota_balance_usd)}`
  if (row.provider_name === 'jina') return `额度 ${formatNumber(row.official_quota_balance)} tokens`
  if (row.provider_name === 'exa') return `用量 ${formatCurrency(row.official_quota_used_usd)}`
  return `额度 ${formatNumber(row.official_quota_balance)}`
}

function quotaMetaClass(row: EditableKey) {
  if (quotaValue(row) < 0) return 'quota-meta is-danger'
  return row.official_quota_status === 'success' ? 'quota-meta is-success' : 'quota-meta is-muted'
}

function quotaValue(row: EditableKey) {
  if (row.provider_name === 'you') return row.official_quota_balance_usd ?? row.official_quota_balance ?? 0
  if (row.provider_name === 'jina') return row.official_quota_balance ?? 0
  return row.official_quota_balance ?? 0
}

function keyDisabledReason(row: EditableKey) {
  if (row.status === 'exhausted') return row.official_quota_message || '额度不足或已耗尽'
  if (row.status === 'cooling') return row.cooldown_until ? `限流冷却至 ${formatTime(row.cooldown_until)}` : '限流冷却中'
  if (row.status === 'disabled') return row.official_quota_message || '手动停用或鉴权失败'
  return row.official_quota_message || row.status
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
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
  next.settings = { ...(next.settings || {}), request_result_limit: Number(next.settings?.request_result_limit || 10), key_retry_count: Number(next.settings?.key_retry_count ?? 3) }
  providerForm.value = next
  creatingRow.value = false
  draftKey.value = ''
  draftExaServiceKey.value = ''
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
  draftExaServiceKey.value = ''
}

function cancelCreateKey() {
  creatingRow.value = false
  draftKey.value = ''
  draftExaServiceKey.value = ''
}

async function createKey() {
  if (!providerForm.value) return
  if (!draftKey.value.trim()) { ElMessage.warning('请填写平台密钥'); return }
  if (providerForm.value.name === 'exa' && !draftExaServiceKey.value.trim()) { ElMessage.warning('请填写 Exa x-api-key'); return }
  creatingKey.value = true
  try {
    await api.createKey({ provider_name: providerForm.value.name, alias: `${providerForm.value.name}-${Date.now()}`, key: draftKey.value.trim(), exa_service_key: draftExaServiceKey.value.trim(), weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0, max_concurrency: 1 })
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

async function queryQuota(row: EditableKey) {
  quotaLoadingKeyId.value = row.id
  try {
    const quota = await api.queryKeyQuota(row.id)
    applyQuotaResult(row.id, quota)
    if (quota.supported && quota.status === 'success') {
      ElMessage.success('官方额度已更新')
    } else {
      ElMessage.warning(quota.message || '该渠道暂不支持官方额度查询')
    }
    await load()
  } catch (error) {
    ElNotification.error({ title: '额度查询失败', message: (error as Error).message, position: 'top-right' })
  } finally {
    quotaLoadingKeyId.value = null
  }
}

function applyQuotaResult(id: number, quota: OfficialQuotaResult) {
  keys.value = keys.value.map((item) => item.id === id ? {
    ...item,
    official_quota_status: quota.status,
    official_quota_message: quota.message || '',
    official_quota_unit: quota.unit || '',
    official_quota_balance: quota.balance,
    official_quota_balance_usd: quota.balance_usd,
    official_quota_used_usd: quota.total_cost_usd,
    official_quota_total_quantity: quota.total_quantity,
    official_quota_account_id: quota.account_id || '',
    official_quota_checked_at: quota.fetched_at
  } : item)
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
.channel-dialog-body {
  box-sizing: border-box;
  overflow: visible;
  padding: 0 2px 2px;
}

.channel-section {
  margin-bottom: 22px;
}

.section-line-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  font-size: 16px;
  font-weight: 800;
  color: var(--text);
}

.section-line-header :deep(.el-button) {
  margin-left: 0;
}

.base-url-input :deep(.el-input__wrapper),
.key-input :deep(.el-input__wrapper) {
  height: 44px;
}

.api-key-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: visible;
}

.api-key-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) repeat(4, 26px);
  align-items: start;
  column-gap: 0;
  width: 100%;
  min-width: 0;
  overflow: visible;
}

.api-key-row.is-new {
  grid-template-columns: minmax(0, 1fr) auto;
}

.new-key-fields {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.api-key-row>* {
  min-width: 0;
}

.api-key-row .row-icon-button {
  justify-self: center;
  align-self: start;
  width: 26px;
  height: 44px;
  padding: 0;
  margin-left: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.key-main {
  min-width: 0;
  overflow: hidden;
}

.key-icon-actions {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.key-icon-actions :deep(.el-button) {
  width: 20px;
  height: 20px;
  padding: 0;
  margin-left: 0;
  font-size: 16px;
}

.key-status-reason {
  display: inline-flex;
  align-items: center;
  color: var(--danger);
  font-size: 14px;
  line-height: 1;
}

.key-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 5px;
  padding-left: 2px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1;
  flex-wrap: wrap;
}

.key-meta span {
  white-space: nowrap;
}

.quota-meta.is-success {
  color: var(--primary);
}

.quota-meta.is-muted,
.quota-meta.is-danger {
  color: var(--danger);
}

.new-key-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

.new-key-actions :deep(.el-button) {
  width: 28px;
  height: 28px;
  padding: 0;
  margin-left: 0;
}

.empty-key-row {
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: var(--el-border-radius-base);
  background: #fff;
}

.status-summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.summary-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  background: #fff;
  border: 1px solid var(--border);
  border-radius: var(--el-border-radius-base);
}

.summary-label {
  color: var(--muted);
}

.summary-card strong {
  color: var(--text);
}

.advanced-collapse {
  margin-bottom: 22px;
  border: 1px solid var(--border);
  border-radius: var(--el-border-radius-base);
  background: #fff;
  overflow: hidden;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  font-weight: 800;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding: 0;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
}

.advanced-title {
  color: var(--text);
}

.advanced-form {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.advanced-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  min-height: 56px;
  padding: 10px 16px;
  border-right: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

.advanced-field:nth-child(3n) {
  border-right: 0;
}

.advanced-field:nth-last-child(-n + 3) {
  border-bottom: 0;
}

.advanced-field-label {
  color: var(--text);
  font-weight: 700;
  line-height: 1.4;
  white-space: nowrap;
}

.advanced-field :deep(.el-input-number) {
  width: 112px;
  flex-shrink: 0;
}

.advanced-field :deep(.el-select) {
  flex: 1;
  min-width: 180px;
}

.advanced-field-wide {
  grid-column: span 2;
}

.advanced-field :deep(.el-switch) {
  flex-shrink: 0;
}

.dialog-action-bar {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  width: 100%;
}

.dialog-action-bar :deep(.el-button) {
  margin-left: 0;
}

:deep(.channel-style-dialog .el-dialog) {
  background: #fbfaf8;
}

:deep(.provider-dialog .el-dialog__header) {
  padding: 20px 26px 8px;
  margin-right: 0;
}

:deep(.provider-dialog .el-dialog__title) {
  font-size: 24px;
  font-weight: 900;
  color: var(--text);
}

:deep(.provider-dialog .el-dialog__headerbtn) {
  top: 20px;
  right: 24px;
  font-size: 22px;
}

:deep(.provider-dialog .el-dialog__body) {
  padding: 18px 26px 8px;
  overflow: visible;
}

:deep(.provider-dialog .el-dialog__footer) {
  padding: 10px 26px 24px;
}
</style>
