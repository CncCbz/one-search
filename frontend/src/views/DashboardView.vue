<template>
  <div class="dashboard-page">
    <div class="page-hd">
      <h1>仪表盘</h1>
      <div class="page-actions">
        <div class="range-seg" role="tablist" aria-label="时间范围">
          <button
            v-for="item in rangeOptions"
            :key="item.key"
            type="button"
            class="range-chip"
            :class="{ on: rangeKey === item.key }"
            @click="setRange(item.key)"
          >{{ item.label }}</button>
        </div>
        <el-button :icon="Refresh" circle title="刷新" :loading="loading" @click="load" />
      </div>
    </div>

    <PageSkeleton v-if="loading && !loaded" type="dashboard" />
    <template v-else>
      <section class="kpi-strip card">
        <div v-for="item in metrics" :key="item.label" class="kpi-cell">
          <div class="kpi-label">{{ item.label }}</div>
          <div class="kpi-value">{{ item.value }}</div>
          <div :ref="(el) => setSparkRef(item.key, el)" class="kpi-spark" />
        </div>
      </section>

      <section class="chart-grid">
        <div class="card chart-card">
          <div class="sec-hd">
            <h3>请求与延迟</h3>
            <span class="muted">{{ rangeMeta.label }}</span>
          </div>
          <div ref="dualRef" class="chart dual" />
        </div>
        <div class="card chart-card">
          <div class="sec-hd">
            <h3>平台贡献</h3>
            <span class="muted">请求占比</span>
          </div>
          <div ref="donutRef" class="chart donut" />
        </div>
      </section>

      <section class="card health-card" v-loading="loading">
        <div class="sec-hd health-hd">
          <div>
            <h3>渠道健康</h3>
            <p class="health-sub">{{ healthWindowLabel }}</p>
          </div>
          <div class="hlegend">
            <span><i class="ok" />正常</span>
            <span><i class="degraded" />降级</span>
            <span><i class="down" />故障</span>
            <span><i class="off" />无请求</span>
          </div>
        </div>
        <div v-if="healthRows.length" class="health-grid">
          <article v-for="row in healthRows" :key="row.provider_name" class="hcard">
            <div class="hcard-top">
              <div class="hleft">
                <span class="hcheck" :class="statusClass(row.status)" aria-hidden="true">
                  <svg v-if="row.status === 'healthy'" viewBox="0 0 16 16" width="11" height="11"><path fill="currentColor" d="M6.5 11.2 3.3 8l1.1-1.1 2.1 2.1 5-5L12.6 5.1z"/></svg>
                  <svg v-else-if="row.status === 'degraded'" viewBox="0 0 16 16" width="11" height="11"><path fill="currentColor" d="M7.1 3h1.8l.2 7H6.9l.2-7zm.9 10.2a1.1 1.1 0 1 1 0-2.2 1.1 1.1 0 0 1 0 2.2z"/></svg>
                  <svg v-else-if="row.status === 'down' || row.status === 'no_keys'" viewBox="0 0 16 16" width="11" height="11"><path fill="currentColor" d="m8 9.1 2.8 2.8 1.1-1.1L9.1 8l2.8-2.8-1.1-1.1L8 6.9 5.2 4.1 4.1 5.2 6.9 8l-2.8 2.8 1.1 1.1z"/></svg>
                  <svg v-else viewBox="0 0 16 16" width="11" height="11"><path fill="currentColor" d="M3.5 7.25h9v1.5h-9z"/></svg>
                </span>
                <div class="htitle">
                  <strong>{{ row.display_name || providerLabel(row.provider_name) }}</strong>
                  <span class="meta">{{ row.available_keys }}/{{ row.total_keys }} 密钥 · {{ healthLabel(row.status) }}</span>
                </div>
              </div>
              <div class="hright">
                <b>{{ formatUptime(row.uptime_percent, row.status) }}</b>
              </div>
            </div>
            <div class="hbar" :style="{ gridTemplateColumns: `repeat(${Math.max(row.segments.length, 1)}, minmax(0, 1fr))` }">
              <el-tooltip
                v-for="(seg, idx) in (row.segments.length ? row.segments : emptySegments())"
                :key="idx"
                effect="dark"
                placement="top"
                :show-after="80"
                :content="segmentTitle(seg, idx, row)"
              >
                <i :class="seg.status || 'off'" />
              </el-tooltip>
            </div>
          </article>
        </div>
        <div v-else class="empty-health muted">暂无平台，请先在平台管理中配置</div>
      </section>

      <section class="bottom-grid">
        <div class="card chart-card">
          <div class="sec-hd">
            <h3>用量成本</h3>
            <span class="muted">{{ rangeMeta.label }}</span>
          </div>
          <div ref="costRef" class="chart cost" />
        </div>
        <div class="card bill-card">
          <div class="sec-hd">
            <h3>账单明细</h3>
            <span class="muted">USD</span>
          </div>
          <div class="bill-list">
            <div v-for="item in billing.units" :key="`${item.provider_name}-${item.unit}`" class="bill-row">
              <div>
                <strong>{{ providerLabel(item.provider_name) }}</strong>
                <small>{{ usageUnitLabel(item.unit) }} · {{ formatNumber(item.quantity_total) }}</small>
              </div>
              <b>{{ formatCurrency(item.cost_usd_total) }}</b>
            </div>
            <div v-if="!billing.units.length" class="empty-health muted">暂无账单数据</div>
            <div v-else class="bill-row total">
              <div>
                <strong>合计</strong>
                <small>{{ rangeMeta.label }}</small>
              </div>
              <b>{{ formatCurrency(billingTotal) }}</b>
            </div>
          </div>
        </div>
        <div class="alert-card" :class="{ quiet: !attentionText }">
          <h4>{{ attentionText ? '需要关注' : '运行平稳' }}</h4>
          <p>{{ attentionText || '近窗平台状态正常，暂无需要处理的告警。' }}</p>
          <div ref="failRef" class="chart fail" />
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { Refresh } from '@element-plus/icons-vue'
import PageSkeleton from '../components/PageSkeleton.vue'
import {
  api,
  BillingSummary,
  DashboardRangeKey,
  DashboardRangeMeta,
  HealthSegmentPoint,
  HealthSegmentSeries,
  ProviderConfig,
  ProviderHealth,
  ProviderUsagePoint,
  SearchLog,
  UsageSeries,
  UsageSummary
} from '../api/client'
import { providerLabel } from '../utils/providers'

