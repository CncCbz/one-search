<template>
  <div>
    <div class="page-actions">
      <el-button type="primary" @click="save">保存设置</el-button>
    </div>

    <div v-if="settings" class="settings-grid">
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
              <el-option value="exa" label="Exa" />
              <el-option value="you" label="You.com" />
              <el-option value="jina" label="Jina" />
            </el-select>
          </div>
          <div class="settings-item">
            <span>汇总返回结果数</span>
            <el-input-number v-model="settings.default_limit" :min="1" :max="50" controls-position="right" />
          </div>
          <div class="settings-item">
            <span>结果去重</span>
            <el-switch v-model="settings.default_dedupe" />
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
        </div>
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
import { ElMessage } from 'element-plus'
import { api, RuntimeSettings } from '../api/client'

const settings = ref<RuntimeSettings>()

async function load() {
  settings.value = await api.settings()
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
.compat-item { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 16px; border: 1px solid var(--border); border-radius: var(--el-border-radius-base); background: rgba(255, 255, 255, .58); }
.compat-item strong { display: block; color: var(--text); }
.compat-item span { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; }
@media (max-width: 1100px) {
  .settings-grid, .settings-items, .compat-grid { grid-template-columns: 1fr; }
  .settings-section-wide { grid-column: auto; }
}
</style>
