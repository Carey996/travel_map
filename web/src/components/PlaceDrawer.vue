<script setup lang="ts">
/** 地点详情抽屉：改名、分类、评分、标签、Markdown 摘要、照片与旅行笔记。 */
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Note, Photo, PlaceKind } from '@/api/client';
import { formatCoords, formatDate } from '@/lib/format';
import { renderMarkdown } from '@/lib/markdown';
import { useGalleryStore } from '@/stores/gallery';
import PhotoUploader from './PhotoUploader.vue';

const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'locate'): void;
  (event: 'open-photo', photo: Photo): void;
}>();

const { t, locale } = useI18n();
const gallery = useGalleryStore();

const KINDS: PlaceKind[] = ['city', 'spot', 'region', 'other'];

const place = computed(() => gallery.selectedPlace);
const visible = computed(() => place.value !== null);

/** 封面：该地点第一张照片的预览图 */
const cover = computed(() => gallery.photosOfSelectedPlace[0] ?? null);

const name = ref('');
const summary = ref('');
const summaryPreview = ref(false);
const tagInput = ref('');
const saving = ref(false);

// 笔记编辑器的临时状态：id 为 null 表示正在新建
type NoteDraft = { id: string | null; title: string; body: string; day: string };
const noteDraft = ref<NoteDraft | null>(null);

const summaryHtml = computed(() => renderMarkdown(summary.value));

watch(
  () => place.value?.id,
  () => {
    name.value = place.value?.name ?? '';
    summary.value = place.value?.summary_md ?? '';
    summaryPreview.value = false;
    noteDraft.value = null;
    tagInput.value = '';
  },
  { immediate: true },
);

