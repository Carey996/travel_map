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
    size="440px"
    :with-header="false"
    class="place-drawer"
    @close="emit('close')"
  >
    <div v-if="place" v-loading="saving" class="drawer-body">
      <div
        class="drawer-cover"
        :style="
          gallery.photosOfSelectedPlace.length
            ? { backgroundImage: 'url(' + gallery.photosOfSelectedPlace[0].preview_url + ')' }
            : undefined
        "
      >
        <el-button circle class="drawer-close" @click="emit('close')">
          <el-icon><Close /></el-icon>
        </el-button>
      </div>

      <div class="drawer-content">
        <div class="title-row">
          <el-input
            v-model="name"
            size="large"
            :placeholder="t('place.namePlaceholder')"
            @blur="commitName"
            @keyup.enter="commitName"
          />
          <el-button v-if="!place.geocoded" type="primary" plain @click="onGeocode">
            {{ t('place.geocodeNow') }}
          </el-button>
        </div>

        <div class="meta-row">
          <el-select
            :model-value="place.kind"
            style="width: 106px"
            @change="(value: PlaceKind) => patch({ kind: value })"
          >
            <el-option v-for="kind in KINDS" :key="kind" :value="kind" :label="t('place.kind.' + kind)" />
          </el-select>

          <el-rate
            :model-value="place.rating"
            :max="5"
            @change="(value: number) => patch({ rating: value === place!.rating ? 0 : value })"
          />

          <el-button text @click="emit('locate')">
            <el-icon><Aim /></el-icon>
          </el-button>
        </div>

        <p class="coords">
          {{ formatCoords(place.lat, place.lng) }}
          <template v-if="place.address"> · {{ place.address }}</template>
        </p>

        <div class="section-head">
          <h4>{{ t('place.summary') }}</h4>
          <el-button text size="small" @click="summaryPreview = !summaryPreview">
            {{ summaryPreview ? t('common.edit') : t('common.preview') }}
          </el-button>
        </div>
        <div
          v-if="summaryPreview"
          class="markdown"
          v-html="summaryHtml || '<p class=&quot;muted&quot;>' + t('place.summaryEmpty') + '</p>'"
        />
        <el-input
          v-else
          v-model="summary"
          type="textarea"
          :rows="5"
          :placeholder="t('place.summaryPlaceholder')"
          @blur="commitSummary"
        />

        <div class="tag-editor">
          <el-tag
            v-for="tag in place.tags"
            :key="tag"
            closable
            effect="plain"
            @close="removeTag(tag)"
          >
            #{{ tag }}
          </el-tag>
          <el-input
            v-model="tagInput"
            size="small"
            :placeholder="t('place.addTag')"
            @keyup.enter="addTag"
          />
        </div>

        <div class="section-head">
          <h4>{{ t('place.photos') }} · {{ gallery.photosOfSelectedPlace.length }}</h4>
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
          <h4>{{ t('place.notes') }} · {{ gallery.notes.length }}</h4>
          <el-button text size="small" @click="startNewNote">{{ t('note.new') }}</el-button>
        </div>

        <el-card v-if="noteDraft" shadow="never" class="note-editor">
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
            :placeholder="t('note.bodyPlaceholder')"
            class="mb"
          />
          <div class="note-actions">
            <el-button type="primary" @click="saveNote">{{ t('common.save') }}</el-button>
            <el-button @click="noteDraft = null">{{ t('common.cancel') }}</el-button>
          </div>
        </el-card>

        <el-skeleton v-if="gallery.notesLoading" :rows="3" animated class="mb" />
        <el-empty v-else-if="gallery.notes.length === 0" :description="t('note.empty')" :image-size="56" />

        <div v-for="note in gallery.notes" :key="note.id" class="note-item">
          <div class="note-head">
            <strong>{{ note.title || t('note.untitled') }}</strong>
            <span class="muted">{{ formatDate(note.happened_on, locale) }}</span>
          </div>
          <div class="markdown" v-html="renderMarkdown(note.body_md)" />
          <div class="note-actions">
            <el-button text size="small" @click="editNote(note)">{{ t('common.edit') }}</el-button>
            <el-button
              text
              size="small"
              type="danger"
              @click="run(() => gallery.deleteNote(note.id))"
            >
              {{ t('common.delete') }}
            </el-button>
          </div>
        </div>

        <el-collapse class="danger">
          <el-collapse-item :title="t('place.dangerZone')" name="danger">
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
.drawer-body {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.drawer-cover {
  position: relative;
  height: 150px;
  flex: none;
  background: var(--el-fill-color) center / cover no-repeat;
}

.drawer-close {
  position: absolute;
  top: 10px;
  right: 10px;
}

.drawer-content {
  flex: 1;
  overflow-y: auto;
  padding: 14px 18px 30px;
}

.title-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 10px;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

.note-editor {
  margin-bottom: 12px;
}

.note-actions {
  display: flex;
  gap: 4px;
  align-items: center;
  margin-top: 4px;
}

.danger {
  margin-top: 22px;
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
