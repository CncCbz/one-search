<template>
  <div>
    <div class="page-hd">
      <h1>平台管理</h1>
      <div class="page-actions">
        <el-button :icon="Refresh" circle title="刷新" :loading="loading" @click="load" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="cards" />
    <div v-else class="provider-grid" v-loading="loading">
      <el-card v-for="card in providerCards" :key="card.name" class="soft-card provider-card" shadow="never"
        @click="openProvider(card)">
        <div class="provider-card-header">
          <div style="display:flex;align-items:center;gap:12px;min-width:0">
            <div class="provider-icon">{{ providerShortName(card.display_name || card.name) }}</div>
            <div style="min-width:0">
              <div style="font-size:18px;font-weight:800;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">{{
                card.display_name }}</div>
              <div class="muted" style="font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis" :title="card.base_url">{{
                hostOf(card.base_url) }}</div>
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
          <span class="muted" style="font-size:12px">超时 {{ card.timeout_ms }} ms</span>
        </div>
      </el-card>
    </div>

    <el-dialog
      v-model="dialogVisible"
      width="860px"
      top="3vh"
      destroy-on-close
      class="provider-dialog channel-style-dialog provider-tabs-dialog"
      :show-close="true"
    >
      <template #header>
        <div v-if="providerForm" class="dlg-ident">
          <div class="dlg-logo">{{ providerShortName(providerForm.display_name || providerForm.name) }}</div>
          <div class="dlg-ident-main">
            <div class="dlg-title">{{ dialogTitle }}</div>
            <div class="dlg-sub muted">provider · {{ providerForm.name }}</div>
            <div class="dlg-badges">
              <span class="dlg-badge" :class="providerForm.enabled ? 'ok' : 'mute'">{{ providerForm.enabled ? '已启用' : '已停用' }}</span>
              <span class="dlg-badge">{{ selectedKeys.filter((item) => item.status === 'enabled').length }} / {{ selectedKeys.length }} 密钥可用</span>
              <span class="dlg-badge warn">估算 {{ formatCurrency(providerPricePerRequest) }} / req</span>
            </div>
          </div>
        </div>
      </template>

      <template v-if="providerForm">
        <div class="channel-dialog-body">
          <el-tabs v-model="activeTab" class="provider-tabs">
            <el-tab-pane label="密钥" name="keys">
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

              <section class="channel-section compact-bottom">
                <div class="section-line-header">
                  <div>API 密钥 ({{ selectedKeys.length }})</div>
                  <el-tooltip content="添加密钥" placement="top">
                    <el-button link :icon="Plus" :disabled="creatingRow" aria-label="添加密钥" @click="startCreateKey" />
                  </el-tooltip>
                </div>
                <div class="api-key-list">
                  <div v-for="row in tableKeys" :key="row.isNew ? 'new-key' : row.id" class="api-key-row" :class="{ 'is-new': row.isNew }">
                    <template v-if="row.isNew">
                      <div class="new-key-fields">
                        <el-input v-model="draftKey" class="key-input" type="password" show-password placeholder="API Key" />
                        <el-input
                          v-if="providerForm?.name === 'exa'"
                          v-model="draftExaServiceKey"
                          class="key-input"
                          type="password"
                          show-password
                          placeholder="Exa x-api-key / 管理密钥"
                        />
                      </div>
                      <div class="new-key-actions">
                        <el-tooltip content="保存密钥" placement="top">
                          <el-button link type="primary" :icon="Check" :loading="creatingKey" aria-label="保存密钥" @click="createKey" />
                        </el-tooltip>
                        <el-tooltip content="取消添加" placement="top">
                          <el-button link :icon="Close" aria-label="取消添加" @click="cancelCreateKey" />
                        </el-tooltip>
                      </div>
                    </template>
                    <template v-else>
                      <div class="key-main">
                        <el-input class="key-input" :model-value="row.key_hint || '已保存密钥'" readonly />
                        <div class="key-meta">
                          <span>成功 {{ row.total_successes }}</span>
                          <span>失败 {{ row.total_failures }}</span>
                          <span>成功率 {{ successRate(row) }}%</span>
                          <span :class="quotaMetaClass(row)">{{ quotaMetaText(row) }}</span>
                          <el-tooltip v-if="row.status !== 'enabled'" :content="keyDisabledReason(row)" placement="top">
                            <span class="key-status-reason" aria-label="停用原因">
                              <el-icon><WarnTriangleFilled /></el-icon>
                            </span>
                          </el-tooltip>
                          <span v-if="row.provider_name === 'exa' && row.exa_service_key_hint">x-api-key {{ row.exa_service_key_hint }}</span>
                        </div>
                      </div>
                      <el-tooltip content="复制密钥" placement="top">
                        <el-button link class="row-icon-button" :icon="CopyDocument" :loading="copyingKeyId === row.id" aria-label="复制密钥" @click="copyKey(row)" />
                      </el-tooltip>
                      <el-tooltip content="测试密钥" placement="top">
                        <el-button link class="row-icon-button" type="primary" :icon="Refresh" :loading="testingKeyId === row.id" aria-label="测试密钥" @click="testKey(row)" />
                      </el-tooltip>
                      <el-tooltip content="查询官方额度" placement="top">
                        <el-button link class="row-icon-button" :icon="Clock" :loading="quotaLoadingKeyId === row.id" aria-label="查询官方额度" @click="queryQuota(row)" />
                      </el-tooltip>
                      <el-tooltip :content="row.status === 'enabled' ? '停用密钥' : '启用密钥'" placement="top">
                        <el-button
                          link
                          class="row-icon-button"
                          :type="row.status === 'enabled' ? 'warning' : 'success'"
                          :icon="row.status === 'enabled' ? Remove : CircleCheck"
                          :aria-label="row.status === 'enabled' ? '停用密钥' : '启用密钥'"
                          @click="setKeyStatus(row, row.status !== 'enabled')"
                        />
                      </el-tooltip>
                      <el-tooltip content="删除密钥" placement="top">
                        <el-button link class="row-icon-button" type="danger" :icon="Delete" aria-label="删除密钥" @click="removeKey(row.id)" />
                      </el-tooltip>
                    </template>
                  </div>
                  <div v-if="tableKeys.length === 0" class="empty-key-row">暂无密钥，点击右侧“添加”创建</div>
                </div>
              </section>
            </el-tab-pane>

            <el-tab-pane label="计费" name="billing">
              <section class="price-card">
                <div class="price-card-hd">
                  <div>
                    <h3>成本估算单价</h3>
                    <p>仅用于仪表盘成本估算，不是官方账单。0 表示回退内置公开价目表。</p>
                  </div>
                  <span class="dlg-badge warn">非官方账单</span>
                </div>
                <div class="advanced-form billing-form">
                  <div class="advanced-field">
                    <div class="advanced-field-label">单价 / 请求 USD</div>
                    <el-input-number v-model="providerPricePerRequest" :min="0" :step="0.001" :controls="true" controls-position="right" />
                  </div>
                  <div class="advanced-field">
                    <div class="advanced-field-label">单价 / Credit USD</div>
                    <el-input-number v-model="providerPricePerCredit" :min="0" :step="0.001" controls-position="right" />
                  </div>
                  <div class="advanced-field">
                    <div class="advanced-field-label">单价 / Token USD</div>
                    <el-input-number v-model="providerPricePerToken" :min="0" :step="0.00000001" controls-position="right" />
                  </div>
                  <div class="advanced-field">
                    <div class="advanced-field-label">默认计费 Credits</div>
                    <el-tooltip content="上游未返回 usage 时，一次成功搜索默认记多少 credits；0 表示不补 credits" placement="top">
                      <el-input-number v-model="providerDefaultBillableCredits" :min="0" :step="1" controls-position="right" />
                    </el-tooltip>
                  </div>
                </div>
              </section>
              <div class="price-sample-grid">
                <div class="summary-card sample-card">
                  <span class="summary-label">估算样例</span>
                  <strong>{{ formatCurrency(providerPricePerRequest * 100) }}</strong>
                  <small class="muted">100 次成功请求 × 当前单价</small>
                </div>
                <div class="summary-card sample-card">
                  <span class="summary-label">生效范围</span>
                  <div class="sample-tags">
                    <span class="dlg-badge">仅新请求</span>
                    <span class="dlg-badge">成功 call 才计</span>
                    <span class="dlg-badge">可随时改</span>
                  </div>
                </div>
              </div>
            </el-tab-pane>

            <el-tab-pane label="运行" name="runtime">
              <div class="advanced-form runtime-form">
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
                  <el-input-number v-model="providerRequestLimit" :min="1" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">换 key 重试</div>
                  <el-input-number v-model="providerKeyRetryCount" :min="0" :max="20" controls-position="right" />
                </div>
                <div class="advanced-field">
                  <div class="advanced-field-label">渠道并发</div>
                  <el-tooltip content="0 表示不限，正数表示该渠道最大并发请求数" placement="top">
                    <el-input-number v-model="providerMaxConcurrency" :min="0" :max="999" controls-position="right" />
                  </el-tooltip>
                </div>
                <div class="advanced-field advanced-field-wide">
                  <div class="advanced-field-label">Key 路由策略</div>
                  <el-select v-model="providerKeyRoutingStrategy" placeholder="选择策略">
                    <el-option value="round_robin" label="权重优先轮询" />
                    <el-option value="least_used" label="最少使用优先" />
                    <el-option value="random" label="随机" />
                    <el-option value="weighted_random" label="按权重随机" />
                  </el-select>
                </div>
              </div>
            </el-tab-pane>

            <el-tab-pane label="高级" name="advanced">
              <div class="advanced-form advanced-only-form">
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
                  <div class="advanced-field-label">使用代理</div>
                  <el-switch v-model="providerProxyEnabled" />
                </div>
                <div class="advanced-field advanced-field-wide">
                  <div class="advanced-field-label">代理地址</div>
                  <el-input v-model="providerProxyURL" :disabled="!providerProxyEnabled" placeholder="http://127.0.0.1:7897" />
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </template>

      <template #footer>
        <div class="dialog-action-bar">
          <span class="footer-hint muted">保存后立即影响新请求的路由与成本估算</span>
          <div class="footer-actions">
            <el-button @click="dialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="savingProvider" @click="saveProvider">保存</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import PageSkeleton from '../components/PageSkeleton.vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { Check, CircleCheck, Clock, Close, CopyDocument, Delete, Plus, Refresh, Remove, WarnTriangleFilled } from '@element-plus/icons-vue'
