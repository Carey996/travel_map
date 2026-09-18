<script setup lang="ts">
/**
 * 基于 el-upload 的照片上传。
 *
 * Element Plus 负责拖拽区、文件列表与逐张状态，真正的上传编排
 * 交给 lib/upload.ts（解析 EXIF → 生成缩略图 → 算哈希 → 预签名直传 → 登记元数据）。
 */
import { ElMessage, type UploadRequestOptions } from 'element-plus';
import { UploadFilled } from '@element-plus/icons-vue';
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

import { uploadPhotoFile } from '@/lib/upload';

const props = withDefaults(
  defineProps<{
    /** 传了就归入该地点，不传则按 GPS 自动归属 */
    placeId?: string | null;
    variant?: 'button' | 'drop';
    /** 拖拽区的提示文案，留空用默认翻译 */
    hint?: string;
  }>(),
  { placeId: null, variant: 'button', hint: '' },
);

const emit = defineEmits<{ (event: 'uploaded'): void }>();

const { t } = useI18n();

const inflight = ref(0);
const succeeded = ref(0);
const failed = ref(0);

async function customUpload(options: UploadRequestOptions): Promise<unknown> {
  const outcome = await uploadPhotoFile(options.file as File, { placeId: props.placeId });
  if (outcome.error) throw new Error(outcome.error);
  if (outcome.skipped) ElMessage.info(t('upload.skipped', { n: 1 }));
  if (outcome.undecodable) ElMessage.warning(t('upload.undecodable', { n: 1 }));
  return outcome.photo;
}

function beforeUpload(): boolean {
  inflight.value += 1;
  return true;
}

function settle(): void {
  inflight.value = Math.max(0, inflight.value - 1);
  if (inflight.value === 0) {
    if (succeeded.value > 0) ElMessage.success(t('upload.done', { n: succeeded.value }));
    if (failed.value > 0) ElMessage.error(t('upload.partialFail', { n: failed.value, message: '' }));
    if (succeeded.value > 0) emit('uploaded');
    succeeded.value = 0;
    failed.value = 0;
  }
}

function onSuccess(): void {
  succeeded.value += 1;
  settle();
}

function onError(error: Error): void {
  failed.value += 1;
  ElMessage.error(error.message || t('upload.notImage'));
  settle();
}
</script>

<template>
  <el-upload
    v-if="props.variant === 'drop'"
    class="upload-drop"
    drag
    multiple
    :auto-upload="true"
    :show-file-list="true"
    :limit="200"
    accept="image/*,.heic,.heif"
    :http-request="customUpload"
    :before-upload="beforeUpload"
    :on-success="onSuccess"
    :on-error="onError"
  >
    <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
    <div class="el-upload__text">{{ props.hint || t('upload.dropHere') }}</div>
  </el-upload>

  <el-upload
    v-else
    multiple
    :auto-upload="true"
    :show-file-list="false"
    :limit="200"
    accept="image/*,.heic,.heif"
    :http-request="customUpload"
    :before-upload="beforeUpload"
    :on-success="onSuccess"
    :on-error="onError"
  >
    <el-button type="primary" :loading="inflight > 0">
      <el-icon><UploadFilled /></el-icon>
      <span>{{ t('upload.add') }}</span>
    </el-button>
  </el-upload>
</template>

<style scoped>
.upload-drop {
  width: 100%;
}

.upload-drop :deep(.el-upload-dragger) {
  padding: 26px 16px;
}

.upload-drop :deep(.el-upload-list) {
  margin-top: 8px;
}
</style>