const RANGE_OPTIONS: { key: DashboardRangeKey; label: string }[] = [
  { key: '24h', label: '24h' },
  { key: 'today', label: '今日' },
  { key: '7d', label: '7d' },
  { key: '14d', label: '14d' },
  { key: '30d', label: '30d' }
]
const rangeOptions = RANGE_OPTIONS

function defaultRangeMeta(key: DashboardRangeKey): DashboardRangeMeta {
  const now = new Date()
  if (key === 'today') {
    const start = new Date(now)
    start.setHours(0, 0, 0, 0)
    const hours = Math.max(1, Math.min(24, Math.floor((now.getTime() - start.getTime()) / 3600000) + 1))
    return { range: key, label: '今日', granularity: 'hour', segment_minutes: 60, segments: hours, billing_days: 1 }
  }
  const table: Record<Exclude<DashboardRangeKey, 'today'>, DashboardRangeMeta> = {
    '24h': { range: '24h', label: '近 24 小时', granularity: 'hour', segment_minutes: 60, segments: 24, billing_days: 1 },
    '7d': { range: '7d', label: '近 7 天', granularity: 'day', segment_minutes: 1440, segments: 7, billing_days: 7 },
    '14d': { range: '14d', label: '近 14 天', granularity: 'day', segment_minutes: 1440, segments: 14, billing_days: 14 },
    '30d': { range: '30d', label: '近 30 天', granularity: 'day', segment_minutes: 1440, segments: 30, billing_days: 30 }
  }
  return table[key] || table['14d']
}

const savedRange = (localStorage.getItem('osr.dashboard.range') as DashboardRangeKey) || '14d'
const rangeKey = ref<DashboardRangeKey>(RANGE_OPTIONS.some((item) => item.key === savedRange) ? savedRange : '14d')
const rangeMeta = ref<DashboardRangeMeta>(defaultRangeMeta(rangeKey.value))
const healthFromLogs = ref(false)

