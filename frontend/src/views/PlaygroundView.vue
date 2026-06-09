<template>
  <div>
    <div class="page-actions"><el-button type="primary" :loading="loading" @click="run">开始搜索</el-button></div>
    <el-card class="soft-card" shadow="never">
      <el-form label-position="top">
        <el-form-item label="搜索词"><el-input v-model="form.query" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="8"><el-form-item label="搜索模式"><el-select v-model="form.mode"><el-option value="parallel" label="并发聚合" /><el-option value="fallback" label="失败转移" /><el-option value="single" label="单平台" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="搜索平台"><el-select v-model="form.providers" multiple><el-option value="exa" label="Exa" /><el-option value="you" label="You.com" /><el-option value="jina" label="Jina" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="结果数量"><el-input-number v-model="form.limit" :min="1" :max="50" /></el-form-item></el-col>
        </el-row>
      </el-form>
    </el-card>
    <el-card v-if="result" class="soft-card" shadow="never" style="margin-top:16px">
      <template #header>搜索结果</template>
      <pre class="code-box">{{ JSON.stringify(result, null, 2) }}</pre>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'
const loading = ref(false)
const result = ref<unknown>(null)
const form = reactive({ query: 'latest web search APIs', mode: 'parallel', providers: ['exa', 'you', 'jina'], limit: 10, cache: 'default' })
async function run() { loading.value = true; try { result.value = await api.playgroundSearch(form) } catch (error) { ElMessage.error((error as Error).message) } finally { loading.value = false } }
</script>
