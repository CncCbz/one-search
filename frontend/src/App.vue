<template>
  <router-view v-if="$route.path === '/login'" />
  <el-container v-else class="layout-shell">
    <el-aside width="232px" class="side-nav">
      <div class="brand-block">
        <div class="brand-logo">搜</div>
        <div>
          <div class="brand-title">一搜中转</div>
          <div class="brand-subtitle">搜索中转控制台</div>
        </div>
      </div>
      <el-menu router :default-active="$route.path" class="nav-menu">
        <el-menu-item index="/">仪表盘</el-menu-item>
        <el-menu-item index="/providers">平台管理</el-menu-item>
        <el-menu-item index="/tokens">接口令牌</el-menu-item>
        <el-menu-item index="/playground">搜索调试</el-menu-item>
        <el-menu-item index="/logs">请求日志</el-menu-item>
        <el-menu-item index="/audit">审计日志</el-menu-item>
        <el-menu-item index="/settings">系统设置</el-menu-item>
      </el-menu>
      <div class="nav-footer">
        <el-button :icon="SwitchButton" class="logout-button" title="退出登录" @click="logout">
          退出登录
        </el-button>
      </div>
    </el-aside>
    <el-container>
      <el-main class="page-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { SwitchButton } from '@element-plus/icons-vue'
import { useSessionStore } from './stores/session'

const router = useRouter()
const session = useSessionStore()
function logout() {
  session.logout()
  router.push('/login')
}
</script>
