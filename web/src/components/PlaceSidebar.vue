<script setup lang="ts">
/** 左栏：地点列表 / 时间线 / 未定位照片 三个视图。 */
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo, Trip } from '@/api/client';
import { formatDate, formatDayLabel } from '@/lib/format';
import { tripColor } from '@/lib/trips';
import { useGalleryStore } from '@/stores/gallery';

const emit = defineEmits<{
  (event: 'select-place', id: string): void;
  (event: 'open-photo', photo: Photo): void;
  (event: 'place-unplaced'): void;
  (event: 'add-photos'): void;
  (event: 'focus-trip', tripId: string): void;
}>();

const gallery = useGalleryStore();
const { t, locale } = useI18n();

const tab = ref<'places' | 'trips' | 'timeline' | 'unplaced'>('places');
const filter = ref('');

function coverOf(placeId: string): Photo | null {
  return gallery.photos.find((photo) => photo.place_id === placeId) ?? null;
}

/** 本地筛选：数据量不大，直接在内存里过滤，不发请求 */
const visiblePlaces = computed(() => {
  const keyword = filter.value.trim().toLowerCase();
  if (!keyword) return gallery.places;
  return gallery.places.filter((place) =>
    [place.name, place.address ?? ''].join(' ').toLowerCase().includes(keyword),
  );
});

async function onDeleteTrip(trip: Trip): Promise<void> {
  const name = trip.title || t('trip.untitled');
  try {
    await ElMessageBox.confirm(t('trip.confirmDelete', { name }), t('common.confirm'), {
      type: 'warning',
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel'),
    });
  } catch {
    return;
  }
  try {
    await gallery.deleteTrip(trip.id);
    ElMessage.success(t('trip.deleted'));
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
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
    <div class="sidebar-head">
      <el-radio-group v-model="tab" size="default" class="tab-switch">
        <el-radio-button value="places">
          <el-icon><Location /></el-icon>
          <span>{{ t('tabs.places') }}</span>
        </el-radio-button>
        <el-radio-button value="trips">
          <el-icon><Flag /></el-icon>
          <span>{{ t('tabs.trips') }}</span>
        </el-radio-button>
        <el-radio-button value="timeline">
          <el-icon><Clock /></el-icon>
          <span>{{ t('tabs.timeline') }}</span>
        </el-radio-button>
        <el-radio-button value="unplaced">
          <el-icon><Picture /></el-icon>
          <span>{{ t('tabs.unplaced') }}</span>
          <span v-if="gallery.unplacedPhotos.length" class="tab-count">
            {{ gallery.unplacedPhotos.length }}
          </span>
        </el-radio-button>
      </el-radio-group>

      <div v-if="tab === 'places'" class="head-sub">
        <el-input
          v-model="filter"
          size="small"
          clearable
          :placeholder="t('place.filterPlaceholder')"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="head-count">{{ t('place.count', { n: gallery.places.length }) }}</span>
      </div>
    </div>

    <el-scrollbar class="sidebar-body">
      <div class="sidebar-inner">
        <el-skeleton v-if="gallery.loading && gallery.places.length === 0" :rows="5" animated />

        <template v-else-if="tab === 'places'">
          <el-empty v-if="gallery.places.length === 0" :description="t('place.empty')" :image-size="72" />
          <el-empty v-else-if="visiblePlaces.length === 0" :description="t('place.filterPlaceholder')" :image-size="60" />
          <div v-else class="place-list">
            <div
              v-for="place in visiblePlaces"
              :key="place.id"
              class="place-card"
              :class="{ 'is-active': place.id === gallery.selectedPlaceId }"
              @click="emit('select-place', place.id)"
            >
              <div class="place-thumb">
                <img v-if="coverOf(place.id)" :src="coverOf(place.id)!.thumb_url" alt="" loading="lazy" />
                <el-icon v-else><LocationFilled /></el-icon>
              </div>
              <div class="place-text">
                <span class="place-name">
                  {{ place.name || t('place.unnamed') }}
                  <el-tag v-if="!place.geocoded" size="small" type="warning" effect="light" round>
                    {{ t('place.needName') }}
                  </el-tag>
                </span>
                <span class="place-sub">
                  <span>{{ formatDate(place.date_from ?? place.created_at, locale) }}</span>
                  <span class="dot">·</span>
                  <span>{{ t('place.photoCount', { n: place.photo_count ?? 0 }) }}</span>
                </span>
              </div>
            </div>
          </div>
        </template>

        <template v-else-if="tab === 'trips'">
          <el-empty v-if="gallery.trips.length === 0" :description="t('trip.empty')" :image-size="72" />
          <div v-else class="trip-list">
            <div
              v-for="trip in gallery.trips"
              :key="trip.id"
              class="trip-card"
              @click="emit('focus-trip', trip.id)"
            >
              <span class="trip-dot" :style="{ background: tripColor(trip.id) }" />
              <div class="trip-text">
                <span class="trip-title">{{ trip.title || t('trip.untitled') }}</span>
                <span class="place-sub">
                  <span>{{ t('trip.placeCount', { n: trip.place_count }) }}</span>
                  <span class="dot">·</span>
                  <span>{{ t('trip.photoCount', { n: trip.photo_count }) }}</span>
                </span>
              </div>
              <el-button text size="small" class="trip-del" @click.stop="onDeleteTrip(trip)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </div>
        </template>

        <template v-else-if="tab === 'timeline'">
          <el-empty v-if="gallery.photos.length === 0" :description="t('photo.empty')" :image-size="72" />
          <section v-for="day in timeline" v-else :key="day.key" class="day-group">
            <h4>
              <el-icon><Calendar /></el-icon>
              {{ formatDayLabel(day.photos[0].taken_at ?? day.photos[0].created_at, locale) }}
              <span class="count">{{ t('place.totalPhotos', { n: day.photos.length }) }}</span>
            </h4>
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
                <el-icon><Aim /></el-icon>
                <span>{{ t('photo.placeOnMap') }}</span>
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
      </div>
    </el-scrollbar>
  </aside>
</template>

<style scoped>
.sidebar-head {
  flex: none;
  padding: 12px 12px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.tab-switch {
  width: 100%;
  display: flex;
}

.tab-switch :deep(.el-radio-button) {
  flex: 1;
}

.tab-switch :deep(.el-radio-button__inner) {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 8px 6px;
  font-size: 13px;
}

.tab-count {
  min-width: 17px;
  height: 17px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--el-color-primary);
  color: #fff;
  font-size: 11px;
  line-height: 17px;
  text-align: center;
}

.head-sub {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}

.head-count {
  flex: none;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.sidebar-body {
  flex: 1;
  min-height: 0;
}

.sidebar-inner {
  padding: 12px;
}

.trip-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.trip-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 10px 10px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--tb-radius);
  cursor: pointer;
  transition: background 0.16s, box-shadow 0.16s;
}

.trip-card:hover {
  background: var(--el-fill-color-light);
  box-shadow: var(--tb-shadow-1);
}

.trip-dot {
  width: 10px;
  height: 10px;
  flex: none;
  border-radius: 3px;
}

.trip-text {
  flex: 1;
  min-width: 0;
}

.trip-title {
  display: block;
  font-size: 13.5px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trip-del {
  opacity: 0;
  transition: opacity 0.16s;
}

.trip-card:hover .trip-del {
  opacity: 1;
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
