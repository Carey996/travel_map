<script setup lang="ts">
/** 顶栏：地名搜索、新建地点、上传、底图切换、语言与账号。 */
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { ElMessage } from 'element-plus';

import { api, type GeoSearchItem } from '@/api/client';
import { useAuthStore } from '@/stores/auth';
import { useGalleryStore } from '@/stores/gallery';
import { TILE_PROVIDERS } from '@/lib/tiles';
import { setLang, SUPPORTED_LANGS, type Lang } from '@/i18n';
import PhotoUploader from './PhotoUploader.vue';

const props = defineProps<{ placing: boolean }>();
const emit = defineEmits<{
  (event: 'toggle-placing'): void;
  (event: 'fit-all'): void;
  (event: 'uploaded'): void;
  (event: 'focus', lat: number, lng: number): void;
}>();

const tileProviderId = defineModel<string>('tileProviderId', { required: true });
const showPhotoDots = defineModel<boolean>('showPhotoDots', { required: true });

const { t, locale } = useI18n();
const auth = useAuthStore();
const gallery = useGalleryStore();

const query = ref('');

/** 头像里显示的首字母：优先昵称，其次邮箱 */
const initial = computed(() => {
  const source = auth.displayName || auth.user?.email || '?';
  return source.trim().charAt(0).toUpperCase();
});

/** el-autocomplete 需要 { value, ... } 形状的建议项 */
async function fetchSuggestions(keyword: string, callback: (items: unknown[]) => void): Promise<void> {
  if (keyword.trim().length < 2) {
    callback([]);
    return;
  }
  try {
    const items = await api.searchPlaces(keyword, 6);
    callback(items.map((item) => ({ ...item, value: item.name })));
  } catch {
    callback([]);
  }
}