echarts.use([BarChart, LineChart, PieChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

type EChartsType = echarts.ECharts

const brand = '#0b6e4f'
const brand2 = '#32b27f'
const warn = '#f79009'
const danger = '#f04438'
const mute = '#d0d5dd'

const loading = ref(true)
const loaded = ref(false)
const usage = ref<UsageSummary>({
  requests_total: 0,
  requests_success: 0,
  requests_failed: 0,
  cache_hits: 0,
  results_total: 0,
  average_latency_ms: 0
})
const providers = ref<ProviderConfig[]>([])
const providerHealth = ref<ProviderHealth[]>([])
const billing = ref<BillingSummary>({ days: 30, units: [] })
const usageSeries = ref<UsageSeries>({ days: 14, points: [] })
const providerSeries = ref<ProviderUsagePoint[]>([])
const healthSeries = ref<HealthSegmentSeries[]>([])

const dualRef = ref<HTMLElement | null>(null)
const donutRef = ref<HTMLElement | null>(null)
const costRef = ref<HTMLElement | null>(null)
const failRef = ref<HTMLElement | null>(null)
const sparkRefs = ref<Record<string, HTMLElement | null>>({})

let dualChart: EChartsType | null = null
let donutChart: EChartsType | null = null
let costChart: EChartsType | null = null
let failChart: EChartsType | null = null
const sparkCharts: Record<string, EChartsType | null> = {}

const metrics = computed(() => [
  { key: 'total', label: '总请求', value: formatInt(usage.value.requests_total) },
  { key: 'success', label: '成功', value: formatInt(usage.value.requests_success) },
  { key: 'failed', label: '失败', value: formatInt(usage.value.requests_failed) },
  { key: 'results', label: '结果', value: formatInt(usage.value.results_total) },
  { key: 'cache', label: '缓存命中', value: formatInt(usage.value.cache_hits) },
  { key: 'latency', label: '平均延迟', value: `${(usage.value.average_latency_ms || 0).toFixed(1)}ms` }
])

const billingTotal = computed(() => billing.value.units.reduce((sum, item) => sum + (item.cost_usd_total || 0), 0))

const HEALTH_SEGMENTS = 30
const HEALTH_SEGMENT_MINUTES = 1440

function emptySegments(count = rangeMeta.value.segments || HEALTH_SEGMENTS) {
  const n = Math.max(1, count || HEALTH_SEGMENTS)
  return Array.from({ length: n }, () => ({ status: 'off', success: 0, failed: 0, total: 0 }))
}

function normalizeUsageSeries(series?: UsageSeries | null): UsageSeries {
  return {
    range: series?.range || rangeKey.value,
    granularity: series?.granularity || rangeMeta.value.granularity,
    days: series?.days || 0,
    points: series?.points || []
  }
}

function setRange(key: DashboardRangeKey) {
  if (rangeKey.value === key && loaded.value) {
    void load()
    return
  }
  rangeKey.value = key
  localStorage.setItem('osr.dashboard.range', key)
  void load()
}

function formatAxisLabel(value: string, granularity?: string) {
  if (!value) return ''
  if ((granularity || usageSeries.value.granularity) === 'hour') {
    // 2006-01-02 15:00 -> 15:00 or 07-13 15:00
    const parts = value.split(' ')
    if (parts.length === 2) {
      const [date, hour] = parts
      return `${date.slice(5)} ${hour.slice(0, 5)}`
    }
  }
  return value.length >= 10 ? value.slice(5) : value
}

// 始终按平台列表渲染健康条；无真实请求时段=灰，绝不画绿兜底。
const healthRows = computed<HealthSegmentSeries[]>(() => {
  const byName = new Map(healthSeries.value.map((item) => [item.provider_name, item]))
  if (providers.value.length) {
    return providers.value.map((provider) => {
      const series = byName.get(provider.name)
      if (series?.segments?.length) {
        return {
          ...series,
          display_name: series.display_name || provider.display_name || provider.name,
          available_keys: series.available_keys ?? provider.available_keys ?? 0,
          total_keys: series.total_keys || provider.available_keys || 0
        }
      }
      const health = providerHealth.value.find((item) => item.provider_name === provider.name)
      const disabled = health?.status === 'disabled' || !provider.enabled
      const noKeys = health?.status === 'no_keys' || ((health?.total_keys ?? provider.available_keys ?? 0) === 0)
      return {
        provider_name: provider.name,
        display_name: provider.display_name || provider.name,
        status: disabled ? 'disabled' : noKeys ? 'no_keys' : 'idle',
        available_keys: health?.available_keys ?? provider.available_keys ?? 0,
        total_keys: health?.total_keys || provider.available_keys || 0,
        success_rate: 0,
        uptime_percent: 0,
        segments: emptySegments(rangeMeta.value.segments),
        segment_minutes: rangeMeta.value.segment_minutes || HEALTH_SEGMENT_MINUTES
      }
    })
  }
  // 没有 providers 时仍展示后端返回的序列
  return healthSeries.value.map((item) => ({
    ...item,
    segments: item.segments?.length ? item.segments : emptySegments()
  }))
})

const healthWindowLabel = computed(() => {
  const minutes = healthRows.value[0]?.segment_minutes || rangeMeta.value.segment_minutes || HEALTH_SEGMENT_MINUTES
  const segs = healthRows.value[0]?.segments?.length || rangeMeta.value.segments || HEALTH_SEGMENTS
  const unit = minutes >= 1440 ? `${Math.round(minutes / 1440)} 天/段` : minutes >= 60 ? `${Math.round(minutes / 60)} 小时/段` : `${minutes} 分钟/段`
  const source = healthFromLogs.value ? ' · 日志回填' : ''
  return `${rangeMeta.value.label} · ${unit} · ${segs} 段${source}`
})

const attentionText = computed(() => {
  const bad = healthRows.value.filter((item) => ['degraded', 'down', 'no_keys'].includes(item.status))
  if (!bad.length) return ''
  return bad
    .map((item) => `${item.display_name || providerLabel(item.provider_name)}（${healthLabel(item.status)}）`)
    .join('、') + ' 需要关注。'
})

function setSparkRef(key: string, el: unknown) {
  sparkRefs.value[key] = (el as HTMLElement | null) || null
}

function formatInt(value: number) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value || 0)
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value || 0)
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value || 0)
}

