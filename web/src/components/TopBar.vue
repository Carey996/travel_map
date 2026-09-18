<script setup lang="ts">
/** 顶栏：地名搜索、新建地点、上传、底图切换、语言与账号。 */
import { ref } from 'vue';
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

    <el-autocomplete
      v-model="query"
      class="search"
      :fetch-suggestions="fetchSuggestions"
      :trigger-on-focus="false"
      :debounce="350"
      clearable
      :placeholder="t('map.searchPlaceholder')"
      @select="onSelectSearch"
    >
      <template #default="{ item }">
        <div class="suggestion">
          <span class="suggestion-name">{{ item.value }}</span>
          <small class="suggestion-sub">{{ item.display_name }}</small>
        </div>
      </template>
    </el-autocomplete>

    <div class="topbar-actions">
      <el-button
        :type="props.placing ? 'primary' : 'default'"
        :plain="props.placing"
        @click="emit('toggle-placing')"
      >
        <el-icon><Location /></el-icon>
        <span>{{ props.placing ? t('map.clickToPlace') : t('map.newPlace') }}</span>
      </el-button>

      <PhotoUploader @uploaded="emit('uploaded')" />

      <el-select v-model="tileProviderId" style="width: 148px" :title="t('map.layers')">
        <el-option
          v-for="provider in TILE_PROVIDERS"
          :key="provider.id"
          :value="provider.id"
          :label="locale === 'zh' ? provider.labelZh : provider.labelEn"
        />
      </el-select>

      <el-checkbox v-model="showPhotoDots" :title="t('map.showPhotoDots')">
        {{ t('map.dots') }}
      </el-checkbox>

      <el-button @click="emit('fit-all')">{{ t('map.fitAll') }}</el-button>

      <el-radio-group :model-value="lang()" size="small" @change="switchLang">
        <el-radio-button v-for="code in SUPPORTED_LANGS" :key="code" :value="code">
          {{ code === 'zh' ? '中文' : 'EN' }}
        </el-radio-button>
      </el-radio-group>

      <el-dropdown trigger="click">
        <el-button text>
          {{ auth.displayName }}
          <el-icon><ArrowDown /></el-icon>
        </el-button>
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
  padding: 8px 12px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
  z-index: 1200;
}

.search {
  flex: 1;
  max-width: 420px;
}

.suggestion {
  display: flex;
  flex-direction: column;
  line-height: 1.35;
  padding: 2px 0;
}

.suggestion-sub {
  color: var(--el-text-color-secondary);
  font-size: 11.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
  flex-wrap: wrap;
}

@media (max-width: 900px) {
  .topbar {
    flex-wrap: wrap;
  }

  .search {
    order: 3;
    flex-basis: 100%;
    max-width: none;
  }

  .topbar-actions {
    margin-left: 0;
  }
}
</style>