async function run(action: () => Promise<unknown>, successMessage?: string): Promise<void> {
  saving.value = true;
  try {
    await action();
    if (successMessage) ElMessage.success(successMessage);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}

function patch(patchData: Parameters<typeof gallery.updatePlace>[1]): void {
  if (!place.value) return;
  void run(() => gallery.updatePlace(place.value!.id, patchData));
}

function commitName(): void {
  if (!place.value || name.value === place.value.name) return;
  patch({ name: name.value });
}

function commitSummary(): void {
  if (!place.value || summary.value === place.value.summary_md) return;
  patch({ summary_md: summary.value });
}

function addTag(): void {
  const value = tagInput.value.trim().replace(/^#/, '');
  if (!place.value || !value) return;
  if (place.value.tags.includes(value)) {
    tagInput.value = '';
    return;
  }
  tagInput.value = '';
  patch({ tags: [...place.value.tags, value] });
}

function removeTag(tag: string): void {
  if (!place.value) return;
  patch({ tags: place.value.tags.filter((item) => item !== tag) });
}

function startNewNote(): void {
  noteDraft.value = {
    id: null,
    title: '',
    body: '',
    day: place.value?.date_from ?? new Date().toISOString().slice(0, 10),
  };
}

function editNote(note: Note): void {
  noteDraft.value = {
    id: note.id,
    title: note.title,
    body: note.body_md,
    day: note.happened_on ?? '',
  };
}

async function saveNote(): Promise<void> {
  const draft = noteDraft.value;
  if (!draft) return;
  const payload = { title: draft.title, body_md: draft.body, happened_on: draft.day || null };
  await run(async () => {
    if (draft.id) await gallery.updateNote(draft.id, payload);
    else await gallery.createNote(payload);
    noteDraft.value = null;
  }, t('common.save'));
}

async function confirmDeletePlace(withPhotos: boolean): Promise<void> {
  if (!place.value) return;
  const title = place.value.name || t('place.unnamed');
  try {
    await ElMessageBox.confirm(
      withPhotos ? t('place.confirmDeleteWithPhotos', { name: title }) : t('place.confirmDelete', { name: title }),
      t('common.confirm'),
      { type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel') },
    );
  } catch {
    return; // 用户取消
  }
  await run(async () => {
    await gallery.deletePlace(place.value!.id, withPhotos);
    emit('close');
  }, t('place.deleted'));
}

async function onMerge(targetId: string): Promise<void> {
  if (!place.value || !targetId) return;
  await run(() => gallery.mergePlace(place.value!.id, targetId), t('place.merged'));
}

async function onGeocode(): Promise<void> {
  if (!place.value) return;
  await run(() => gallery.fillMissingNames());
}
</script>

<template>
  <el-drawer
    :model-value="visible"
    direction="rtl"
    size="460px"
    :with-header="false"
    class="place-drawer"
    @close="emit('close')"
  >
    <div v-if="place" v-loading="saving" class="drawer-body">
      <div class="drawer-hero" :class="{ 'is-empty': !cover }">
        <img v-if="cover" class="hero-img" :src="cover.preview_url" alt="" />
        <div class="hero-scrim" />

        <div class="hero-bar">
          <el-tag class="hero-kind" effect="dark" round size="small">
            {{ t('place.kind.' + place.kind) }}
          </el-tag>
          <el-button circle class="drawer-close" @click="emit('close')">
            <el-icon><Close /></el-icon>
          </el-button>
        </div>

        <div class="hero-title">
          <h3>{{ place.name || t('place.unnamed') }}</h3>
          <p>
            <span>{{ formatDate(place.date_from ?? place.created_at, locale) }}</span>
            <span class="dot">·</span>
            <span>{{ t('place.photoCount', { n: gallery.photosOfSelectedPlace.length }) }}</span>
            <span v-if="place.rating" class="dot">·</span>
            <span v-if="place.rating">★ {{ place.rating }}</span>
          </p>
        </div>
      </div>

      <div class="drawer-content">
        <div class="title-row">
          <el-input
            v-model="name"
            size="large"
            :placeholder="t('place.namePlaceholder')"
            @blur="commitName"
            @keyup.enter="commitName"
          >
            <template #prefix>
              <el-icon><EditPen /></el-icon>
            </template>
          </el-input>
          <el-button v-if="!place.geocoded" type="primary" plain @click="onGeocode">
            <el-icon><MagicStick /></el-icon>
            <span>{{ t('place.geocodeNow') }}</span>
          </el-button>
        </div>

        <div class="meta-row">
          <el-select
            :model-value="place.kind"
            style="width: 108px"
            @change="(value: PlaceKind) => patch({ kind: value })"
          >
            <el-option v-for="kind in KINDS" :key="kind" :value="kind" :label="t('place.kind.' + kind)" />
          </el-select>

          <el-rate
            :model-value="place.rating"
            @change="(value: number) => patch({ rating: value === place!.rating ? 0 : value })"
          />

          <div class="meta-spacer" />

          <el-button text size="small" @click="emit('locate')">
            <el-icon><Aim /></el-icon>
            <span>{{ t('photo.locate') }}</span>
          </el-button>
        </div>

        <div class="coord-chip">
          <el-icon><Place /></el-icon>
          <span>{{ formatCoords(place.lat, place.lng) }}</span>
          <template v-if="place.address">
            <span class="dot">·</span>
            <span class="addr">{{ place.address }}</span>
          </template>
        </div>

        <div class="section-head">
          <el-icon><Document /></el-icon>
          <h4>{{ t('place.summary') }}</h4>
          <el-button text size="small" @click="summaryPreview = !summaryPreview">
            {{ summaryPreview ? t('common.edit') : t('common.preview') }}
          </el-button>
        </div>
        <div
          v-if="summaryPreview"
          class="markdown summary-box"
          v-html="summaryHtml || '<p class=&quot;muted&quot;>' + t('place.summaryEmpty') + '</p>'"
        />
        <el-input
          v-else
          v-model="summary"
          type="textarea"
          :rows="4"
          resize="none"
          :placeholder="t('place.summaryPlaceholder')"
          @blur="commitSummary"
        />

        <div class="tag-editor">
          <el-tag
            v-for="tag in place.tags"
            :key="tag"
            closable
            effect="plain"
            round
            @close="removeTag(tag)"
          >
            #{{ tag }}
          </el-tag>
          <el-input
            v-model="tagInput"
            size="small"
            :placeholder="t('place.addTag')"
            @keyup.enter="addTag"
          >
            <template #prefix>
              <el-icon><PriceTag /></el-icon>
            </template>
          </el-input>
        </div>

        <div class="section-head">
          <el-icon><Picture /></el-icon>
          <h4>
            {{ t('place.photos') }}
            <span class="count">{{ gallery.photosOfSelectedPlace.length }}</span>
          </h4>
          <PhotoUploader :place-id="place.id" @uploaded="gallery.refresh()" />
        </div>
        <el-empty
          v-if="gallery.photosOfSelectedPlace.length === 0"
          :description="t('place.noPhotos')"
          :image-size="64"
        />
        <div v-else class="thumb-grid">
          <el-image
            v-for="photo in gallery.photosOfSelectedPlace"
            :key="photo.id"
            :src="photo.thumb_url"
            :alt="photo.caption || photo.original_name"
            fit="cover"
            lazy
            @click="emit('open-photo', photo)"
          />
        </div>

        <div class="section-head">
          <el-icon><Notebook /></el-icon>
          <h4>
            {{ t('place.notes') }}
            <span class="count">{{ gallery.notes.length }}</span>
          </h4>
          <el-button text size="small" @click="startNewNote">
            <el-icon><Plus /></el-icon>
            <span>{{ t('note.new') }}</span>
          </el-button>
        </div>

        <div v-if="noteDraft" class="note-editor">
          <el-input v-model="noteDraft.title" :placeholder="t('note.titlePlaceholder')" class="mb" />
          <el-date-picker
            v-model="noteDraft.day"
            type="date"
            value-format="YYYY-MM-DD"
            :placeholder="t('note.happenedOn')"
            class="mb"
            style="width: 100%"
          />
          <el-input
            v-model="noteDraft.body"
            type="textarea"
            :rows="4"
            resize="none"
            :placeholder="t('note.bodyPlaceholder')"
            class="mb"
          />
          <div class="note-actions">
            <el-button type="primary" @click="saveNote">{{ t('common.save') }}</el-button>
            <el-button @click="noteDraft = null">{{ t('common.cancel') }}</el-button>
          </div>
        </div>

        <el-skeleton v-if="gallery.notesLoading" :rows="3" animated class="mb" />
        <el-empty v-else-if="gallery.notes.length === 0" :description="t('note.empty')" :image-size="56" />

        <div v-for="note in gallery.notes" :key="note.id" class="note-item">
          <div class="note-head">
            <strong>{{ note.title || t('note.untitled') }}</strong>
            <span class="muted">{{ formatDate(note.happened_on, locale) }}</span>
          </div>
          <div class="markdown" v-html="renderMarkdown(note.body_md)" />
          <div class="note-actions">
            <el-button text size="small" @click="editNote(note)">
              <el-icon><EditPen /></el-icon>
              <span>{{ t('common.edit') }}</span>
            </el-button>
            <el-button
              text
              size="small"
              type="danger"
              @click="run(() => gallery.deleteNote(note.id))"
            >
              <el-icon><Delete /></el-icon>
              <span>{{ t('common.delete') }}</span>
            </el-button>
          </div>
        </div>

        <el-collapse class="danger">
          <el-collapse-item name="danger">
            <template #title>
              <el-icon class="danger-icon"><WarningFilled /></el-icon>
              <span>{{ t('place.dangerZone') }}</span>
            </template>
            <div class="danger-actions">
              <el-select
                :placeholder="t('place.mergeInto')"
                style="width: 100%"
                @change="onMerge"
              >
                <el-option
                  v-for="other in gallery.places.filter((item) => item.id !== place!.id)"
                  :key="other.id"
                  :value="other.id"
                  :label="other.name || t('place.unnamed')"
                />
              </el-select>
              <el-button type="danger" plain @click="confirmDeletePlace(false)">
                {{ t('place.deleteKeepPhotos') }}
              </el-button>
              <el-button type="danger" @click="confirmDeletePlace(true)">
                {{ t('place.deleteWithPhotos') }}
              </el-button>
            </div>
          </el-collapse-item>
        </el-collapse>
      </div>
    </div>
  </el-drawer>
</template>

<style scoped>
.place-drawer :deep(.el-drawer__body) {
  padding: 0;
  overflow: hidden;
}

.drawer-body {
  display: flex;
  flex-direction: column;
  height: 100%;
}

/* ---------------- 封面 ---------------- */

.drawer-hero {
  position: relative;
  height: 190px;
  flex: none;
  background: linear-gradient(135deg, var(--el-color-primary-light-3), var(--el-color-primary));
  overflow: hidden;
}

.drawer-hero.is-empty {
  height: 108px;
}

.hero-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.hero-scrim {
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.28) 0%, rgba(0, 0, 0, 0) 38%, rgba(0, 0, 0, 0.62) 100%);
}

.hero-bar {
  position: absolute;
  top: 10px;
  left: 12px;
  right: 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hero-kind {
  border: 0;
  background: rgba(0, 0, 0, 0.42) !important;
  backdrop-filter: blur(4px);
}

.drawer-close {
  background: rgba(0, 0, 0, 0.38);
  border-color: transparent;
  color: #fff;
}

.drawer-close:hover {
  background: rgba(0, 0, 0, 0.58);
  color: #fff;
}

.hero-title {
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: 12px;
  color: #fff;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.5);
}

.hero-title h3 {
  margin: 0 0 2px;
  font-size: 19px;
  letter-spacing: -0.01em;
}

.hero-title p {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: 12.5px;
  opacity: 0.92;
}

.hero-title .dot,
.coord-chip .dot {
  opacity: 0.55;
}

/* ---------------- 正文 ---------------- */

.drawer-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px 18px 34px;
}

.title-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.meta-spacer {
  flex: 1;
}

.coord-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  padding: 7px 10px;
  border-radius: var(--tb-radius-sm);
  background: var(--el-fill-color-light);
  color: var(--el-text-color-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.coord-chip .addr {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-box {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--tb-radius-sm);
  padding: 10px 12px;
  background: var(--el-fill-color-lighter);
}

.note-editor {
  border: 1px solid var(--el-color-primary-light-7);
  border-radius: var(--tb-radius);
  padding: 12px;
  background: var(--el-color-primary-light-9);
  margin-bottom: 12px;
}

.note-actions {
  display: flex;
  gap: 4px;
  align-items: center;
  margin-top: 6px;
}

.danger {
  margin-top: 26px;
  border-top: 1px solid var(--el-border-color-lighter);
  border-bottom: 0;
}

.danger :deep(.el-collapse-item__header),
.danger :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
  background: transparent;
}

.danger-icon {
  margin-right: 6px;
  color: var(--el-color-danger);
}

.danger-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mb {
  margin-bottom: 8px;
}
</style>