import { api, OfficialQuotaResult, ProviderConfig, ProviderKey } from '../api/client'

type EditableKey = ProviderKey & { isNew?: boolean }
type ProviderCard = ProviderConfig & { keyCount: number; enabledKeyCount: number; totalSuccess: number; totalFailure: number; totalCalls: number }

const loading = ref(true)
const loaded = ref(false)
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
const copyingKeyId = ref<number | null>(null)
const activeTab = ref<'keys' | 'billing' | 'runtime' | 'advanced'>('keys')

const providerCards = computed<ProviderCard[]>(() => providers.value.map((provider) => {
  const ownedKeys = keys.value.filter((item) => item.provider_name === provider.name)
  const totalSuccess = ownedKeys.reduce((sum, item) => sum + item.total_successes, 0)
  const totalFailure = ownedKeys.reduce((sum, item) => sum + item.total_failures, 0)
  return { ...provider, keyCount: ownedKeys.length, enabledKeyCount: ownedKeys.filter((item) => item.status === 'enabled').length, totalSuccess, totalFailure, totalCalls: totalSuccess + totalFailure }
}))
const selectedKeys = computed(() => providerForm.value ? keys.value.filter((item) => item.provider_name === providerForm.value?.name) : [])
const tableKeys = computed<EditableKey[]>(() => creatingRow.value ? [{ id: 0, provider_id: 0, provider_name: providerForm.value?.name || '', alias: '', key_hint: '', exa_service_key_hint: '', status: 'enabled', weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0, max_concurrency: 0, current_failures: 0, total_successes: 0, total_failures: 0, daily_used: 0, monthly_used: 0, official_quota_status: '', official_quota_message: '', official_quota_unit: '', created_at: '', updated_at: '', isNew: true }, ...selectedKeys.value] : selectedKeys.value)
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
const providerMaxConcurrency = computed({
  get() {
    return normalizeMaxConcurrency(providerForm.value?.settings?.max_concurrency)
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), max_concurrency: normalizeMaxConcurrency(value) }
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
    // empty / unknown = backend default weighted round-robin
    if (typeof value !== 'string' || !value || value === 'weighted' || value === 'default') return 'round_robin'
    return value
  },
  set(value: string) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), key_routing_strategy: value || 'round_robin' }
  }
})
const providerProxyEnabled = computed<boolean>({
  get() {
    return Boolean(providerForm.value?.settings?.proxy_enabled)
  },
  set(value: boolean) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), proxy_enabled: value }
  }
})