function formatUptime(value: number, status?: string) {
  if (status === 'disabled') return '已停用'
  if (status === 'no_keys') return '无密钥'
  if (status === 'idle' || !Number.isFinite(value) || value <= 0) return '无请求样本'
  return `${value.toFixed(2)}% uptime`
}

function usageUnitLabel(unit: string) {
  return ({ requests: '请求', credits: 'Credits', tokens: 'Tokens', usd: 'USD' } as Record<string, string>)[unit] || unit
}

function healthLabel(status: string) {
  const labels: Record<string, string> = {
    healthy: '健康',
    degraded: '降级',
    down: '不可用',
    disabled: '停用',
    no_keys: '无密钥',
    idle: '无流量',
    unknown: '未知'
  }
  return labels[status] || status
}

function statusClass(status: string) {
  if (status === 'healthy') return 'ok'
  if (status === 'degraded') return 'warn'
  if (status === 'down' || status === 'no_keys') return 'bad'
  return 'mute' // idle / disabled / unknown
}

function statusIcon(status: string) {
  if (status === 'healthy') return '✓'
  if (status === 'degraded') return '!'
  if (status === 'down' || status === 'no_keys') return '×'
  return '–'
}

function segmentTitle(seg: { status?: string; success?: number; failed?: number; total?: number } | string, idx: number, row: HealthSegmentSeries) {
  const point = typeof seg === 'string'
    ? { status: seg, success: 0, failed: 0, total: 0 }
    : seg
  const status = point.status || 'off'
  const label = ({ ok: '正常', degraded: '降级', down: '故障', off: '无请求' } as Record<string, string>)[status] || status
  const success = Number(point.success || 0)
  const failed = Number(point.failed || 0)
  const total = Number(point.total || success + failed)
  const minutes = row.segment_minutes || rangeMeta.value.segment_minutes || 60
  const bucketLabel = minutes >= 1440 ? `${Math.round(minutes / 1440)}天` : minutes >= 60 ? `${Math.round(minutes / 60)}小时` : `${minutes}分钟`
  if (status === 'off' || total <= 0) {
    return `${label} · ${bucketLabel} · 成功 0 · 失败 0`
  }
  return `${label} · ${bucketLabel} · 成功 ${success} · 失败 ${failed} · 共 ${total}`
}

function ensureChart(el: HTMLElement | null, current: EChartsType | null) {
  if (!el) return null
  if (current && !current.isDisposed()) return current
  return echarts.init(el)
}

function sparkOption(data: number[], color: string) {
  return {
    animation: false,
    grid: { left: 0, right: 0, top: 4, bottom: 0 },
    xAxis: { type: 'category', show: false, data: data.map((_, i) => i) },
    yAxis: { type: 'value', show: false },
    series: [{
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2, color },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: color + '55' },
          { offset: 1, color: color + '00' }
        ])
      },
      data
    }]
  }
}

