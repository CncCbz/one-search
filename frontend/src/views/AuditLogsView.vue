<template>
  <div class="audit-page">
    <div class="page-hd">
      <div>
        <h1>审计日志</h1>
        <p class="page-sub">安全时间线 · 活动流 · 高敏动作高亮</p>
      </div>
      <div class="page-actions">
        <el-button :icon="Refresh" circle title="刷新" :loading="loading" @click="load" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="table" :rows="8" class="audit-skeleton" />
    <template v-else>
      <section class="kpi-row">
        <div class="kpi-card">
          <span>近窗事件</span>
          <b>{{ logs.length }}</b>
        </div>
        <div class="kpi-card">
          <span>登录</span>
          <b>{{ loginCount }}</b>
        </div>
        <div class="kpi-card">
          <span>配置变更</span>
          <b>{{ changeCount }}</b>
        </div>
        <div class="kpi-card hot">
          <span>高敏动作</span>
          <b>{{ hotCount }}</b>
        </div>
      </section>

      <section class="filters">
        <el-input
          v-model="filterQ"
          clearable
          placeholder="搜索 actor / action / request_id / IP / 资源"
          :prefix-icon="Search"
        />
        <el-select v-model="filterRisk" clearable placeholder="全部风险" style="width: 120px">
          <el-option label="高敏" value="hot" />
          <el-option label="变更" value="warn" />
          <el-option label="常规" value="ok" />
        </el-select>
        <el-select v-model="filterResource" clearable placeholder="全部资源" style="width: 140px">
          <el-option v-for="item in resourceOptions" :key="item" :label="resourceLabel(item)" :value="item" />
        </el-select>
        <div class="chips">
          <button
            v-for="chip in actionChips"
            :key="chip.value"
            type="button"
            class="chip"
            :class="{ on: filterActionPrefix === chip.value }"
            @click="toggleActionChip(chip.value)"
          >{{ chip.label }}</button>
        </div>
      </section>

      <div v-loading="loading" class="stream">
        <article
          v-for="row in filteredLogs"
          :key="row.id"
          class="audit-card"
          :class="[riskClass(row), { active: selected?.id === row.id && drawerVisible }]"
          @click="openDetail(row)"
        >
          <div class="rail" :class="riskClass(row)" />
          <div class="body">
            <div class="title">{{ actionTitle(row.action) }}</div>
            <div class="sub">
              {{ row.actor || '—' }}
              · {{ resourceText(row) }}
              <template v-if="row.ip_address"> · {{ shortIP(row.ip_address) }}</template>
            </div>
            <div class="tags">
              <span class="tag" :class="riskClass(row)">{{ row.action }}</span>
              <span v-if="riskClass(row) === 'hot'" class="tag hot">高敏</span>
              <span v-else-if="riskClass(row) === 'warn'" class="tag warn">变更</span>
              <span v-if="row.request_id" class="tag mono">{{ shortRequestId(row.request_id) }}</span>
              <span v-for="chip in metadataChips(row).slice(0, 3)" :key="chip" class="tag">{{ chip }}</span>
            </div>
          </div>
          <div class="side">
            <b>{{ formatClock(row.created_at) }}</b>
            <span>{{ formatDay(row.created_at) }}</span>
          </div>
        </article>
        <div v-if="!filteredLogs.length" class="empty muted">暂无匹配审计事件</div>
      </div>
    </template>

    <Teleport to="body">
      <div v-if="drawerVisible" class="audit-mask" @click="drawerVisible = false" />
      <aside class="audit-drawer" :class="{ open: drawerVisible }" role="dialog" aria-modal="true">
        <template v-if="selected">
          <div class="dhd">
            <div class="dhd-main">
              <h2>{{ actionTitle(selected.action) }}</h2>
              <p>{{ selected.request_id || '—' }} · {{ formatTime(selected.created_at) }}</p>
            </div>
            <el-button circle :icon="Close" @click="drawerVisible = false" />
          </div>

          <div class="kv-grid">
            <div class="kv"><span>操作者</span><b>{{ selected.actor || '—' }}</b></div>
            <div class="kv"><span>动作</span><b>{{ selected.action || '—' }}</b></div>
            <div class="kv"><span>对象</span><b>{{ resourceText(selected) }}</b></div>
            <div class="kv"><span>IP</span><b>{{ selected.ip_address || '—' }}</b></div>
            <div class="kv"><span>时间</span><b>{{ formatTime(selected.created_at) }}</b></div>
            <div class="kv">
              <span>风险</span>
              <b :class="riskClass(selected)">{{ riskLabel(selected) }}</b>
            </div>
          </div>

          <div class="meta-label">metadata</div>
          <pre class="meta-pre">{{ prettyMetadata(selected.metadata) }}</pre>
        </template>
      </aside>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Close, Refresh, Search } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { api, AuditLog } from '../api/client'