const DEFAULT_PROVIDER_PRICING: Record<string, { price_per_request: number; price_per_credit: number; price_per_token: number; default_billable_credits: number }> = {
  exa: { price_per_request: 0.007, price_per_credit: 0, price_per_token: 0, default_billable_credits: 0 },
  you: { price_per_request: 0.005, price_per_credit: 0, price_per_token: 0, default_billable_credits: 0 },
  tavily: { price_per_request: 0.008, price_per_credit: 0.008, price_per_token: 0, default_billable_credits: 1 },
  serper: { price_per_request: 0.001, price_per_credit: 0.001, price_per_token: 0, default_billable_credits: 1 },
  brave: { price_per_request: 0.005, price_per_credit: 0, price_per_token: 0, default_billable_credits: 0 },
  firecrawl: { price_per_request: 0.00166, price_per_credit: 0.00083, price_per_token: 0, default_billable_credits: 2 },
  jina: { price_per_request: 0.0005, price_per_credit: 0, price_per_token: 0.00000005, default_billable_credits: 0 }
}

function defaultPricingFor(name: string) {
  return DEFAULT_PROVIDER_PRICING[name] || { price_per_request: 0, price_per_credit: 0, price_per_token: 0, default_billable_credits: 0 }
}

function numberSetting(settings: Record<string, unknown> | undefined, key: string, fallback = 0) {
  const value = settings?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}
