<script setup lang="ts">
/** 左栏：地点列表 / 时间线 / 未定位照片 三个视图。 */
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo } from '@/api/client';
import { formatDate, formatDayLabel } from '@/lib/format';
import { useGalleryStore } from '@/stores/gallery';

const emit = defineEmits<{
  (event: 'select-place', id: string): void;
  (event: 'open-photo', photo: Photo): void;
  (event: 'place-unplaced'): void;
  (event: 'add-photos'): void;
}>();

const gallery = useGalleryStore();
const { t, locale } = useI18n();

const tab = ref<'places' | 'timeline' | 'unplaced'>('places');

function coverOf(placeId: string): Photo | null {
  return gallery.photos.find((photo) => photo.place_id === placeId) ?? null;
}

/** 按「天」把照片分组，用于时间线视图 */
const timeline = computed(() => {
  const buckets = new Map<string, Photo[]>();
  for (const photo of gallery.photos) {
    const iso = photo.taken_at ?? photo.created_at;
    const key = iso ? iso.slice(0, 10) : 'unknown';
    const list = buckets.get(key);
    if (list) list.push(photo);
    else buckets.set(key, [photo]);
  }
  return Array.from(buckets.entries())
    .sort((a, b) => (a[0] < b[0] ? 1 : -1))
    .map(([key, list]) => ({ key, photos: list }));
});
</script>

<template>
  <aside class="sidebar">
    <el-tabs v-model="tab" stretch class="sidebar-tabs">
      <el-tab-pane :label="t('tabs.places')" name="places" />
      <el-tab-pane :label="t('tabs.timeline')" name="timeline" />
      <el-tab-pane name="unplaced">
        <template #label>
          <el-badge
            :value="gallery.unplacedPhotos.length"
            :hidden="gallery.unplacedPhotos.length === 0"
            class="tab-badge"
          >
            {{ t('tabs.unplaced') }}
          </el-badge>
        </template>
      </el-tab-pane>
    </el-tabs>

    <el-scrollbar class="sidebar-body">
      <el-skeleton v-if="gallery.loading && gallery.places.length === 0" :rows="5" animated />

      <template v-else-if="tab === 'places'">
        <el-empty v-if="gallery.places.length === 0" :description="t('place.empty')" :image-size="72" />
        <div v-else class="place-list">
          <div
            v-for="place in gallery.places"
            :key="place.id"
            class="place-card"
            :class="{ 'is-active': place.id === gallery.selectedPlaceId }"
            @click="emit('select-place', place.id)"
          >
            <div class="place-thumb">
              <img v-if="coverOf(place.id)" :src="coverOf(place.id)!.thumb_url" alt="" loading="lazy" />
              <span v-else>📍</span>
            </div>
            <div class="place-text">
              <span class="place-name">
                {{ place.name || t('place.unnamed') }}
                <el-tag v-if="!place.geocoded" size="small" type="primary" effect="light">
                  {{ t('place.needName') }}
                </el-tag>
              </span>
              <span class="place-sub">
                {{
                  [
                    formatDate(place.date_from ?? place.created_at, locale),
                    t('place.photoCount', { n: place.photo_count ?? 0 }),
                  ].join(' · ')
                }}
              </span>
            </div>
          </div>
        </div>
      </template>

      <template v-else-if="tab === 'timeline'">
        <el-empty v-if="gallery.photos.length === 0" :description="t('photo.empty')" :image-size="72" />
        <section v-for="day in timeline" v-else :key="day.key" class="day-group">
          <h4>{{ formatDayLabel(day.photos[0].taken_at ?? day.photos[0].created_at, locale) }}</h4>
          <div class="thumb-grid">
            <el-image
              v-for="photo in day.photos"
              :key="photo.id"
              :src="photo.thumb_url"
              :alt="photo.caption || photo.original_name"
              fit="cover"
              lazy
              @click="emit('open-photo', photo)"
            />
          </div>
        </section>
      </template>

      <template v-else>
        <el-empty
          v-if="gallery.unplacedPhotos.length === 0"
          :description="t('photo.noUnplaced')"
          :image-size="72"
        />
        <template v-else>
          <el-alert :title="t('photo.unplacedHint')" type="info" :closable="false" show-icon class="mb" />
          <div class="sidebar-actions">
            <el-button type="primary" @click="emit('place-unplaced')">
              {{ t('photo.placeOnMap') }}
            </el-button>
            <el-button @click="emit('add-photos')">{{ t('upload.add') }}</el-button>
          </div>
          <div class="thumb-grid">
            <el-image
              v-for="photo in gallery.unplacedPhotos"
              :key="photo.id"
              :src="photo.thumb_url"
              :alt="photo.original_name"
              fit="cover"
              lazy
              @click="emit('open-photo', photo)"
            />
          </div>
        </template>
      </template>
    </el-scrollbar>
  </aside>
</template>

<style scoped>
.sidebar-tabs {
  flex: none;
  padding: 0 8px;
}

.sidebar-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.sidebar-tabs :deep(.el-tabs__item) {
  font-size: 13px;
}

.tab-badge :deep(.el-badge__content) {
  transform: translateY(-6px) translateX(4px);
}

.sidebar-body {
  flex: 1;
  min-height: 0;
}

.sidebar-body :deep(.el-scrollbar__view) {
  padding: 10px;
}

.sidebar-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.mb {
  margin-bottom: 12px;
}
</style>
