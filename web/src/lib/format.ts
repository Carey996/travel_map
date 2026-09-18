/** 展示层的格式化工具。 */

import type { Photo } from '../api/client';

export function formatDate(iso: string | null | undefined, lang: string): string {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(lang === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function formatDateTime(iso: string | null | undefined, lang: string): string {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleString(lang === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatDayLabel(iso: string | null | undefined, lang: string): string {
  const date = iso ? new Date(iso) : null;
  if (!date || Number.isNaN(date.getTime())) return lang === 'zh' ? '未知日期' : 'Unknown date';
  return date.toLocaleDateString(lang === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'short',
  });
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  return (bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1) + ' ' + units[index];
}

export function formatCoords(lat: number | null, lng: number | null): string {
  if (lat === null || lng === null) return '';
  return lat.toFixed(5) + ', ' + lng.toFixed(5);
}

/** 按「天」把照片分组，用于时间线视图 */
export function groupPhotosByDay(photos: Photo[]): Array<{ key: string; photos: Photo[] }> {
  const buckets = new Map<string, Photo[]>();
  for (const photo of photos) {
    const iso = photo.taken_at ?? photo.created_at;
    const key = iso ? iso.slice(0, 10) : 'unknown';
    const list = buckets.get(key);
    if (list) list.push(photo);
    else buckets.set(key, [photo]);
  }
  return Array.from(buckets.entries())
    .sort((a, b) => (a[0] < b[0] ? 1 : -1))
    .map(([key, list]) => ({ key, photos: list }));
}
