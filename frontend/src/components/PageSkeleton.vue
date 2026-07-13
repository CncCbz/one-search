<template>
  <div class="page-skeleton" :class="`is-${type}`" aria-hidden="true">
    <template v-if="type === 'dashboard'">
      <div class="sk-metrics">
        <div v-for="n in 6" :key="n" class="sk-block sk-metric" />
      </div>
      <div class="sk-split">
        <div class="sk-panel">
          <div class="sk-line w-30" />
          <div v-for="n in 5" :key="n" class="sk-line" />
        </div>
        <div class="sk-panel">
          <div class="sk-line w-30" />
          <div v-for="n in 4" :key="'b'+n" class="sk-line" />
        </div>
      </div>
    </template>

    <template v-else-if="type === 'cards'">
      <div class="sk-cards">
        <div v-for="n in 4" :key="n" class="sk-panel sk-card">
          <div class="sk-card-top">
            <div class="sk-avatar" />
            <div class="sk-card-text">
              <div class="sk-line w-40" />
              <div class="sk-line w-60" />
            </div>
          </div>
          <div class="sk-card-metrics">
            <div class="sk-block sk-mini" />
            <div class="sk-block sk-mini" />
            <div class="sk-block sk-mini" />
            <div class="sk-block sk-mini" />
          </div>
        </div>
      </div>
    </template>

    <template v-else-if="type === 'playground'">
      <div class="sk-panel sk-search">
        <div class="sk-search-row">
          <div class="sk-line tall grow" />
          <div class="sk-chip action" />
        </div>
        <div class="sk-tools">
          <div class="sk-chip" />
          <div class="sk-chip" />
          <div class="sk-chip" />
          <div class="sk-chip wide" />
          <div class="sk-chip" />
        </div>
      </div>
    </template>

    <template v-else-if="type === 'form'">
      <div class="sk-form-grid">
        <div v-for="n in 4" :key="n" class="sk-panel">
          <div class="sk-line w-30" />
          <div v-for="i in 4" :key="i" class="sk-form-row">
            <div class="sk-line w-30" />
            <div class="sk-line w-50" />
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="sk-panel">
        <div class="sk-table-head">
          <div v-for="n in 5" :key="n" class="sk-line" />
        </div>
        <div v-for="n in rows" :key="n" class="sk-table-row">
          <div class="sk-line w-40" />
          <div class="sk-line w-50" />
          <div class="sk-line w-30" />
          <div class="sk-line w-20" />
          <div class="sk-line w-20" />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  type?: 'dashboard' | 'cards' | 'table' | 'form' | 'playground'
  rows?: number
}>(), {
  type: 'table',
  rows: 6
})
</script>

<style scoped>
.page-skeleton { width: 100%; }
.sk-block,
.sk-line,
.sk-avatar,
.sk-chip {
  background: linear-gradient(90deg, #eef1f4 25%, #f7f8fa 37%, #eef1f4 63%);
  background-size: 400% 100%;
  animation: sk-shine 1.2s ease infinite;
  border-radius: 8px;
}
.sk-line {
  height: 12px;
  margin-top: 12px;
  width: 100%;
}
.sk-line.tall { height: 40px; margin-top: 0; }
.sk-line.w-20 { width: 20%; }
.sk-line.w-30 { width: 30%; }
.sk-line.w-40 { width: 40%; }
.sk-line.w-50 { width: 50%; }
.sk-line.w-60 { width: 60%; }
.sk-line.w-80 { width: 80%; }
.sk-panel {
  border: 1px solid var(--border);
  background: var(--card);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 16px;
}
.sk-metrics {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}
.sk-metric { height: 88px; border-radius: 14px; }
.sk-split {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: 14px;
}
.sk-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}
.sk-card-top { display: flex; gap: 12px; align-items: center; }
.sk-avatar { width: 42px; height: 42px; border-radius: 12px; flex: 0 0 auto; }
.sk-card-text { flex: 1; min-width: 0; }
.sk-card-text .sk-line:first-child { margin-top: 0; }
.sk-card-metrics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 14px;
}
.sk-mini { height: 58px; }
.sk-search { width: 100%; max-width: none; margin: 0; }
.sk-search-row { display: grid; grid-template-columns: 1fr 88px; gap: 10px; align-items: center; }
.sk-line.grow { width: 100%; margin-top: 0; }
.sk-tools { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.sk-chip { width: 72px; height: 28px; border-radius: 999px; }
.sk-chip.wide { width: 160px; flex: 1; min-width: 120px; }
.sk-chip.action { width: 88px; height: 40px; border-radius: 10px; }
.sk-form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.sk-form-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
}
.sk-form-row .sk-line { margin-top: 0; }
.sk-table-head,
.sk-table-row {
  display: grid;
  grid-template-columns: 1.4fr 1.2fr 1fr 0.7fr 0.7fr;
  gap: 12px;
  align-items: center;
}
.sk-table-head .sk-line { margin-top: 0; height: 10px; opacity: 0.7; }
.sk-table-row { margin-top: 14px; }
.sk-table-row .sk-line { margin-top: 0; }

@keyframes sk-shine {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

@media (max-width: 980px) {
  .sk-metrics { grid-template-columns: repeat(2, 1fr); }
  .sk-split,
  .sk-form-grid { grid-template-columns: 1fr; }
}
</style>