function renderCharts() {
  const points = usageSeries.value.points || []
  const dates = points.map((p) => formatAxisLabel(p.date, usageSeries.value.granularity))
  const totals = points.map((p) => p.requests_total || 0)
  const fails = points.map((p) => p.requests_failed || 0)
  const successes = points.map((p) => p.requests_success || 0)
  const latencies = points.map((p) => Number((p.average_latency_ms || 0).toFixed(1)))
  const caches = points.map((p) => p.cache_hits || 0)
  const results = points.map((p) => p.results_total || 0)

  const sparkMap: Record<string, { data: number[]; color: string }> = {
    total: { data: totals, color: brand },
    success: { data: successes, color: brand2 },
    failed: { data: fails, color: danger },
    results: { data: results, color: brand },
    cache: { data: caches, color: brand2 },
    latency: { data: latencies, color: brand }
  }
  for (const key of Object.keys(sparkMap)) {
    const el = sparkRefs.value[key]
    if (!el) continue
    sparkCharts[key] = ensureChart(el, sparkCharts[key] || null)
    sparkCharts[key]?.setOption(sparkOption(sparkMap[key].data, sparkMap[key].color), true)
  }

  dualChart = ensureChart(dualRef.value, dualChart)
  dualChart?.setOption({
    color: [brand, brand2],
    grid: { left: 42, right: 42, top: 36, bottom: 28 },
    tooltip: { trigger: 'axis' },
    legend: { top: 0, right: 0, textStyle: { color: '#667085' } },
    xAxis: {
      type: 'category',
      data: dates,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#e6e8ec' } },
      axisLabel: { color: '#98a2b3' }
    },
    yAxis: [
      { type: 'value', name: '请求', splitLine: { lineStyle: { color: '#eef1f4' } }, axisLabel: { color: '#98a2b3' } },
      { type: 'value', name: 'ms', splitLine: { show: false }, axisLabel: { color: '#98a2b3' } }
    ],
    series: [
      {
        name: '请求',
        type: 'bar',
        barWidth: 16,
        itemStyle: {
          borderRadius: [6, 6, 0, 0],
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: brand2 },
            { offset: 1, color: brand }
          ])
        },
        data: totals
      },
      {
        name: '延迟',
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { width: 2 },
        data: latencies
      }
    ]
  }, true)

  const pieData = (providerSeries.value.length
    ? providerSeries.value
    : providers.value.map((p) => ({
      provider_name: p.name,
      display_name: p.display_name,
      requests_total: providerHealth.value.find((h) => h.provider_name === p.name)?.requests_total || 0
    }))
  ).map((item) => ({
    name: item.display_name || providerLabel(item.provider_name),
    value: item.requests_total || 0
  })).filter((item) => item.value > 0)

  donutChart = ensureChart(donutRef.value, donutChart)
  donutChart?.setOption({
    color: [brand, brand2, warn, mute, '#86d4b2'],
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: '#667085' } },
    series: [{
      type: 'pie',
      radius: ['52%', '74%'],
      center: ['50%', '45%'],
      itemStyle: { borderRadius: 8, borderColor: '#fff', borderWidth: 3 },
      label: { show: false },
      data: pieData.length ? pieData : [{ name: '暂无数据', value: 1, itemStyle: { color: mute } }]
    }]
  }, true)

  const costLabels = billing.value.units.map((item) => providerLabel(item.provider_name))
  const costValues = billing.value.units.map((item) => Number((item.cost_usd_total || 0).toFixed(4)))
  costChart = ensureChart(costRef.value, costChart)
  costChart?.setOption({
    grid: { left: 48, right: 12, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis', valueFormatter: (v: number) => `$${v}` },
    xAxis: {
      type: 'category',
      data: costLabels.length ? costLabels : ['—'],
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#e6e8ec' } },
      axisLabel: { color: '#667085' }
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: '#eef1f4' } },
      axisLabel: { color: '#98a2b3', formatter: '${value}' }
    },
    series: [{
      type: 'bar',
      barWidth: 26,
      itemStyle: { borderRadius: [8, 8, 0, 0], color: brand },
      data: costValues.length ? costValues : [0]
    }]
  }, true)

  failChart = ensureChart(failRef.value, failChart)
  failChart?.setOption({
    grid: { left: 28, right: 8, top: 10, bottom: 20 },
    xAxis: {
      type: 'category',
      data: dates,
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: { color: '#98a2b3', fontSize: 10 }
    },
    yAxis: { show: false },
    series: [{
      type: 'bar',
      barWidth: 8,
      itemStyle: { borderRadius: [4, 4, 0, 0], color: danger },
      data: fails
    }]
  }, true)
}

function resizeCharts() {
  dualChart?.resize()
  donutChart?.resize()
  costChart?.resize()
  failChart?.resize()
  Object.values(sparkCharts).forEach((chart) => chart?.resize())
}

function disposeCharts() {
  dualChart?.dispose()
  donutChart?.dispose()
  costChart?.dispose()
  failChart?.dispose()
  Object.keys(sparkCharts).forEach((key) => {
    sparkCharts[key]?.dispose()
    sparkCharts[key] = null
  })
  dualChart = null
  donutChart = null
  costChart = null
  failChart = null
}


