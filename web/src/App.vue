<script setup lang="ts">
/**
 * 应用外壳。
 *
 * 职责：决定展示登录页还是主界面、装配地图与侧栏/抽屉、
 * 处理「在地图上点选建点」这一跨组件的交互，以及全窗口拖拽上传。
 */
import { ElMessage } from 'element-plus';
import elementEn from 'element-plus/es/locale/lang/en';
import elementZh from 'element-plus/es/locale/lang/zh-cn';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo } from '@/api/client';
import LoginView from '@/components/LoginView.vue';
import MapCanvas, { type MapFocus } from '@/components/MapCanvas.vue';
import PhotoLightbox from '@/components/PhotoLightbox.vue';
import PlaceDrawer from '@/components/PlaceDrawer.vue';
import PlaceSidebar from '@/components/PlaceSidebar.vue';
import TopBar from '@/components/TopBar.vue';
import { DEFAULT_TILE_PROVIDER } from '@/lib/tiles';
import { uploadMany } from '@/lib/upload';
import { useAuthStore } from '@/stores/auth';
import { useGalleryStore } from '@/stores/gallery';

const { t, locale } = useI18n();
const auth = useAuthStore();
const gallery = useGalleryStore();

const tileProviderId = ref(DEFAULT_TILE_PROVIDER);
const showPhotoDots = ref(true);

/** null = 普通浏览；'place' = 点地图新建地点；'photos' = 点地图给未定位照片选位置 */
const placing = ref<'place' | 'photos' | null>(null);
const focus = ref<MapFocus | null>(null);
const fitToken = ref(0);

const dragging = ref(false);
const batch = ref<{ done: number; total: number } | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);

function onFilePicked(event: Event): void {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = '';
  if (files.length) void runBatch(files);
}

const epLocale = computed(() => (locale.value === 'zh' ? elementZh : elementEn));
const hasPlaces = computed(() => gallery.places.length > 0);

/** 拉取全部数据；顺手给还没命名的地点补上反向地理编码结果 */
async function syncAll(): Promise<void> {
  await gallery.refresh();
  const changed = await gallery.fillMissingNames();
  if (changed) await gallery.refresh();
}

onMounted(async () => {
  await auth.restore();
  if (auth.isAuthenticated) await syncAll();
});

watch(
  () => auth.isAuthenticated,
  async (authed) => {
    if (authed) await syncAll();
    else gallery.reset();
  },
);

async function onMapClick(lat: number, lng: number): Promise<void> {
  if (placing.value) {
    const mode = placing.value;
    placing.value = null;
    try {
      const place = await gallery.createPlace({ lat, lng, name: '', kind: 'spot' });
      if (mode === 'photos') await gallery.assignUnplacedTo(place.id);
      focus.value = { lat, lng, zoom: 13, token: Date.now() };
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : String(error));
    }
    return;
  }
  await gallery.selectPlace(null);
}

async function runBatch(files: File[]): Promise<void> {
  const images = files.filter(
    (file) => file.type.startsWith('image/') || /\.(heic|heif)$/i.test(file.name),
  );
  if (images.length === 0) {
    ElMessage.error(t('upload.notImage'));
    return;
  }

  batch.value = { done: 0, total: images.length };
  try {
    const outcomes = await uploadMany(images, {
      placeId: gallery.selectedPlaceId,
      concurrency: 3,
      onProgress: (done, total) => {
        batch.value = { done, total };
      },
    });

    const failed = outcomes.filter((outcome) => outcome.error);
    const skipped = outcomes.filter((outcome) => outcome.skipped).length;
    const undecodable = outcomes.filter((outcome) => outcome.undecodable).length;

    if (failed.length) {
      ElMessage.error(t('upload.partialFail', { n: failed.length, message: failed[0].error ?? '' }));
    }
    if (skipped) ElMessage.info(t('upload.skipped', { n: skipped }));
    if (undecodable) ElMessage.warning(t('upload.undecodable', { n: undecodable }));
    if (!failed.length) {
      ElMessage.success(t('upload.done', { n: outcomes.length - skipped }));
    }
    await syncAll();
  } finally {
    batch.value = null;
  }
}

function onDragOver(event: DragEvent): void {
  if (!auth.isAuthenticated) return;
  event.preventDefault();
  dragging.value = true;
}

function onDragLeave(event: DragEvent): void {
  if (event.relatedTarget === null) dragging.value = false;
}

async function onDrop(event: DragEvent): Promise<void> {
  if (!auth.isAuthenticated) return;
  event.preventDefault();
  dragging.value = false;
  const files = Array.from(event.dataTransfer?.files ?? []);
  if (files.length) await runBatch(files);
}

