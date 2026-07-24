<template>
  <router-view v-if="$route.path === '/login'" />
  <div v-else class="app-shell">
    <aside class="float-nav" aria-label="主导航">
      <router-link class="float-brand" to="/playground" title="One Search">
        <img class="float-mark" src="/icon-192.png" alt="" width="28" height="28" />
        <strong>One Search</strong>
      </router-link>

      <el-menu router :default-active="activeMenu" class="float-menu">
        <el-menu-item index="/playground" title="搜索调试">
          <el-icon><Search /></el-icon>
          <span>搜索调试</span>
        </el-menu-item>
        <el-menu-item index="/" title="仪表盘">
          <el-icon><Odometer /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/providers" title="平台管理">
          <el-icon><Grid /></el-icon>
          <span>平台管理</span>
        </el-menu-item>
        <el-menu-item index="/tokens" title="接口令牌">
          <el-icon><Key /></el-icon>
          <span>接口令牌</span>
        </el-menu-item>
        <el-menu-item index="/logs" title="请求日志">
          <el-icon><Document /></el-icon>
          <span>请求日志</span>
        </el-menu-item>
        <el-menu-item index="/audit" title="审计日志">
          <el-icon><List /></el-icon>
          <span>审计日志</span>
        </el-menu-item>
        <el-menu-item index="/settings" title="系统设置">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
      </el-menu>

      <div class="float-foot">
        <div class="float-user" title="admin">
          <span class="float-avatar">AD</span>
          <span class="txt">admin</span>
        </div>
        <button class="float-logout" type="button" title="退出" @click="logout">
          <el-icon :size="18"><SwitchButton /></el-icon>
          <span class="txt">退出</span>
        </button>
      </div>
    </aside>

    <main class="page-main">
      <router-view v-slot="{ Component }">
        <keep-alive include="PlaygroundView">
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Document,
  Grid,
  Key,
  List,
  Odometer,
  Search,
  Setting,
  SwitchButton
} from '@element-plus/icons-vue'
import { api } from './api/client'
import { useSessionStore } from './stores/session'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()

const activeMenu = computed(() => {
  if (route.path.startsWith('/keys')) return '/providers'
  return route.path
})

async function logout() {
  try {
    await api.logout()
  } finally {
    session.logout()
    router.push('/login')
  }
}
</script>