function parseAPITime(value?: string | null) {
  if (!value) return null
  const normalized = value.replace(/(\.\d{1,6})\d*(Z|[+-]\d{2}:?\d{2})?$/, (_, frac, tz) => {
    const digits = String(frac || '.').slice(1).padEnd(6, '0').slice(0, 6)
    return `.${digits}${tz || ''}`
  })
  const date = new Date(normalized)
  return Number.isNaN(date.getTime()) ? null : date
}

function rangeStart(meta: DashboardRangeMeta, now = new Date()) {
  if (meta.range === 'today') {
    const start = new Date(now)
    start.setHours(0, 0, 0, 0)
    return start
  }
  if (meta.range === '24h') return new Date(now.getTime() - 24 * 3600 * 1000)
  const days = meta.billing_days || meta.segments || 14
  return new Date(now.getTime() - days * 24 * 3600 * 1000)
}

function segmentStatus(success: number, failed: number) {
  const total = success + failed
  if (total <= 0) return 'off'
  const rate = success / total
  if (rate < 0.5) return 'down'
  if (rate < 0.9) return 'degraded'
  return 'ok'
}

function buildEmptyHealthSeries(): HealthSegmentSeries[] {
  const segs = rangeMeta.value.segments || HEALTH_SEGMENTS
  const minutes = rangeMeta.value.segment_minutes || HEALTH_SEGMENT_MINUTES
  return providers.value.map((provider) => {
    const health = providerHealth.value.find((item) => item.provider_name === provider.name)
    const disabled = health?.status === 'disabled' || !provider.enabled
    const noKeys = health?.status === 'no_keys' || ((health?.total_keys ?? provider.available_keys ?? 0) === 0)
    return {
      provider_name: provider.name,
      display_name: provider.display_name || provider.name,
      status: disabled ? 'disabled' : noKeys ? 'no_keys' : 'idle',
      available_keys: health?.available_keys ?? provider.available_keys ?? 0,
      total_keys: health?.total_keys || provider.available_keys || 0,
      success_rate: 0,
      uptime_percent: 0,
      segments: emptySegments(segs),
      segment_minutes: minutes
    }
  })
}

async function mapPool<T, R>(items: T[], concurrency: number, worker: (item: T, index: number) => Promise<R>) {
  const results = new Array<R>(items.length)
  let cursor = 0
  async function run() {
    while (cursor < items.length) {
      const index = cursor++
      results[index] = await worker(items[index], index)
    }
  }
  const runners = Array.from({ length: Math.min(concurrency, Math.max(items.length, 1)) }, () => run())
  await Promise.all(runners)
  return results
}