async function onSelectSearch(item: GeoSearchItem & { value: string }): Promise<void> {
  query.value = '';
  emit('focus', item.lat, item.lng);
  try {
    await gallery.createPlace({
      lat: item.lat,
      lng: item.lng,
      name: item.name,
      kind: item.kind,
      address: item.display_name,
    });
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}

const lang = () => (locale.value === 'zh' ? 'zh' : 'en');

function switchLang(value: string | number | boolean | undefined): void {
  if (value === 'zh' || value === 'en') setLang(value as Lang);
}

async function onLogout(): Promise<void> {
  await auth.logout();
  gallery.reset();
}
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <span class="brand-icon">🗺</span>
      <span class="brand-name">{{ t('app.title') }}</span>
    </div>

    <div class="tb-search">
      <el-autocomplete
        v-model="query"
        :fetch-suggestions="fetchSuggestions"
        :trigger-on-focus="false"
        :debounce="350"
        clearable
        :placeholder="t('map.searchPlaceholder')"
        @select="onSelectSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
        <template #default="{ item }">
          <div class="suggestion">
            <span class="suggestion-name">{{ item.value }}</span>
            <small class="suggestion-sub">{{ item.display_name }}</small>
          </div>
        </template>
      </el-autocomplete>
    </div>

    <div class="tb-group">
      <el-button
        :type="props.placing ? 'primary' : 'default'"
        :plain="props.placing"
        @click="emit('toggle-placing')"
      >
        <el-icon><Location /></el-icon>
        <span>{{ props.placing ? t('map.clickToPlace') : t('map.newPlace') }}</span>
      </el-button>

      <PhotoUploader @uploaded="emit('uploaded')" />
    </div>

    <div class="tb-spacer" />

    <div class="tb-group tb-tools">
      <el-tooltip :content="t('map.showPhotoDots')" placement="bottom">
        <el-checkbox v-model="showPhotoDots" class="tb-check">
          <el-icon><Picture /></el-icon>
          <span class="tb-label">{{ t('map.dots') }}</span>
        </el-checkbox>
      </el-tooltip>

      <el-tooltip :content="t('map.fitAll')" placement="bottom">
        <el-button circle @click="emit('fit-all')">
          <el-icon><FullScreen /></el-icon>
        </el-button>
      </el-tooltip>

      <el-tooltip :content="t('map.layers')" placement="bottom">
        <el-select v-model="tileProviderId" class="tb-layers">
          <el-option
            v-for="provider in TILE_PROVIDERS"
            :key="provider.id"
            :value="provider.id"
            :label="locale === 'zh' ? provider.labelZh : provider.labelEn"
          />
        </el-select>
      </el-tooltip>

      <el-divider direction="vertical" class="tb-divider" />

      <el-radio-group :model-value="lang()" size="small" @change="switchLang">
        <el-radio-button v-for="code in SUPPORTED_LANGS" :key="code" :value="code">
          {{ code === 'zh' ? '中文' : 'EN' }}
        </el-radio-button>
      </el-radio-group>

      <el-dropdown trigger="click">
        <button class="tb-user" type="button">
          <el-avatar :size="28" class="tb-avatar">{{ initial }}</el-avatar>
          <span class="tb-label tb-user-name">{{ auth.displayName }}</span>
          <el-icon class="tb-caret"><ArrowDown /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item disabled>{{ auth.user?.email }}</el-dropdown-item>
            <el-dropdown-item divided @click="onLogout">{{ t('auth.signOut') }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  flex: none;
  display: flex;
  align-items: center;
  gap: 10px;
  height: var(--tb-topbar-h);
  padding: 0 14px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
  z-index: 1200;
}

.tb-search {
  flex: none;
  width: 320px;
}

.tb-search :deep(.el-autocomplete) {
  width: 100%;
}

.topbar :deep(.tb-search .el-input__wrapper) {
  border-radius: 999px;
  background: var(--el-fill-color-light);
  box-shadow: none;
  padding-left: 12px;
}

.topbar :deep(.tb-search .el-input__wrapper.is-focus) {
  background: var(--el-bg-color);
  box-shadow: 0 0 0 1px var(--el-color-primary) inset;
}

.topbar :deep(.tb-search .el-input__prefix) {
  color: var(--el-text-color-secondary);
}

.tb-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 顶栏里的东西一律不压缩、不换行——一旦被挤，Element 的控件会自己折成两行 */
.topbar > *,
.tb-group > * {
  flex: none;
}

.tb-group :deep(.el-radio-button__inner),
.tb-group :deep(.el-checkbox__label),
.tb-group :deep(.el-button > span) {
  white-space: nowrap;
}

.tb-spacer {
  flex: 1 1 auto;
  min-width: 4px;
}

.tb-divider {
  margin: 0 2px;
}

.topbar :deep(.tb-layers) {
  width: 158px;
}

.topbar :deep(.tb-check) {
  height: 32px;
  margin-right: 2px;
}

.topbar :deep(.tb-check .el-checkbox__label) {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding-left: 7px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

/* 账号入口：整块可点，比纯文字按钮更像一个「入口」 */
.tb-user {
  display: flex;
  align-items: center;
  gap: 7px;
  white-space: nowrap;
  height: 36px;
  padding: 0 8px 0 4px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--el-text-color-primary);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.16s, border-color 0.16s;
}

.tb-user:hover {
  background: var(--el-fill-color-light);
  border-color: var(--el-border-color-lighter);
}

.tb-avatar {
  background: var(--el-color-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

.tb-caret {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.suggestion {
  display: flex;
  flex-direction: column;
  line-height: 1.35;
  padding: 2px 0;
}

.suggestion-name {
  font-size: 13.5px;
}

.suggestion-sub {
  color: var(--el-text-color-secondary);
  font-size: 11.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1320px) {
  .tb-search {
    width: 230px;
  }
  .tb-layers {
    width: 120px;
  }
}

@media (max-width: 1120px) {
  .tb-label {
    display: none;
  }
  .tb-search {
    width: 190px;
  }
}

@media (max-width: 900px) {
  .topbar {
    flex-wrap: wrap;
    height: auto;
    padding: 8px 12px;
  }
  .tb-search {
    order: 9;
    width: 100%;
  }
  .tb-spacer {
    display: none;
  }
  .tb-tools {
    margin-left: auto;
  }
}
</style>
