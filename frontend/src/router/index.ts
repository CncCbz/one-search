import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from '../stores/session'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'
import ProvidersView from '../views/ProvidersView.vue'
import TokensView from '../views/TokensView.vue'
import PlaygroundView from '../views/PlaygroundView.vue'
import LogsView from '../views/LogsView.vue'
import AuditLogsView from '../views/AuditLogsView.vue'
import SettingsView from '../views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView, meta: { public: true } },
    { path: '/', component: DashboardView },
    { path: '/providers', component: ProvidersView },
    { path: '/keys', redirect: '/providers' },
    { path: '/tokens', component: TokensView },
    { path: '/playground', component: PlaygroundView },
    { path: '/logs', component: LogsView },
    { path: '/audit', component: AuditLogsView },
    { path: '/usage', redirect: '/' },
    { path: '/settings', component: SettingsView }
  ]
})

router.beforeEach((to) => {
  const session = useSessionStore()
  if (!to.meta.public && !session.token) return '/login'
  if (to.path === '/login' && session.token) return '/'
})

export default router
