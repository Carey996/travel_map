/**
 * 浏览器端 EXIF 解析与缩略图生成。
 *
 * 放在前端做有两个好处：
 *  1. 后端不需要任何图像依赖，也不必中转文件字节流（配合预签名直传）；
 *  2. 原图不必先上传再解析，用户选完图立刻就能在地图上看到位置。
 *
 * 代价：浏览器解不了 HEIC（部分 iPhone 原图），这类文件仍可上传，
 * 但生成不出缩略图，界面会给出提示。
 */

import exifr from 'exifr';

export type PhotoExif = {
  lat: number | null;
  lng: number | null;
  altitude: number | null;
  takenAt: string | null;
  cameraMake: string;
  cameraModel: string;
  lens: string;
  iso: number | null;
  fNumber: number | null;
  exposure: string;
  focalLength: number | null;
  orientation: number | null;
};

const EMPTY: PhotoExif = {
  lat: null,
  lng: null,
  altitude: null,
  takenAt: null,
  cameraMake: '',
  cameraModel: '',
  lens: '',
  iso: null,
  fNumber: null,
  exposure: '',
  focalLength: null,
  orientation: null,
};

function toNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function toIso(value: unknown): string | null {
  if (value instanceof Date && !Number.isNaN(value.getTime())) return value.toISOString();
  if (typeof value === 'string') {
    const parsed = new Date(value);
    if (!Number.isNaN(parsed.getTime())) return parsed.toISOString();
  }
  return null;
}

function formatExposure(value: unknown): string {
  const seconds = toNumber(value);
  if (seconds === null || seconds <= 0) return '';
  if (seconds >= 1) return seconds + 's';
  return '1/' + Math.round(1 / seconds) + 's';
}

/** 把 Make / Model 里重复的品牌名去掉，例如 "Apple Apple iPhone 15 Pro" */
function cleanCamera(make: string, model: string): [string, string] {
  const cleanMake = make.trim();
  let cleanModel = model.trim();
  if (cleanMake && cleanModel.toLowerCase().startsWith(cleanMake.toLowerCase())) {
    cleanModel = cleanModel.slice(cleanMake.length).trim();
  }
  return [cleanMake, cleanModel];
}

export async function readExif(file: File): Promise<PhotoExif> {
  try {
    const tags = (await exifr.parse(file, true)) as Record<string, unknown> | undefined;
    if (!tags) return { ...EMPTY };

    let lat = toNumber(tags.latitude);
    let lng = toNumber(tags.longitude);
    if (lat === null || lng === null) {
      // 某些机型 GPS 在单独的段里，用 gps() 兜一次
      const gps = (await exifr.gps(file).catch(() => null)) as
        | { latitude?: number; longitude?: number }
        | null;
      lat = toNumber(gps?.latitude);
      lng = toNumber(gps?.longitude);
    }

    const make = typeof tags.Make === 'string' ? tags.Make : '';
    const model = typeof tags.Model === 'string' ? tags.Model : '';
    const [cameraMake, cameraModel] = cleanCamera(make, model);

    return {
      lat,
      lng,
      altitude: toNumber(tags.GPSAltitude) ?? toNumber(tags.altitude),
      takenAt:
        toIso(tags.DateTimeOriginal) ?? toIso(tags.CreateDate) ?? toIso(tags.ModifyDate),
      cameraMake,
      cameraModel,
      lens: typeof tags.LensModel === 'string' ? tags.LensModel : '',
      iso: toNumber(tags.ISO),
      fNumber: toNumber(tags.FNumber),
      exposure: formatExposure(tags.ExposureTime),
      focalLength: toNumber(tags.FocalLength),
      orientation: toNumber(tags.Orientation),
    };
  } catch {
    // EXIF 缺失或格式异常都不应阻断上传
    return { ...EMPTY };
  }
}

export type DerivedImages = {
  thumb: Blob | null;
  preview: Blob | null;
  width: number | null;
  height: number | null;
  decodable: boolean;
};

const THUMB_MAX = 480;
const PREVIEW_MAX = 1600;

let webpSupport: boolean | null = null;

function supportsWebp(): boolean {
  if (webpSupport !== null) return webpSupport;
  try {
    const canvas = document.createElement('canvas');
    canvas.width = 1;
    canvas.height = 1;
    webpSupport = canvas.toDataURL('image/webp').startsWith('data:image/webp');
  } catch {
    webpSupport = false;
  }
  return webpSupport;
}

async function encode(
  bitmap: ImageBitmap,
  maxDimension: number,
  quality: number,
): Promise<Blob | null> {
  const scale = Math.min(1, maxDimension / Math.max(bitmap.width, bitmap.height));
  const width = Math.max(1, Math.round(bitmap.width * scale));
  const height = Math.max(1, Math.round(bitmap.height * scale));

  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;
  ctx.drawImage(bitmap, 0, 0, width, height);

  const type = supportsWebp() ? 'image/webp' : 'image/jpeg';
  return new Promise<Blob | null>((resolve) => {
    canvas.toBlob((blob) => resolve(blob), type, quality);
  });
}

export async function deriveImages(file: File): Promise<DerivedImages> {
  let bitmap: ImageBitmap | null = null;
  try {
    // imageOrientation: 'from-image' 让浏览器按 EXIF 方向摆正，
    // 生成的缩略图因此不需要额外的旋转处理
    bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
  } catch {
    try {
      bitmap = await createImageBitmap(file);
    } catch {
      return { thumb: null, preview: null, width: null, height: null, decodable: false };
    }
  }

  try {
    const [thumb, preview] = await Promise.all([
      encode(bitmap, THUMB_MAX, 0.82),
      encode(bitmap, PREVIEW_MAX, 0.86),
    ]);
    return {
      thumb,
      preview,
      width: bitmap.width,
      height: bitmap.height,
      decodable: true,
    };
  } finally {
    bitmap.close();
  }
}

export async function sha256Hex(blob: Blob): Promise<string> {
  const buffer = await blob.arrayBuffer();
  const digest = await crypto.subtle.digest('SHA-256', buffer);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}