const loading = ref(true)
const loaded = ref(false)
const logs = ref<AuditLog[]>([])
const filterQ = ref('')
const filterRisk = ref('')
const filterResource = ref('')
const filterActionPrefix = ref('')
const selected = ref<AuditLog | null>(null)
const drawerVisible = ref(false)

const actionChips = [
  { label: '全部', value: '' },
  { label: '登录', value: 'admin.login' },
  { label: '密钥', value: 'provider_key.' },
  { label: '令牌', value: 'api_token.' },
  { label: '设置', value: 'settings.' },
  { label: '渠道', value: 'provider.' },
]

const HOT_ACTIONS = new Set([
  'provider_key.reveal',
  'settings.admin_api_key.rotate',
])

const WARN_ACTIONS = new Set([
  'settings.update',
  'provider.update',
  'provider_key.create',
  'provider_key.update',
  'provider_key.delete',
  'api_token.create',
  'api_token.update',
  'api_token.status',
  'api_token.delete',
  'admin.login.failed',
])

const ACTION_TITLES: Record<string, string> = {
  'admin.login': '管理员登录',
  'admin.login.failed': '登录失败',
  'admin.logout': '管理员登出',
  'settings.update': '更新运行时设置',
  'settings.admin_api_key.rotate': '轮换管理 API Key',
  'provider.update': '更新渠道配置',
  'provider_key.create': '创建渠道密钥',
  'provider_key.update': '更新渠道密钥',
  'provider_key.delete': '删除渠道密钥',
  'provider_key.reveal': '揭示渠道密钥',
  'provider_key.test': '测试渠道密钥',
  'provider_key.quota': '查询密钥配额',
  'api_token.create': '创建 API 令牌',
  'api_token.update': '更新 API 令牌',
  'api_token.status': '变更令牌状态',
  'api_token.delete': '删除 API 令牌',
}

const resourceOptions = computed(() => {
  const set = new Set<string>()
  for (const row of logs.value) {
    if (row.resource_type) set.add(row.resource_type)
  }
  return [...set].sort()
})

const loginCount = computed(() => logs.value.filter((row) => row.action === 'admin.login' || row.action.startsWith('admin.login')).length)
const changeCount = computed(() => logs.value.filter((row) => riskClass(row) === 'warn').length)
const hotCount = computed(() => logs.value.filter((row) => riskClass(row) === 'hot').length)

const filteredLogs = computed(() => {
  const q = filterQ.value.trim().toLowerCase()
  return logs.value.filter((row) => {
    if (filterRisk.value && riskClass(row) !== filterRisk.value) return false
    if (filterResource.value && row.resource_type !== filterResource.value) return false
    if (filterActionPrefix.value) {
      const prefix = filterActionPrefix.value
      if (prefix.endsWith('.')) {
        if (!row.action.startsWith(prefix) && row.action !== prefix.slice(0, -1)) return false
      } else if (row.action !== prefix && !row.action.startsWith(prefix + '.')) {
        return false
      }
    }
    if (!q) return true
    const hay = [
      row.actor,
      row.action,
      row.request_id,
      row.ip_address,
      row.resource_type,
      row.resource_id,
      actionTitle(row.action),
      JSON.stringify(row.metadata || {}),
    ].join(' ').toLowerCase()
    return hay.includes(q)
  })
})