onMounted(() => {
  window.addEventListener('dragover', onDragOver);
  window.addEventListener('dragleave', onDragLeave);
  window.addEventListener('drop', onDrop);
});

onBeforeUnmount(() => {
  window.removeEventListener('dragover', onDragOver);
  window.removeEventListener('dragleave', onDragLeave);
  window.removeEventListener('drop', onDrop);
});

function onPhotoLocated(photo: Photo): void {
  gallery.selectedPhotoId = null;
  if (photo.lat !== null && photo.lng !== null) {
    focus.value = { lat: photo.lat, lng: photo.lng, zoom: 16, token: Date.now() };
  }
}

function onSetPhotoLocation(): void {
  gallery.selectedPhotoId = null;
  placing.value = 'photos';
  ElMessage.info(t('map.pickPointForPhotos'));
}
</script>

<template>
  <el-config-provider :locale="epLocale">
    <div v-if="auth.checking" class="boot" v-loading="true" element-loading-text="" />

    <LoginView v-else-if="!auth.isAuthenticated" />

    <div v-else class="app-shell">
      <TopBar
        v-model:tileProviderId="tileProviderId"
        v-model:showPhotoDots="showPhotoDots"
        :placing="placing === 'place'"
        @toggle-placing="placing = placing === 'place' ? null : 'place'"
        @fit-all="fitToken += 1"
        @uploaded="syncAll()"
        @focus="(lat: number, lng: number) => (focus = { lat, lng, zoom: 12, token: Date.now() })"
      />

      <div class="workspace">
        <PlaceSidebar
          @select-place="
            (id: string) => {
              gallery.selectPlace(id);
              const place = gallery.places.find((item) => item.id === id);
              if (place) focus = { lat: place.lat, lng: place.lng, zoom: 13, token: Date.now() };
            }
          "
          @open-photo="(photo: Photo) => (gallery.selectedPhotoId = photo.id)"
          @place-unplaced="
            placing = 'photos';
            ElMessage.info(t('map.pickPointForPhotos'));
          "
          @add-photos="fileInput?.click()"
        />

        <main class="map-area">
          <MapCanvas
            :places="gallery.places"
            :photos="gallery.locatedPhotos"
            :selected-place-id="gallery.selectedPlaceId"
            :tile-provider-id="tileProviderId"
            :show-photo-dots="showPhotoDots"
            :placing-mode="placing !== null"
            :focus="focus"
            :fit-token="fitToken"
            @select-place="(id: string | null) => gallery.selectPlace(id)"
            @map-click="onMapClick"
            @photo-click="(photo: Photo) => (gallery.selectedPhotoId = photo.id)"
          />

          <el-card v-if="!hasPlaces && !gallery.loading" class="map-hint" shadow="always">
            <div class="hint-icon"><el-icon><MapLocation /></el-icon></div>
            <h3>{{ t('map.emptyTitle') }}</h3>
            <p class="muted">{{ t('map.emptyBody') }}</p>
            <el-button type="primary" @click="fileInput?.click()">
              <el-icon><UploadFilled /></el-icon>
              <span>{{ t('upload.add') }}</span>
            </el-button>
          </el-card>
        </main>
      </div>

      <PlaceDrawer
        @close="gallery.selectPlace(null)"
        @locate="
          () => {
            const place = gallery.selectedPlace;
            if (place) focus = { lat: place.lat, lng: place.lng, zoom: 15, token: Date.now() };
          }
        "
        @open-photo="(photo: Photo) => (gallery.selectedPhotoId = photo.id)"
      />

      <PhotoLightbox @close="gallery.selectedPhotoId = null" @locate="onPhotoLocated" @set-location="onSetPhotoLocation" />
    </div>

    <input
      ref="fileInput"
      type="file"
      accept="image/*,.heic,.heif"
      multiple
      hidden
      @change="onFilePicked"
    />

    <el-card v-if="batch" class="upload-float" shadow="always">
      <div class="upload-float-head">
        <el-icon class="upload-float-icon"><Loading /></el-icon>
        <span>{{ t('upload.uploading', { done: batch.done, total: batch.total }) }}</span>
      </div>
      <el-progress
        :percentage="batch.total ? Math.round((batch.done / batch.total) * 100) : 0"
        :stroke-width="8"
        striped
        striped-flow
      />
    </el-card>

    <div v-if="dragging" class="drop-mask">{{ t('upload.dropHere') }}</div>
  </el-config-provider>
</template>

<style scoped>
.boot {
  height: 100%;
}
</style>
