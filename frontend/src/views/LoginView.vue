<template>
  <div class="login-page">
    <el-card class="login-card soft-card" shadow="never">
      <div class="login-logo">搜</div>
      <h2>一搜中转</h2>
      <p class="muted">搜索中转控制台 · 管理员登录</p>
      <el-form label-position="top" @submit.prevent="login">
        <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        <el-button type="primary" :loading="loading" class="full" @click="login">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus/es/components/message/index'
import { api } from '../api/client'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const session = useSessionStore()
const loading = ref(false)
const form = reactive({ username: '', password: '' })

async function login() {
  loading.value = true
  try {
    const result = await api.login(form.username, form.password)
    session.setToken(result.token)
    router.push('/')
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
.login-card { width: 390px; text-align: center; }
.login-logo { width: 56px; height: 56px; display: grid; place-items: center; margin: 0 auto 12px; border-radius: var(--el-border-radius-base); background: var(--primary); color: #fff; font-weight: 900; font-size: 22px; }
.full { width: 100%; }
</style>