/** 旧版 dashboard 无 health_series 时，用请求日志 + provider_calls 回填健康条。 */
async function buildHealthSeriesFromLogs(meta: DashboardRangeMeta): Promise<HealthSegmentSeries[]> {
  const minutes = Math.max(1, meta.segment_minutes || 60)
  const segments = Math.max(1, meta.segments || 24)
  const now = Date.now()
  const start = rangeStart(meta, new Date(now)).getTime()
  const base = buildEmptyHealthSeries()
  if (!base.length) return []

  let logs: SearchLog[] = []
  try {
    logs = (await api.logs(200)).logs || []
  } catch {
    return base
  }
  const inRange = logs.filter((log) => {
    const ts = parseAPITime(log.created_at)?.getTime()
    return typeof ts === 'number' && ts >= start && ts <= now
  })
  if (!inRange.length) return base

  // 旧接口日志详情不含 call.created_at，统一用 search log 时间分桶。
  const details = await mapPool(inRange.slice(0, 80), 4, async (log) => {
    try {
      return await api.logDetail(log.id)
    } catch {
      return null
    }
  })

  type Acc = { success: number; failed: number }
  const buckets = new Map<string, Acc[]>()
  for (const row of base) {
    buckets.set(row.provider_name, Array.from({ length: segments }, () => ({ success: 0, failed: 0 })))
  }

  details.forEach((detail, index) => {
    if (!detail) return
    const log = inRange[index]
    const ts = parseAPITime(log.created_at)?.getTime()
    if (typeof ts !== 'number') return
    const ageMin = Math.max(0, (now - ts) / 60000)
    let bucket = Math.floor(ageMin / minutes)
    if (bucket < 0) bucket = 0
    if (bucket >= segments) return
    const idx = segments - 1 - bucket
    const calls = detail.calls || []
    if (!calls.length) {
      // 没有 call 明细时，退化为按 log.providers + log.status 记账
      const ok = log.status !== 'error'
      for (const name of log.providers || []) {
        const arr = buckets.get(name)
        if (!arr) continue
        if (ok) arr[idx].success += 1
        else arr[idx].failed += 1
      }
      return
    }
    for (const call of calls) {
      const arr = buckets.get(call.provider_name)
      if (!arr) continue
      if (call.status === 'error') arr[idx].failed += 1
      else if (call.status === 'success') arr[idx].success += 1
      // skipped 不计入
    }
  })

  return base.map((row) => {
    const arr = buckets.get(row.provider_name) || []
    let score = 0
    let scored = 0
    let okBuckets = 0
    let degradedBuckets = 0
    let downBuckets = 0
    let reqSuccess = 0
    let reqFailed = 0
    const points: HealthSegmentPoint[] = arr.map((cell) => {
      const total = cell.success + cell.failed
      const status = segmentStatus(cell.success, cell.failed)
      if (total > 0) {
        reqSuccess += cell.success
        reqFailed += cell.failed
        scored += 1
        if (status === 'ok') { okBuckets += 1; score += 1 }
        else if (status === 'degraded') { degradedBuckets += 1; score += 0.5 }
        else if (status === 'down') { downBuckets += 1 }
      }
      return { status, success: cell.success, failed: cell.failed, total }
    })
    let status = row.status
    if (status !== 'disabled' && status !== 'no_keys') {
      if (scored === 0) status = 'idle'
      else if (downBuckets > 0 && downBuckets >= degradedBuckets && downBuckets >= okBuckets) status = 'down'
      else if (downBuckets > 0 || degradedBuckets > 0) status = 'degraded'
      else status = 'healthy'
    }
    const total = reqSuccess + reqFailed
    return {
      ...row,
      status,
      success_rate: total > 0 ? reqSuccess / total : 0,
      uptime_percent: scored > 0 ? (score / scored) * 100 : 0,
      segments: points,
      segment_minutes: minutes
    }
  })
}

async function load() {
  loading.value = true
  try {
    // 先用本地口径，避免旧后端没返回 range 时仍显示 15m/90 段。
    rangeMeta.value = defaultRangeMeta(rangeKey.value)
    healthFromLogs.value = false
    const result = await api.dashboard(rangeKey.value)
    if (result.range) {
      rangeMeta.value = {
        range: (result.range.range as DashboardRangeKey) || rangeKey.value,
        label: result.range.label || rangeMeta.value.label,
        granularity: result.range.granularity || rangeMeta.value.granularity,
        segment_minutes: result.range.segment_minutes || rangeMeta.value.segment_minutes,
        segments: result.range.segments || rangeMeta.value.segments,
        billing_days: result.range.billing_days || rangeMeta.value.billing_days
      }
    }
    usage.value = result.usage
    providers.value = result.providers
    providerHealth.value = result.provider_health || []
    billing.value = result.billing || { days: rangeMeta.value.billing_days || 14, units: [] }
    usageSeries.value = normalizeUsageSeries(result.usage_series)
    providerSeries.value = result.provider_series || []

    const remoteHealth = (result.health_series || []).map((item) => ({
      ...item,
      segments: item.segments?.length ? item.segments : emptySegments(rangeMeta.value.segments)
    }))
    // 远程旧接口没有 health_series / 全灰时，回退到请求日志真实 provider_calls。
    const hasTraffic = remoteHealth.some((row) => (row.segments || []).some((seg) => (seg?.total || 0) > 0 || (seg?.status && seg.status !== 'off')))
    if (remoteHealth.length && hasTraffic) {
      healthSeries.value = remoteHealth
    } else {
      healthSeries.value = await buildHealthSeriesFromLogs(rangeMeta.value)
      healthFromLogs.value = true
    }

    loaded.value = true
    await nextTick()
    renderCharts()
  } finally {
    loading.value = false
  }
}

watch(loaded, async (value) => {
  if (!value) return
  await nextTick()
  renderCharts()
})

onMounted(() => {
  void load()
  window.addEventListener('resize', resizeCharts)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts)
  disposeCharts()
})
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.range-seg {
  display: inline-flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: #fff;
}
.range-chip {
  height: 30px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font: inherit;
}
.range-chip.on {
  background: var(--primary-soft);
  color: var(--primary-ink);
  font-weight: 700;
}