async function load() {
  loading.value = true
  try {
    logs.value = (await api.auditLogs()).logs
    loaded.value = true
  } finally {
    loading.value = false
  }
}

function openDetail(row: AuditLog) {
  selected.value = row
  drawerVisible.value = true
}

function toggleActionChip(value: string) {
  filterActionPrefix.value = filterActionPrefix.value === value ? '' : value
}

function riskClass(row: AuditLog): 'hot' | 'warn' | 'ok' {
  if (HOT_ACTIONS.has(row.action) || row.action.includes('reveal') || row.action.includes('rotate')) return 'hot'
  if (WARN_ACTIONS.has(row.action) || row.action.endsWith('.delete') || row.action.endsWith('.create') || row.action.endsWith('.update')) return 'warn'
  return 'ok'
}

function riskLabel(row: AuditLog) {
  const level = riskClass(row)
  if (level === 'hot') return '高敏'
  if (level === 'warn') return '变更'
  return '常规'
}

function actionTitle(action: string) {
  return ACTION_TITLES[action] || action || '未知动作'
}

function resourceLabel(type: string) {
  const map: Record<string, string> = {
    session: '会话',
    settings: '设置',
    provider: '渠道',
    provider_key: '渠道密钥',
    api_token: 'API 令牌',
  }
  return map[type] || type
}

function resourceText(row: AuditLog) {
  const type = row.resource_type ? resourceLabel(row.resource_type) : '—'
  if (row.resource_id) return `${type} #${row.resource_id}`
  return type
}

function shortRequestId(id: string) {
  if (!id) return '—'
  return id.length > 10 ? `${id.slice(0, 8)}…` : id
}

