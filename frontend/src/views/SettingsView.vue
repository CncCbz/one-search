<template>
  <div class="settings-page">
    <div class="page-hd">
      <div>
        <h1>系统设置</h1>
        <p class="page-sub">全局运行配置 · 保存后对新请求立即生效</p>
      </div>
      <div class="page-actions">
        <el-button :loading="loading" :disabled="dirty" @click="load">刷新</el-button>
      </div>
    </div>

    <PageSkeleton v-if="loading && !settings" type="form" />
    <div v-else-if="settings" v-loading="loading" class="settings-stack">
      <section class="settings-card">
        <div class="sec-hd">
          <div>
            <h2>搜索默认值</h2>
            <p>影响 Playground 与未指定参数的 API 请求默认行为</p>
          </div>
        </div>
        <div class="field-grid">
          <div class="field">
            <label>默认模式</label>
            <el-select v-model="settings.default_mode">
              <el-option value="parallel" label="并发聚合" />
              <el-option value="fallback" label="失败转移" />
              <el-option value="single" label="单平台" />
            </el-select>
          </div>
          <div class="field">
            <label>汇总返回结果数</label>
            <el-input-number v-model="settings.default_limit" :min="1" controls-position="right" />
          </div>
          <div class="field field-full">
            <label>默认平台</label>
            <el-select v-model="settings.default_providers" multiple collapse-tags collapse-tags-tooltip>
              <el-option v-for="item in providerOptions" :key="item.value" :value="item.value" :label="item.label" />
            </el-select>
          </div>
          <div class="field field-switch">
            <div>
              <label>结果去重</label>
              <span class="hint">合并结果时按 URL / 标题去重</span>
            </div>
            <el-switch v-model="settings.default_dedupe" />
          </div>
          <div class="field">
            <label>平台路由策略</label>
            <el-select v-model="settings.provider_routing_strategy">
              <el-option value="fixed" label="固定顺序" />
              <el-option value="priority" label="优先级优先" />
              <el-option value="weighted" label="权重优先" />
              <el-option value="weighted_random" label="按权重随机" />
              <el-option value="available_keys" label="可用 Key 优先" />
              <el-option value="random" label="随机" />
            </el-select>
          </div>
        </div>
      </section>

      <section class="settings-card">
        <div class="sec-hd">
          <div>
            <h2>运行与接口</h2>
            <p>超时、鉴权、健康统计与日志生命周期</p>
          </div>
        </div>
        <div class="field-grid">
          <div class="field">
            <label>请求超时 (ms)</label>
            <el-input-number v-model="settings.request_timeout_ms" :min="1000" :step="1000" controls-position="right" />
          </div>
          <div class="field field-switch">
            <div>
              <label>接口令牌鉴权</label>
              <span class="hint">关闭后允许匿名调用搜索 API</span>
            </div>
            <el-switch v-model="settings.api_auth_required" />
          </div>
          <div class="field">
            <label>健康统计窗口 (分钟)</label>
            <el-input-number v-model="settings.provider_health_window_minutes" :min="1" :max="1440" controls-position="right" />
          </div>
          <div class="field">
            <label>日志保留天数</label>
            <el-input-number v-model="settings.log_retention_days" :min="1" :max="365" controls-position="right" />
          </div>
        </div>
        <div v-if="!settings.api_auth_required" class="warn-banner">
          接口鉴权已关闭，仅建议本地调试环境使用。
        </div>
      </section>

      <section class="settings-card">
        <div class="sec-hd">
          <div>
            <h2>搜索缓存（全局）</h2>
            <p>整次搜索结果缓存 · parallel 部分失败不写 · 相同请求 singleflight 合并</p>
          </div>
          <el-tag :type="settings.cache_enabled ? 'success' : 'info'" effect="plain" round>
            {{ settings.cache_enabled ? '已启用' : '已关闭' }}
          </el-tag>
        </div>
        <div class="field-grid">
          <div class="field field-switch">
            <div>
              <label>启用缓存</label>
              <span class="hint">请求级 cache 策略可覆盖</span>
            </div>
            <el-switch v-model="settings.cache_enabled" />
          </div>
          <div class="field">
            <label>缓存 TTL (秒)</label>
            <el-input-number
              v-model="settings.cache_ttl_seconds"
              :min="0"
              controls-position="right"
              :disabled="!settings.cache_enabled"
            />
          </div>
          <div class="field">
            <label>最大缓存结果数</label>
            <el-input-number
              v-model="settings.cache_max_results"
              :min="0"
              controls-position="right"
              :disabled="!settings.cache_enabled"
            />
            <span class="hint">单次响应超出则截断后写入；0 表示不截断</span>
          </div>
        </div>
      </section>

      <section class="settings-card">
        <div class="sec-hd">
          <div>
            <h2>安全</h2>
            <p>管理员 API Key 拥有完整管理权限，轮换后旧 Key 立即失效</p>
          </div>
        </div>
        <div class="admin-key-banner">
          <div class="admin-key-info">
            <strong>管理员 API Key</strong>
            <span>用于外部系统调用管理接口</span>
            <code v-if="adminAPIKey?.key_prefix">{{ adminAPIKey.key_prefix }}…</code>
            <el-tag v-else type="info" effect="plain">未生成</el-tag>
          </div>
          <el-button type="danger" plain @click="generateAdminAPIKey">
            {{ adminAPIKey?.key_prefix ? '重新随机生成' : '随机生成' }}
          </el-button>
        </div>
        <el-alert
          v-if="rawAdminAPIKey"
          type="success"
          show-icon
          :closable="false"
          class="admin-key-alert"
        >
          <template #title>
            <span>新管理员 API Key 只显示一次：</span>
            <code>{{ rawAdminAPIKey }}</code>
            <el-button link type="primary" @click="copyText(rawAdminAPIKey)">复制</el-button>
          </template>
        </el-alert>
        <div class="hint-banner">生成后明文只显示一次，请立即保存到密钥管理系统。操作会写入审计日志。</div>
      </section>

      <section class="settings-card">
        <div class="sec-hd">
          <div>
            <h2>兼容接口</h2>
            <p>对外暴露第三方形态的搜索入口，可按需开关</p>
          </div>
        </div>
        <div class="compat-grid">
          <div class="compat-item">
            <div>
              <strong>Tavily</strong>
              <span>/v1/compat/tavily/search</span>
            </div>
            <el-switch v-model="settings.compat_tavily_enabled" />
          </div>
          <div class="compat-item">
            <div>
              <strong>Serper</strong>
              <span>/v1/compat/serper/search</span>
            </div>
            <el-switch v-model="settings.compat_serper_enabled" />
          </div>
          <div class="compat-item">
            <div>
              <strong>OpenAI</strong>
              <span>/v1/compat/openai/responses-search</span>
            </div>
            <el-switch v-model="settings.compat_openai_enabled" />
          </div>
        </div>
      </section>

      <div class="settings-bottom-space" />
    </div>

    <div v-if="settings" class="savebar" :class="{ dirty }">
      <div class="savebar-inner">
        <span>{{ dirty ? '有未保存更改 · 保存后立即生效' : '已与服务器同步 · 修改后可保存' }}</span>
        <div class="savebar-actions">
          <el-button :disabled="!dirty" :loading="loading" @click="discard">放弃更改</el-button>
          <el-button type="primary" :disabled="!dirty" :loading="saving" @click="save">保存设置</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { api, AdminAPIKey, RuntimeSettings } from '../api/client'
