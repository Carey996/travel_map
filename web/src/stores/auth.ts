import { computed, ref } from 'vue';
import { defineStore } from 'pinia';

import { ApiError, api, type User } from '@/api/client';

/**
 * 登录态。
 *
 * 会话本身由后端通过 HttpOnly Cookie 维护，这里只缓存「当前用户是谁」这一份展示数据；
 * 刷新页面时靠 restore() 调 /auth/me 重新确认。
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null);
  const checking = ref(true);
  const submitting = ref(false);

  const isAuthenticated = computed(() => user.value !== null);
  const displayName = computed(() => user.value?.display_name || user.value?.email || '');

  async function restore(): Promise<void> {
    checking.value = true;
    try {
      user.value = await api.me();
    } catch (error) {
      // 401 表示「还没登录」，属于正常状态，不当作错误
      if (!(error instanceof ApiError && error.status === 401)) {
        console.error('[auth] 恢复登录态失败', error);
      }
      user.value = null;
    } finally {
      checking.value = false;
    }
  }

  async function login(email: string, password: string): Promise<void> {
    submitting.value = true;
    try {
      user.value = await api.login(email, password);
    } finally {
      submitting.value = false;
    }
  }

  async function register(email: string, password: string, displayName: string): Promise<void> {
    submitting.value = true;
    try {
      user.value = await api.register(email, password, displayName);
    } finally {
      submitting.value = false;
    }
  }

  async function logout(): Promise<void> {
    await api.logout().catch(() => undefined);
    user.value = null;
  }

  return {
    user,
    checking,
    submitting,
    isAuthenticated,
    displayName,
    restore,
    login,
    register,
    logout,
  };
});
