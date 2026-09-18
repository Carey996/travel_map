<script setup lang="ts">
/** 登录 / 注册页。会话由后端用 HttpOnly Cookie 维护，这里只负责收集凭据。 */
import type { FormInstance, FormRules } from 'element-plus';
import { computed, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import { SUPPORTED_LANGS, setLang, type Lang } from '@/i18n';
import { useAuthStore } from '@/stores/auth';

const emit = defineEmits<{ (event: 'authenticated'): void }>();

const { t, locale } = useI18n();
const auth = useAuthStore();

const mode = ref<'login' | 'register'>('login');
const formRef = ref<FormInstance>();
const error = ref('');

const form = reactive({ email: '', password: '', displayName: '' });

const rules = computed<FormRules>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.emailInvalid'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), trigger: 'blur' },
    ...(mode.value === 'register'
      ? [{ min: 8, message: t('auth.passwordHint'), trigger: 'blur' }]
      : []),
  ],
}));

async function submit(): Promise<void> {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;

  error.value = '';
  try {
    if (mode.value === 'login') {
      await auth.login(form.email, form.password);
    } else {
      await auth.register(form.email, form.password, form.displayName);
    }
    emit('authenticated');
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : String(caught);
  }
}

function switchMode(next: 'login' | 'register'): void {
  mode.value = next;
  error.value = '';
  formRef.value?.clearValidate();
}

function switchLang(value: string | number | boolean | undefined): void {
  if (value === 'zh' || value === 'en') setLang(value as Lang);
}
</script>

<template>
  <div class="login-page">
    <el-card class="login-card" shadow="always">
      <div class="login-head">
        <div>
          <h1>{{ t('app.title') }}</h1>
          <p class="muted">{{ t('app.subtitle') }}</p>
        </div>
        <el-radio-group
          :model-value="locale === 'zh' ? 'zh' : 'en'"
          size="small"
          @change="switchLang"
        >
          <el-radio-button v-for="code in SUPPORTED_LANGS" :key="code" :value="code">
            {{ code === 'zh' ? '中文' : 'EN' }}
          </el-radio-button>
        </el-radio-group>
      </div>

      <el-radio-group
        :model-value="mode"
        class="mode-switch"
        @change="(value: string | number | boolean | undefined) => switchMode(value as 'login' | 'register')"
      >
        <el-radio-button value="login">{{ t('auth.login') }}</el-radio-button>
        <el-radio-button value="register">{{ t('auth.register') }}</el-radio-button>
      </el-radio-group>

      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon class="mb" />

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="submit"
      >
        <el-form-item :label="t('auth.email')" prop="email">
          <el-input v-model="form.email" type="email" autocomplete="email" />
        </el-form-item>

        <el-form-item v-if="mode === 'register'" :label="t('auth.displayName')" prop="displayName">
          <el-input v-model="form.displayName" />
        </el-form-item>

        <el-form-item :label="t('auth.password')" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :autocomplete="mode === 'register' ? 'new-password' : 'current-password'"
          />
        </el-form-item>

        <el-button
          type="primary"
          native-type="submit"
          class="submit"
          :loading="auth.submitting"
        >
          {{ mode === 'login' ? t('auth.signIn') : t('auth.signUp') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.mode-switch {
  width: 100%;
  margin-bottom: 18px;
}

.mode-switch :deep(.el-radio-button),
.mode-switch :deep(.el-radio-button__inner) {
  width: 100%;
}

.mb {
  margin-bottom: 14px;
}

.submit {
  width: 100%;
}
</style>
