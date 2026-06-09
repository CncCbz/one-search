<template>
  <div>
    <div class="page-actions"><el-button @click="load">刷新</el-button></div>
    <el-row :gutter="16">
      <el-col :span="8" v-for="item in metrics" :key="item.label"><el-card class="metric-card" shadow="never"><div class="muted">{{ item.label }}</div><h2>{{ item.value }}</h2></el-card></el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, UsageSummary } from '../api/client'
const usage = ref<UsageSummary>({ requests_total: 0, requests_success: 0, requests_failed: 0, cache_hits: 0, results_total: 0, average_latency_ms: 0 })
const metrics = computed(() => [
  { label: '总请求', value: usage.value.requests_total },
  { label: '失败请求', value: usage.value.requests_failed },
  { label: '结果总数', value: usage.value.results_total },
  { label: '缓存命中', value: usage.value.cache_hits },
  { label: '平均延迟(ms)', value: usage.value.average_latency_ms.toFixed(1) }
])
async function load() { usage.value = await api.usageSummary() }
onMounted(load)
</script>
