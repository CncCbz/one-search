import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from '../stores/session'

const LoginView = () => import('../views/LoginView.vue')
const DashboardView = () => import('../views/DashboardView.vue')
const ProvidersView = () => import('../views/ProvidersView.vue')
const TokensView = () => import('../views/TokensView.vue')
const PlaygroundView = () => import('../views/PlaygroundView.vue')
const LogsView = () => import('../views/LogsView.vue')
const AuditLogsView = () => import('../views/AuditLogsView.vue')
const SettingsView = () => import('../views/SettingsView.vue')

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
  if (to.path === '/login' && session.token) return '/playground'
})

export default router
