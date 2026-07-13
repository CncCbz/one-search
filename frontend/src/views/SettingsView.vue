<template>
  <div>
    <div class="page-hd">
      <h1>系统设置</h1>
      <div class="page-actions">
        <el-button type="primary" :loading="loading" @click="save">保存</el-button>
      </div>
    </div>

    <PageSkeleton v-if="loading && !settings" type="form" />
    <div v-else-if="settings" class="settings-grid" v-loading="loading">
      <el-card class="soft-card settings-section" shadow="never">
        <template #header>搜索默认值</template>
        <div class="settings-items">
          <div class="settings-item">
            <span>默认模式</span>
            <el-select v-model="settings.default_mode">
              <el-option value="parallel" label="并发聚合" />
              <el-option value="fallback" label="失败转移" />
              <el-option value="single" label="单平台" />
            </el-select>
          </div>
          <div class="settings-item">
            <span>默认平台</span>
            <el-select v-model="settings.default_providers" multiple collapse-tags collapse-tags-tooltip>
              <el-option v-for="item in providerOptions" :key="item.value" :value="item.value" :label="item.label" />
            </el-select>
          </div>
          <div class="settings-item">
            <span>汇总返回结果数</span>
            <el-input-number v-model="settings.default_limit" :min="1" controls-position="right" />
          </div>
          <div class="settings-item">
            <span>结果去重</span>
            <el-switch v-model="settings.default_dedupe" />
          </div>
          <div class="settings-item">
            <span>平台路由策略</span>
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
      </el-card>

      <el-card class="soft-card settings-section" shadow="never">
        <template #header>接口与鉴权</template>
        <div class="settings-items">
          <div class="settings-item">
            <span>请求超时</span>
            <el-input-number v-model="settings.request_timeout_ms" :min="1000" controls-position="right" />
          </div>
          <div class="settings-item">
            <span>接口令牌鉴权</span>
            <el-switch v-model="settings.api_auth_required" />
          </div>
          <div class="settings-item">
            <span>健康统计窗口(分钟)</span>
            <el-input-number v-model="settings.provider_health_window_minutes" :min="1" :max="1440" controls-position="right" />
          </div>
          <div class="settings-item">
            <span>日志保留天数</span>
            <el-input-number v-model="settings.log_retention_days" :min="1" :max="365" controls-position="right" />
          </div>
        </div>
      </el-card>

      <el-card class="soft-card settings-section" shadow="never">
        <template #header>搜索缓存</template>
        <div class="settings-items">
          <div class="settings-item">
            <span>启用缓存</span>
            <el-switch v-model="settings.cache_enabled" />
          </div>
          <div class="settings-item">
            <span>缓存 TTL(秒)</span>
            <el-input-number v-model="settings.cache_ttl_seconds" :min="0" controls-position="right" :disabled="!settings.cache_enabled" />
          </div>
          <div class="settings-item">
            <span>最大缓存结果数</span>
            <el-input-number v-model="settings.cache_max_results" :min="0" controls-position="right" :disabled="!settings.cache_enabled" />
          </div>
        </div>
      </el-card>

      <el-card class="soft-card settings-section settings-section-wide" shadow="never">
        <template #header>管理员 API Key</template>
        <div class="admin-key-card">
          <div class="admin-key-info">
            <span>用于外部系统调用管理接口，拥有完整管理员权限。</span>
            <code v-if="adminAPIKey?.key_prefix">{{ adminAPIKey.key_prefix }}...</code>
            <el-tag v-else type="info">未生成</el-tag>
          </div>
          <el-button type="primary" @click="generateAdminAPIKey">{{ adminAPIKey?.key_prefix ? '重新随机生成' : '随机生成' }}</el-button>
        </div>
        <el-alert v-if="rawAdminAPIKey" type="success" show-icon :closable="false" class="admin-key-alert">
          <template #title>
            <span>新管理员 API Key 只显示一次：</span>
            <code>{{ rawAdminAPIKey }}</code>
            <el-button link type="primary" @click="copyText(rawAdminAPIKey)">复制</el-button>
          </template>
        </el-alert>
      </el-card>

      <el-card class="soft-card settings-section settings-section-wide" shadow="never">
        <template #header>兼容接口</template>
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
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { api, AdminAPIKey, RuntimeSettings } from '../api/client'
import { providerOptions } from '../utils/providers'

const loading = ref(true)
const settings = ref<RuntimeSettings>()
const adminAPIKey = ref<AdminAPIKey>()
const rawAdminAPIKey = ref('')

async function load() {
  loading.value = true
  try {
    const [runtimeSettings, currentAdminAPIKey] = await Promise.all([api.settings(), api.adminAPIKey()])
    settings.value = runtimeSettings
    adminAPIKey.value = currentAdminAPIKey
    if (!settings.value.provider_health_window_minutes) settings.value.provider_health_window_minutes = 15
    if (!settings.value.provider_routing_strategy) settings.value.provider_routing_strategy = 'fixed'
    if (!settings.value.log_retention_days) settings.value.log_retention_days = 3
  } finally {
    loading.value = false
  }
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
  if (!settings.value) return
  settings.value = await api.updateSettings(settings.value)
  ElMessage.success('设置已保存')
}

onMounted(load)
</script>

<style scoped>
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.settings-section-wide { grid-column: 1 / -1; }
.settings-items { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); border-top: 1px solid var(--border); border-left: 1px solid var(--border); }
.settings-item { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 60px; padding: 12px 16px; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.settings-item > span { flex-shrink: 0; color: var(--text); font-weight: 700; }
.settings-item :deep(.el-select), .settings-item :deep(.el-input-number) { width: 180px; }
.compat-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.admin-key-card { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: rgba(255, 255, 255, .58); }
.admin-key-info { display: flex; align-items: center; gap: 12px; min-width: 0; }
.admin-key-info span { color: var(--text); font-weight: 700; }
.admin-key-info code, .admin-key-alert code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.admin-key-alert { margin-top: 12px; }
.compat-item { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 16px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: rgba(255, 255, 255, .58); }
.compat-item strong { display: block; color: var(--text); }
.compat-item span { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; }
@media (max-width: 1100px) {
  .settings-grid, .settings-items, .compat-grid { grid-template-columns: 1fr; }
  .settings-section-wide { grid-column: auto; }
}
</style>