import { providerOptions } from '../utils/providers'

const loading = ref(true)
const saving = ref(false)
const settings = ref<RuntimeSettings>()
const savedSnapshot = ref('')
const adminAPIKey = ref<AdminAPIKey>()
const rawAdminAPIKey = ref('')

const dirty = computed(() => {
  if (!settings.value || !savedSnapshot.value) return false
  return serialize(settings.value) !== savedSnapshot.value
})

function normalize(s: RuntimeSettings): RuntimeSettings {
  return {
    ...s,
    default_providers: [...(s.default_providers || [])].sort(),
    provider_health_window_minutes: s.provider_health_window_minutes || 15,
    provider_routing_strategy: s.provider_routing_strategy || 'fixed',
    log_retention_days: s.log_retention_days || 3,
  }
}

function serialize(s: RuntimeSettings) {
  const n = normalize(s)
  return JSON.stringify({
    ...n,
    default_providers: [...n.default_providers].sort(),
  })
}

function rememberSaved(s: RuntimeSettings) {
  const n = normalize(s)
  settings.value = n
  savedSnapshot.value = serialize(n)
}

async function load() {
  loading.value = true
  try {
    const [runtimeSettings, currentAdminAPIKey] = await Promise.all([api.settings(), api.adminAPIKey()])
    rememberSaved(runtimeSettings)
    adminAPIKey.value = currentAdminAPIKey
    rawAdminAPIKey.value = ''
  } finally {
    loading.value = false
  }
}