const providerProxyURL = computed<string>({
  get() {
    const value = providerForm.value?.settings?.proxy_url
    return typeof value === 'string' ? value : ''
  },
  set(value: string) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), proxy_url: value.trim() }
  }
})
const providerPricePerRequest = computed<number>({
  get() {
    const defaults = defaultPricingFor(providerForm.value?.name || '')
    return numberSetting(providerForm.value?.settings, 'price_per_request', defaults.price_per_request)
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), price_per_request: Math.max(0, Number(value) || 0) }
  }
})
const providerPricePerCredit = computed<number>({
  get() {
    const defaults = defaultPricingFor(providerForm.value?.name || '')
    return numberSetting(providerForm.value?.settings, 'price_per_credit', defaults.price_per_credit)
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), price_per_credit: Math.max(0, Number(value) || 0) }
  }
})
const providerPricePerToken = computed<number>({
  get() {
    const defaults = defaultPricingFor(providerForm.value?.name || '')
    return numberSetting(providerForm.value?.settings, 'price_per_token', defaults.price_per_token)
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), price_per_token: Math.max(0, Number(value) || 0) }
  }
})
const providerDefaultBillableCredits = computed<number>({
  get() {
    const defaults = defaultPricingFor(providerForm.value?.name || '')
    return numberSetting(providerForm.value?.settings, 'default_billable_credits', defaults.default_billable_credits)
  },
  set(value: number) {
    if (!providerForm.value) return
    providerForm.value.settings = { ...(providerForm.value.settings || {}), default_billable_credits: Math.max(0, Number(value) || 0) }
  }
})

function providerShortName(name: string) {
  if (/you/i.test(name)) return 'Y'
  if (/jina/i.test(name)) return 'J'
  if (/exa/i.test(name)) return 'E'
  if (/tavily/i.test(name)) return 'T'
  if (/firecrawl/i.test(name)) return 'F'
  if (/serper/i.test(name)) return 'S'
  if (/brave/i.test(name)) return 'B'
  return (name || 'S').slice(0, 1).toUpperCase()
}

function hostOf(url: string) {
  try {
    return new URL(url).host || url
  } catch {
    return url.replace(/^https?:\/\//, '').split('/')[0] || url
  }
}

function formatNumber(value?: number) {
  if (value === undefined || value === null || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value)
}

function formatCurrency(value?: number) {
  if (value === undefined || value === null || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value)
}

function successRate(row: EditableKey) {
  const total = row.total_successes + row.total_failures
  return total > 0 ? Math.round((row.total_successes / total) * 100) : 0
}

function normalizeMaxConcurrency(value: unknown) {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || numeric < 0) return 0
  return Math.trunc(numeric)
}