.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
}
.kpi-strip {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  overflow: hidden;
}
.kpi-cell {
  padding: 16px 14px;
  border-right: 1px solid var(--border);
}
.kpi-cell:last-child { border-right: 0; }
.kpi-label { color: var(--muted); font-size: 12px; }
.kpi-value {
  margin-top: 8px;
  font-size: 22px;
  font-weight: 800;
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
}
.kpi-spark { height: 36px; margin-top: 8px; }
.chart-grid {
  display: grid;
  grid-template-columns: 1.45fr 1fr;
  gap: 14px;
}
.chart-card, .bill-card, .health-card, .alert-card { min-width: 0; }
.sec-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px 0;
}
.sec-hd h3 { margin: 0; font-size: 15px; }
.chart { height: 300px; padding: 0 4px 8px; }
.chart.cost, .chart.fail { height: 220px; }
.chart.fail { height: 140px; margin-top: 8px; padding: 0; }


.bottom-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px;
}
.bill-list { padding: 8px 10px 12px; }
.bill-row {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 12px;
  border-radius: 12px;
}
.bill-row:hover { background: #f8faf9; }
.bill-row strong { display: block; }
.bill-row small { color: var(--muted); }
.bill-row b { font-variant-numeric: tabular-nums; }
.bill-row.total { border-top: 1px solid var(--border); margin-top: 4px; border-radius: 0; }
.alert-card {
  padding: 16px;
  border-left: 3px solid #b54708;
  background: linear-gradient(90deg, #fffaeb, #fff);
  border-radius: var(--radius);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}
.alert-card.quiet {
  border-left-color: var(--primary);
  background: linear-gradient(90deg, var(--primary-soft), #fff);
}
.alert-card h4 { margin: 0 0 6px; }
.alert-card p { margin: 0; color: var(--muted); font-size: 13px; }

@media (max-width: 1100px) {
  .kpi-strip { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .kpi-cell:nth-child(3n) { border-right: 0; }
  .kpi-cell { border-bottom: 1px solid var(--border); }
  .chart-grid, .bottom-grid { grid-template-columns: 1fr; }
  .health-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

.health-card { padding: 4px 4px 10px; }
.health-hd {
  align-items: flex-start;
  gap: 16px;
  padding: 14px 16px 8px;
}
.health-sub {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
}
.hlegend {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--muted);
  font-size: 12px;
  justify-content: flex-end;
}
.hlegend span { display: inline-flex; align-items: center; gap: 6px; }
.hlegend i {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  display: inline-block;
}
.hlegend .ok { background: #22c55e; }
.hlegend .degraded { background: #f59e0b; }
.hlegend .down { background: #ef4444; }
.hlegend .off { background: #e5e7eb; }
.health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  padding: 4px 12px 14px;
}
.hcard {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: #fff;
  min-width: 0;
  box-shadow: 0 1px 2px rgba(16, 24, 40, 0.03);
}
.hcard:hover {
  border-color: #d7dde5;
  box-shadow: 0 4px 14px rgba(16, 24, 40, 0.05);
}
.hcard-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}
.hleft {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.hcheck {
  width: 20px;
  height: 20px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  color: #fff;
}
.hcheck.ok { background: #22c55e; }
.hcheck.warn { background: #f59e0b; }
.hcheck.bad { background: #ef4444; }
.hcheck.mute { background: #d1d5db; color: #6b7280; }
.htitle {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}
.htitle strong {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: -0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.htitle .meta {
  color: var(--muted);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.hright {
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  flex: 0 0 auto;
  padding-top: 1px;
}
.hright b {
  color: #111827;
  font-weight: 650;
  font-size: 12px;
}
.hbar {
  display: grid;
  gap: 2px;
  height: 22px;
  align-items: stretch;
}
.hbar :deep(.el-tooltip__trigger),
.hbar i {
  display: block;
  min-width: 0;
  height: 100%;
  border-radius: 2px;
  background: #22c55e;
  cursor: default;
  transition: filter .12s ease, transform .12s ease;
}
.hbar i.ok { background: #22c55e; }
.hbar i.degraded { background: #f59e0b; }
.hbar i.down { background: #ef4444; }
.hbar i.off { background: #e5e7eb; }
.hbar :deep(.el-tooltip__trigger:hover) i,
.hbar i:hover {
  filter: brightness(0.92);
  transform: translateY(-1px);
}
.empty-health { padding: 18px 16px; }

@media (max-width: 640px) {
  .kpi-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .kpi-cell:nth-child(2n) { border-right: 0; }
  .health-grid { grid-template-columns: 1fr; }
  .hbar { height: 20px; gap: 1.5px; }
  .hlegend { justify-content: flex-start; }
}
</style>
