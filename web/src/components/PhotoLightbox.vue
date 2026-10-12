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
      <div class="lightbox-head">
        <span class="lightbox-title">{{ photo?.original_name }}</span>
        <span v-if="gallery.photos.length" class="lightbox-counter">
          {{ index + 1 }} / {{ gallery.photos.length }}
        </span>
      </div>
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

      <aside class="lightbox-meta">
        <el-descriptions :column="1" size="small">
          <el-descriptions-item :label="t('photo.takenAt')">
            {{ formatDateTime(photo.taken_at, locale) || '—' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('photo.camera')">{{ camera || '—' }}</el-descriptions-item>
          <el-descriptions-item :label="t('photo.exposure')">
            {{ [photo.f_number ? 'ƒ/' + photo.f_number : '', photo.exposure, photo.iso ? 'ISO ' + photo.iso : ''].filter(Boolean).join(' · ') || '—' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('photo.size')">{{ formatBytes(photo.bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('photo.location')">
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
      </aside>
    </div>
  </el-dialog>
</template>

<style scoped>
.lightbox-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.lightbox-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lightbox-counter {
  flex: none;
  padding: 1px 9px;
  border-radius: 999px;
  background: var(--el-fill-color);
  font-size: 12px;
  color: var(--el-text-color-regular);
  font-variant-numeric: tabular-nums;
}

/* 宽屏：左边看图、右边看信息；窄屏自动退回上下堆叠 */
.lightbox-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 16px;
  align-items: start;
}

.lightbox-stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-darker);
  border-radius: var(--tb-radius);
  padding: 12px;
  min-height: 420px;
}

.lightbox-image {
  max-height: 66vh;
  max-width: 100%;
}

.stage-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 3;
  background: color-mix(in srgb, var(--el-bg-color) 82%, transparent);
  backdrop-filter: blur(3px);
  box-shadow: var(--tb-shadow-1);
}

.stage-nav.is-prev { left: 12px; }
.stage-nav.is-next { right: 12px; }

.lightbox-meta {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-right: 2px;
}

.lightbox-meta :deep(.el-descriptions__label) {
  color: var(--el-text-color-secondary);
  font-weight: 400;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .lightbox-body {
    grid-template-columns: minmax(0, 1fr);
  }
  .lightbox-stage {
    min-height: 260px;
  }
}

.lightbox-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