function quotaMetaText(row: EditableKey) {
  if (!row.official_quota_checked_at) return '额度 未同步'
  if (row.official_quota_status !== 'success') return `额度 ${row.official_quota_message || '同步失败'}`
  if (row.provider_name === 'you') return `额度 ${formatCurrency(row.official_quota_balance_usd)}`
  if (row.provider_name === 'jina') return `额度 ${formatNumber(row.official_quota_balance)} tokens`
  if (row.provider_name === 'exa') return `用量 ${formatCurrency(row.official_quota_used_usd)}`
  if (row.provider_name === 'tavily' || row.provider_name === 'firecrawl' || row.provider_name === 'serper') return `额度 ${formatNumber(row.official_quota_balance)} credits`
  if (row.provider_name === 'brave') return `额度 ${formatNumber(row.official_quota_balance)} requests`
  return `额度 ${formatNumber(row.official_quota_balance)}`
}

function quotaMetaClass(row: EditableKey) {
  if (quotaValue(row) < 0) return 'quota-meta is-danger'
  return row.official_quota_status === 'success' ? 'quota-meta is-success' : 'quota-meta is-muted'
}

function quotaValue(row: EditableKey) {
  if (row.provider_name === 'you') return row.official_quota_balance_usd ?? row.official_quota_balance ?? 0
  if (row.provider_name === 'jina' || row.provider_name === 'tavily' || row.provider_name === 'firecrawl' || row.provider_name === 'serper' || row.provider_name === 'brave') return row.official_quota_balance ?? 0
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

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function copyKey(row: EditableKey) {
  if (!row.id) return
  copyingKeyId.value = row.id
  try {
    const secret = await api.revealKey(row.id)
    if (!secret.key) {
      ElMessage.warning('未找到可复制的密钥')
      return
    }
    await navigator.clipboard.writeText(secret.key)
    ElMessage.success('密钥已复制')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '复制失败')
  } finally {
    copyingKeyId.value = null
  }
}

async function load() {
  loading.value = true
  try {
    const [providerResult, keyResult] = await Promise.all([api.providers(), api.keys()])
    providers.value = providerResult.providers
    keys.value = keyResult.keys
    loaded.value = true
  } finally {
    loading.value = false
  }
}

function openProvider(provider: ProviderConfig) {
  const next = JSON.parse(JSON.stringify(provider))
  const pricing = defaultPricingFor(next.name)
  next.settings = {
    ...(next.settings || {}),
    request_result_limit: Number(next.settings?.request_result_limit || 10),
    key_retry_count: Number(next.settings?.key_retry_count ?? 3),
    max_concurrency: normalizeMaxConcurrency(next.settings?.max_concurrency),
    proxy_enabled: Boolean(next.settings?.proxy_enabled),
    proxy_url: typeof next.settings?.proxy_url === 'string' ? next.settings.proxy_url : '',
    price_per_request: numberSetting(next.settings, 'price_per_request', pricing.price_per_request),
    price_per_credit: numberSetting(next.settings, 'price_per_credit', pricing.price_per_credit),
    price_per_token: numberSetting(next.settings, 'price_per_token', pricing.price_per_token),
    default_billable_credits: numberSetting(next.settings, 'default_billable_credits', pricing.default_billable_credits)
  }
  providerForm.value = next
  creatingRow.value = false
  draftKey.value = ''
  draftExaServiceKey.value = ''
  activeTab.value = 'keys'
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
    await api.createKey({ provider_name: providerForm.value.name, alias: `${providerForm.value.name}-${Date.now()}`, key: draftKey.value.trim(), exa_service_key: draftExaServiceKey.value.trim(), weight: 1, rpm_limit: 0, daily_quota: 0, monthly_quota: 0 })
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
.provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}
.provider-card {
  cursor: pointer;
  transition: border-color .12s ease, box-shadow .12s ease, transform .12s ease;
}
.provider-card:hover {
  border-color: #d7dde5;
  box-shadow: 0 8px 24px rgba(16, 24, 40, 0.06);
  transform: translateY(-1px);
}
.provider-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.provider-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: #fff;
  font-weight: 800;
  background: linear-gradient(145deg, var(--primary), #14966a);
  flex: 0 0 auto;
}
.metric-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-top: 14px;
}
.metric-pill {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px 10px;
  background: #fafbfc;
  min-width: 0;
}
.metric-pill .label {
  color: var(--muted);
  font-size: 11px;
}
.metric-pill .value {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: -0.02em;
}

