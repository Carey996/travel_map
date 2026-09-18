/**
 * 一张照片的完整上传编排。
 *
 * 顺序刻意如此：
 *   解析 EXIF -> 生成缩略图 -> 算 sha256 -> 取预签名 URL
 *   -> 若已存在同哈希则直接短路返回（省掉全部网络传输）
 *   -> 直传对象存储 -> 登记元数据
 */

import { api, type Photo } from '../api/client';
import { deriveImages, readExif, sha256Hex } from './exif';

export type UploadOutcome = {
  file: File;
  photo: Photo | null;
  error: string | null;
  skipped: boolean;
  undecodable: boolean;
};

async function putToStorage(url: string, headers: Record<string, string>, blob: Blob): Promise<void> {
  const response = await fetch(url, { method: 'PUT', body: blob, headers });
  if (!response.ok) {
    throw new Error('对象存储直传失败：HTTP ' + response.status);
  }
}

export async function uploadPhotoFile(
  file: File,
  options: { placeId?: string | null; onStage?: (stage: string) => void } = {},
): Promise<UploadOutcome> {
  const stage = options.onStage ?? (() => {});
  try {
    stage('exif');
    const exif = await readExif(file);

    stage('thumbnails');
    const derived = await deriveImages(file);

    stage('hash');
    const contentHash = await sha256Hex(file);

    stage('sign');
    const signed = await api.createUploadUrl({
      filename: file.name,
      content_type: file.type || 'image/jpeg',
      content_hash: contentHash,
      byte_size: file.size,
      preview: derived.preview ? { content_type: derived.preview.type, suffix: '.jpg' } : null,
      thumb: derived.thumb ? { content_type: derived.thumb.type, suffix: '.jpg' } : null,
    });

    if (signed.duplicate) {
      return { file, photo: signed.duplicate, error: null, skipped: true, undecodable: false };
    }

    stage('upload');
    await putToStorage(signed.original.url, signed.original.headers, file);
    if (signed.preview && derived.preview) {
      await putToStorage(signed.preview.url, signed.preview.headers, derived.preview);
    }
    if (signed.thumb && derived.thumb) {
      await putToStorage(signed.thumb.url, signed.thumb.headers, derived.thumb);
    }

    stage('register');
    const photo = await api.createPhoto({
      content_hash: contentHash,
      storage_key: signed.original.key,
      preview_key: signed.preview?.key ?? null,
      thumb_key: signed.thumb?.key ?? null,
      original_name: file.name,
      mime: file.type || 'image/jpeg',
      bytes: file.size,
      width: derived.width,
      height: derived.height,
      lat: exif.lat,
      lng: exif.lng,
      altitude: exif.altitude,
      taken_at: exif.takenAt ?? new Date(file.lastModified).toISOString(),
      camera_make: exif.cameraMake,
      camera_model: exif.cameraModel,
      lens: exif.lens,
      iso: exif.iso,
      f_number: exif.fNumber,
      exposure: exif.exposure,
      focal_length: exif.focalLength,
      orientation: exif.orientation,
      place_id: options.placeId ?? null,
    });

    return { file, photo, error: null, skipped: false, undecodable: !derived.decodable };
  } catch (error) {
    return {
      file,
      photo: null,
      error: error instanceof Error ? error.message : String(error),
      skipped: false,
      undecodable: false,
    };
  }
}

/** 带并发上限的批量上传；默认 3 路，避免同时解码大量图片把浏览器卡死 */
export async function uploadMany(
  files: File[],
  options: {
    placeId?: string | null;
    concurrency?: number;
    onProgress?: (done: number, total: number, current: string) => void;
    onResult?: (outcome: UploadOutcome) => void;
  } = {},
): Promise<UploadOutcome[]> {
  const concurrency = Math.max(1, options.concurrency ?? 3);
  const queue = [...files];
  const results: UploadOutcome[] = [];
  let done = 0;

  async function worker(): Promise<void> {
    for (;;) {
      const file = queue.shift();
      if (!file) return;
      options.onProgress?.(done, files.length, file.name);
      const outcome = await uploadPhotoFile(file, { placeId: options.placeId });
      results.push(outcome);
      done += 1;
      options.onProgress?.(done, files.length, file.name);
      options.onResult?.(outcome);
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, files.length) }, worker));
  return results;
}
