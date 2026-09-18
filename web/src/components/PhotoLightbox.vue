<script setup lang="ts">
/** 照片查看器：大图、EXIF 信息、说明编辑、定位与删除。 */
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo } from '@/api/client';
import { formatBytes, formatCoords, formatDateTime } from '@/lib/format';
import { useGalleryStore } from '@/stores/gallery';

const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'locate', photo: Photo): void;
  (event: 'set-location', photo: Photo): void;
}>();

const { t, locale } = useI18n();
const gallery = useGalleryStore();

const photo = computed(() => gallery.selectedPhoto);
const visible = computed(() => photo.value !== null);
const caption = ref('');

const index = computed(() => gallery.photos.findIndex((item) => item.id === photo.value?.id));
const camera = computed(() =>
  [photo.value?.camera_make, photo.value?.camera_model].filter(Boolean).join(' '),
);

watch(
  () => photo.value?.id,
  () => {
    caption.value = photo.value?.caption ?? '';
  },
);

function step(delta: number): void {
  const next = gallery.photos[index.value + delta];
  if (next) gallery.selectedPhotoId = next.id;
}

function onKeydown(event: KeyboardEvent): void {
  if (!visible.value) return;
  if (event.key === 'ArrowRight') step(1);
  if (event.key === 'ArrowLeft') step(-1);
}

onMounted(() => window.addEventListener('keydown', onKeydown));
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown));

async function saveCaption(): Promise<void> {
  const current = photo.value;
  if (!current || caption.value === current.caption) return;
  try {
    await gallery.updatePhoto(current.id, { caption: caption.value });
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}

async function removePhoto(): Promise<void> {
  const current = photo.value;
  if (!current) return;
  try {
    await ElMessageBox.confirm(
      t('photo.confirmDelete'),
      t('common.confirm'),
      { type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel') },
    );
  } catch {
    return;
  }
  try {
    await gallery.deletePhoto(current.id);
    emit('close');
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    width="min(1080px, 94vw)"
    top="5vh"
    append-to-body
    destroy-on-close
    class="lightbox"
    @close="emit('close')"
  >
    <template #header>
      <span class="lightbox-title">{{ photo?.original_name }}</span>
    </template>

    <div v-if="photo" class="lightbox-body">
      <div class="lightbox-stage">
        <el-button
          v-if="index > 0"
          circle
          class="stage-nav is-prev"
          @click="step(-1)"
        >
          <el-icon><ArrowLeft /></el-icon>
        </el-button>

        <el-image :src="photo.preview_url" fit="contain" class="lightbox-image" />

        <el-button
          v-if="index >= 0 && index < gallery.photos.length - 1"
          circle
          class="stage-nav is-next"
          @click="step(1)"
        >
          <el-icon><ArrowRight /></el-icon>
        </el-button>
      </div>

      <div class="lightbox-meta">
        <el-descriptions :column="2" size="small" border>
          <el-descriptions-item :label="t('photo.takenAt')">
            {{ formatDateTime(photo.taken_at, locale) || '—' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('photo.camera')">{{ camera || '—' }}</el-descriptions-item>
          <el-descriptions-item :label="t('photo.exposure')">
            {{ [photo.f_number ? 'ƒ/' + photo.f_number : '', photo.exposure, photo.iso ? 'ISO ' + photo.iso : ''].filter(Boolean).join(' · ') || '—' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('photo.size')">{{ formatBytes(photo.bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('photo.location')" :span="2">
            {{ formatCoords(photo.lat, photo.lng) || '—' }}
          </el-descriptions-item>
        </el-descriptions>

        <el-input
          v-model="caption"
          class="lightbox-caption"
          :placeholder="t('photo.captionPlaceholder')"
          @blur="saveCaption"
        />

        <div class="lightbox-actions">
          <el-button v-if="photo.lat !== null && photo.lng !== null" @click="emit('locate', photo)">
            <el-icon><Aim /></el-icon>
            <span>{{ t('photo.locate') }}</span>
          </el-button>
          <el-button @click="emit('set-location', photo)">
            <el-icon><Location /></el-icon>
            <span>{{ t('photo.setLocation') }}</span>
          </el-button>
          <el-button tag="a" :href="photo.url" target="_blank" rel="noopener noreferrer">
            {{ t('photo.original') }}
          </el-button>
          <el-button type="danger" plain @click="removePhoto">{{ t('photo.delete') }}</el-button>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
.lightbox-title {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.lightbox-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.lightbox-stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-darker);
  border-radius: 10px;
  padding: 10px;
  min-height: 260px;
}

.lightbox-image {
  max-height: 58vh;
  max-width: 100%;
}

.stage-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 3;
}

.stage-nav.is-prev { left: 10px; }
.stage-nav.is-next { right: 10px; }

.lightbox-meta {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.lightbox-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
