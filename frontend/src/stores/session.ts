import { defineStore } from 'pinia'

const TOKEN_KEY = 'one-search-admin-token'

function initialToken() {
  localStorage.removeItem(TOKEN_KEY)
  return sessionStorage.getItem(TOKEN_KEY) || ''
}

export const useSessionStore = defineStore('session', {
  state: () => ({ token: initialToken() }),
  actions: {
    setToken(token: string) {
      this.token = token
      sessionStorage.setItem(TOKEN_KEY, token)
    },
    logout() {
      this.token = ''
      sessionStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(TOKEN_KEY)
    }
  }
})
