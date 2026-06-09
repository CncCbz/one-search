import { defineStore } from 'pinia'

const TOKEN_KEY = 'one-search-admin-token'

export const useSessionStore = defineStore('session', {
  state: () => ({ token: localStorage.getItem(TOKEN_KEY) || '' }),
  actions: {
    setToken(token: string) {
      this.token = token
      localStorage.setItem(TOKEN_KEY, token)
    },
    logout() {
      this.token = ''
      localStorage.removeItem(TOKEN_KEY)
    }
  }
})