.dlg-ident {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
  padding-right: 28px;
}
.dlg-logo {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  color: #fff;
  font-weight: 800;
  background: linear-gradient(145deg, var(--primary), #14966a);
  flex: 0 0 auto;
}
.dlg-ident-main { min-width: 0; }
.dlg-title {
  font-size: 18px;
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.dlg-sub { font-size: 12px; margin-top: 2px; }
.dlg-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.dlg-badge {
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  background: #f2f4f7;
  color: #475467;
}
.dlg-badge.ok {
  background: #ecfdf3;
  color: #067647;
}
.dlg-badge.warn {
  background: #fffaeb;
  color: #b54708;
}
.dlg-badge.mute {
  background: #f2f4f7;
  color: #667085;
}

.channel-dialog-body {
  box-sizing: border-box;
  overflow: visible;
  padding: 0 2px 2px;
  min-height: 420px;
}
.provider-tabs :deep(.el-tabs__header) {
  margin: 0 0 14px;
}
.provider-tabs :deep(.el-tabs__item) {
  font-weight: 700;
}
.provider-tabs :deep(.el-tabs__item.is-active) {
  color: var(--primary);
}
.provider-tabs :deep(.el-tabs__active-bar) {
  background: var(--primary);
}
.provider-tabs :deep(.el-tabs__ink-bar) {
  background: var(--primary);
}
.provider-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--border);
}

.channel-section { margin-bottom: 18px; }
.channel-section.compact-bottom { margin-bottom: 4px; }
.section-line-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 800;
  color: var(--text);
}
.base-url-input :deep(.el-input__wrapper),
.key-input :deep(.el-input__wrapper) {
  height: 42px;
}
.api-key-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 280px;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  overscroll-behavior: contain;
}
.api-key-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) repeat(5, 28px);
  align-items: start;
  column-gap: 2px;
  width: 100%;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px 10px 10px 12px;
  background: #fff;
}
.api-key-row.is-new {
  grid-template-columns: minmax(0, 1fr) auto;
  border-style: dashed;
  background: #fcfcfd;
}
.new-key-fields {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.api-key-row > * { min-width: 0; }
.api-key-row .row-icon-button {
  justify-self: center;
  align-self: start;
  width: 28px;
  height: 42px;
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
  margin-top: 6px;
  padding-left: 2px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1;
  flex-wrap: wrap;
}
.key-meta span { white-space: nowrap; }
.quota-meta.is-success { color: var(--primary); }
.quota-meta.is-muted,
.quota-meta.is-danger { color: var(--danger); }
.new-key-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  align-self: center;
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
  border-radius: 12px;
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
  background: #fafbfc;
  border: 1px solid var(--border);
  border-radius: 12px;
}
.summary-label { color: var(--muted); font-size: 12px; }
.summary-card strong {
  color: var(--text);
  font-size: 18px;
  letter-spacing: -0.02em;
}
.sample-card {
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-height: 96px;
}
.sample-card small { font-size: 12px; }
.sample-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}
.price-card {
  border: 1px solid #d7ebe2;
  background: linear-gradient(180deg, #f5fbf8, #fff);
  border-radius: 14px;
  padding: 14px;
  margin-bottom: 14px;
}
.price-card-hd {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.price-card h3 {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 800;
}
.price-card p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}
.price-sample-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.advanced-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  border: 1px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  background: #fff;
}
.billing-form,
.runtime-form,
.advanced-only-form {
  margin-top: 0;
}
.advanced-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  min-height: 56px;
  padding: 10px 14px;
  border-right: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
.advanced-field:nth-child(2n) { border-right: 0; }
.advanced-field-wide {
  grid-column: 1 / -1;
  border-right: 0;
}
.advanced-field-label {
  color: var(--text);
  font-weight: 700;
  line-height: 1.4;
  white-space: nowrap;
}
.advanced-field :deep(.el-input-number) {
  width: 140px;
  flex-shrink: 0;
}
.advanced-field :deep(.el-select) {
  width: min(100%, 280px);
}
.advanced-field :deep(.el-input) {
  width: min(100%, 360px);
}
.dialog-action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}
.footer-hint { font-size: 12px; }
.footer-actions { display: flex; gap: 8px; }

@media (max-width: 720px) {
  .metric-row,
  .status-summary,
  .price-sample-grid,
  .advanced-form {
    grid-template-columns: 1fr;
  }
  .advanced-field,
  .advanced-field:nth-child(2n) {
    border-right: 0;
  }
}
</style>