function shortIP(ip: string) {
  return ip.split(':')[0] || ip
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatClock(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatDay(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const today = new Date()
  if (date.toDateString() === today.toDateString()) return '今天'
  const y = new Date(today)
  y.setDate(today.getDate() - 1)
  if (date.toDateString() === y.toDateString()) return '昨天'
  return date.toLocaleDateString()
}

function metadataChips(row: AuditLog) {
  const meta = row.metadata || {}
  const chips: string[] = []
  for (const [key, val] of Object.entries(meta)) {
    if (val === null || val === undefined || val === '') continue
    if (typeof val === 'object') continue
    const text = `${key}=${String(val)}`
    chips.push(text.length > 28 ? `${text.slice(0, 25)}…` : text)
    if (chips.length >= 3) break
  }
  return chips
}

function prettyMetadata(meta: Record<string, unknown> | undefined) {
  try {
    return JSON.stringify(meta || {}, null, 2)
  } catch {
    return '{}'
  }
}

onMounted(load)
</script>

<style scoped>
.audit-page {
  height: calc(100dvh - 76px);
  max-height: calc(100dvh - 76px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}
.audit-page .page-hd {
  flex: 0 0 auto;
  margin-bottom: 12px;
}
.page-sub {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}
.audit-skeleton {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
.kpi-row {
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}
.kpi-card {
  border: 1px solid var(--border);
  background: var(--card);
  border-radius: 14px;
  padding: 12px 14px;
  box-shadow: var(--shadow);
}
.kpi-card span {
  display: block;
  color: var(--muted);
  font-size: 12px;
}
.kpi-card b {
  display: block;
  margin-top: 4px;
  font-size: 22px;
  letter-spacing: -0.03em;
}
.kpi-card.hot b { color: #b42318; }
.filters {
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--card);
}
.filters .el-input { flex: 1 1 220px; min-width: 200px; }
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.chip {
  border: 1px solid var(--border);
  background: #f6f7f9;
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 12px;
  color: #475467;
  cursor: pointer;
}
.chip.on {
  background: #e8f6f0;
  border-color: #b7e0cf;
  color: var(--primary);
  font-weight: 700;
}
.stream {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-right: 2px;
}
.audit-card {
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: 10px 1fr auto;
  gap: 12px;
  align-items: start;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--card);
  box-shadow: var(--shadow);
  cursor: pointer;
  transition: border-color 0.15s ease, transform 0.15s ease;
}
.audit-card:hover,
.audit-card.active {
  border-color: #b7e0cf;
  transform: translateY(-1px);
}
.audit-card.hot { background: linear-gradient(90deg, #fff8f7, #fff); }
.rail {
  width: 10px;
  min-height: 48px;
  height: 100%;
  border-radius: 999px;
  background: #d0d5dd;
}
.rail.ok { background: #12b76a; }
.rail.warn { background: #f79009; }
.rail.hot { background: #f04438; }
.title {
  font-weight: 800;
  font-size: 14px;
}
.sub {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.tag {
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 999px;
  background: #f2f4f7;
  color: #475467;
  border: 1px solid var(--border);
}
.tag.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
.tag.ok {
  background: #e8f6f0;
  color: var(--primary);
  border-color: #b7e0cf;
}
.tag.warn {
  background: #fffaeb;
  color: #b54708;
  border-color: #fedf89;
}
.tag.hot {
  background: #fef3f2;
  color: #b42318;
  border-color: #fecdca;
}
.side {
  text-align: right;
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}
.side b {
  display: block;
  color: var(--text);
  margin-bottom: 4px;
}
.empty {
  padding: 40px;
  text-align: center;
  background: var(--card);
  border: 1px dashed var(--border);
  border-radius: 16px;
}
.muted { color: var(--muted); }
@media (max-width: 900px) {
  .kpi-row { grid-template-columns: 1fr 1fr; }
  .audit-card { grid-template-columns: 8px 1fr; }
  .side { grid-column: 2; text-align: left; }
}
</style>

<style>
/* Teleport to body — unscoped */
.audit-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(15, 20, 25, 0.28);
  backdrop-filter: blur(2px);
}
.audit-drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2001;
  width: min(480px, 100vw);
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
  overflow: auto;
}
.audit-drawer.open {
  transform: none;
  pointer-events: auto;
}
.audit-drawer .dhd {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 0 0 12px;
  border-bottom: 1px solid var(--border, #e6e8ec);
  margin-bottom: 12px;
  flex: 0 0 auto;
}
.audit-drawer .dhd h2 {
  margin: 0;
  font-size: 16px;
  line-height: 1.35;
  word-break: break-word;
}
.audit-drawer .dhd p {
  margin: 4px 0 0;
  color: var(--muted, #667085);
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}
.audit-drawer .kv-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 12px;
}
.audit-drawer .kv {
  border: 1px solid var(--border, #e6e8ec);
  border-radius: 12px;
  padding: 10px;
  background: #fbfcfd;
}
.audit-drawer .kv span {
  display: block;
  color: var(--muted, #667085);
  font-size: 11px;
}
.audit-drawer .kv b {
  display: block;
  margin-top: 4px;
  font-size: 13px;
  word-break: break-all;
}
.audit-drawer .kv b.hot { color: #b42318; }
.audit-drawer .kv b.warn { color: #b54708; }
.audit-drawer .kv b.ok { color: #027a48; }
.audit-drawer .meta-label {
  color: var(--muted, #667085);
  font-size: 12px;
  margin-bottom: 6px;
}
.audit-drawer .meta-pre {
  margin: 0;
  padding: 12px;
  border-radius: 12px;
  background: #0f1419;
  color: #e8fff4;
  font: 12px/1.55 ui-monospace, Menlo, Consolas, monospace;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
@media (max-width: 720px) {
  .audit-drawer .kv-grid { grid-template-columns: 1fr; }
}
</style>