function discard() {
  if (!savedSnapshot.value) return
  settings.value = JSON.parse(savedSnapshot.value) as RuntimeSettings
}

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

async function generateAdminAPIKey() {
  try {
    await ElMessageBox.confirm('生成新的管理员 API Key 后，旧 Key 将立即失效。是否继续？', '确认生成管理员 API Key', { type: 'warning' })
  } catch {
    return
  }
  const result = await api.rotateAdminAPIKey()
  adminAPIKey.value = result
  rawAdminAPIKey.value = result.key || ''
  ElMessage.success('管理员 API Key 已生成')
}

async function save() {
  if (!settings.value || !dirty.value) return
  saving.value = true
  try {
    const next = await api.updateSettings(settings.value)
    rememberSaved(next)
    ElMessage.success('设置已保存')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.settings-page {
  max-width: 920px;
  margin: 0 auto;
  padding-bottom: 24px;
}
.page-sub {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
}
.settings-stack {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.settings-card {
  border: 1px solid var(--border);
  background: var(--card);
  border-radius: 18px;
  box-shadow: var(--shadow);
  padding: 16px 18px 18px;
}
.sec-hd {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.sec-hd h2 {
  margin: 0;
  font-size: 16px;
  letter-spacing: -0.02em;
}
.sec-hd p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: #fbfcfd;
  min-width: 0;
}
.field-full { grid-column: 1 / -1; }
.field label {
  font-size: 12px;
  font-weight: 700;
  color: var(--muted);
}
.field .hint {
  display: block;
  margin-top: 3px;
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
}
.field :deep(.el-select),
.field :deep(.el-input-number) {
  width: 100%;
}
.field-switch {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}
.warn-banner,
.hint-banner {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.5;
}
.warn-banner {
  background: #fffaeb;
  border: 1px solid #fedf89;
  color: #b54708;
}
.hint-banner {
  background: #f6f7f9;
  border: 1px solid var(--border);
  color: var(--muted);
}
.admin-key-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid #b7e0cf;
  background: linear-gradient(180deg, #f3fbf7, #fff);
}
.admin-key-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.admin-key-info strong { font-size: 14px; }
.admin-key-info span { color: var(--muted); font-size: 12px; }
.admin-key-info code,
.admin-key-alert code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
}
.admin-key-info code {
  display: inline-block;
  width: fit-content;
  margin-top: 4px;
  padding: 4px 8px;
  border-radius: 8px;
  background: #0f1419;
  color: #d9ffe9;
  font-size: 12px;
}
.admin-key-alert { margin-top: 12px; }
.compat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}
.compat-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: #fbfcfd;
}
.compat-item strong { display: block; font-size: 13px; }
.compat-item span {
  display: block;
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
  word-break: break-all;
}
.settings-bottom-space { height: 72px; }
.savebar {
  position: sticky;
  bottom: 8px;
  z-index: 5;
  margin-top: -56px;
  pointer-events: none;
}
.savebar.dirty .savebar-inner {
  border-color: #b7e0cf;
  box-shadow: 0 10px 30px rgba(11, 110, 79, 0.12);
}
.savebar-inner {
  pointer-events: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(8px);
  box-shadow: 0 10px 30px rgba(16, 24, 40, 0.1);
}
.savebar-inner > span {
  color: var(--muted);
  font-size: 13px;
}
.savebar-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
@media (max-width: 860px) {
  .field-grid,
  .compat-grid { grid-template-columns: 1fr; }
  .admin-key-banner,
  .savebar-inner {
    flex-direction: column;
    align-items: stretch;
  }
  .savebar-actions { width: 100%; }
  .savebar-actions :deep(.el-button) { flex: 1; }
}
</style>
