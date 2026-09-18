/**
 * 后端 API 客户端。
 *
 * 两个关键约定集中在这里，避免散落各处：
 *  1. 变更类请求自动带上 X-Requested-With 头 —— 后端用它抵御 CSRF；
 *  2. credentials: 'include' —— 会话走 HttpOnly Cookie。
 */

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly detail?: unknown;

  constructor(status: number, code: string, message: string, detail?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.detail = detail;
  }
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');
const UNSAFE_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

export type HealthResponse = {
  status: string;
  app: string;
  version: string;
  environment: string;
};

export type ReadyResponse = {
  status: string;
  checks: Record<string, string>;
};

export type User = {
  id: string;
  email: string;
  display_name: string;
  created_at: string;
};

export type PlaceKind = 'city' | 'spot' | 'region' | 'other';

export type Place = {
  id: string;
  name: string;
  kind: PlaceKind;
  lat: number;
  lng: number;
  address: string;
  geocoded: boolean;
  summary_md: string;
  tags: string[];
  rating: number;
  date_from: string | null;
  date_to: string | null;
  trip_id: string | null;
  created_at: string;
  updated_at: string;
  photo_count?: number;
  note_count?: number;
};

export type Photo = {
  id: string;
  place_id: string | null;
  caption: string;
  original_name: string;
  mime: string;
  bytes: number;
  width: number | null;
  height: number | null;
  lat: number | null;
  lng: number | null;
  altitude: number | null;
  taken_at: string | null;
  camera_make: string;
  camera_model: string;
  lens: string;
  iso: number | null;
  f_number: number | null;
  exposure: string;
  focal_length: number | null;
  created_at: string;
  url: string;
  preview_url: string;
  thumb_url: string;
};

export type Note = {
  id: string;
  place_id: string;
  title: string;
  body_md: string;
  happened_on: string | null;
  created_at: string;
  updated_at: string;
};

export type SignedUpload = {
  key: string;
  url: string;
  headers: Record<string, string>;
};

export type UploadUrlResponse = {
  duplicate: Photo | null;
  original: SignedUpload;
  preview: SignedUpload | null;
  thumb: SignedUpload | null;
};

export type GeoReverse = {
  lat: number;
  lng: number;
  name: string;
  short_name: string;
  display_name: string;
  kind: PlaceKind;
  country: string;
  address: Record<string, unknown>;
};

export type GeoSearchItem = {
  lat: number;
  lng: number;
  name: string;
  display_name: string;
  kind: PlaceKind;
  category: string;
  type: string;
};

export type PhotoCreatePayload = {
  content_hash: string;
  storage_key: string;
  preview_key?: string | null;
  thumb_key?: string | null;
  original_name?: string;
  mime?: string;
  bytes?: number;
  width?: number | null;
  height?: number | null;
  lat?: number | null;
  lng?: number | null;
  altitude?: number | null;
  taken_at?: string | null;
  camera_make?: string;
  camera_model?: string;
  lens?: string;
  iso?: number | null;
  f_number?: number | null;
  exposure?: string;
  focal_length?: number | null;
  orientation?: number | null;
  place_id?: string | null;
};

export type PhotoQuery = {
  place_id?: string;
  unplaced?: boolean;
  located?: boolean;
  bbox?: string;
  date_from?: string;
  date_to?: string;
  q?: string;
  limit?: number;
  offset?: number;
};

function buildQuery(params: Record<string, unknown> | undefined): string {
  if (!params) return '';
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue;
    search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? '?' + qs : '';
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase();
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');

  if (UNSAFE_METHODS.has(method)) {
    headers.set('X-Requested-With', 'XMLHttpRequest');
  }
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(API_BASE + path, {
    ...init,
    method,
    headers,
    credentials: 'include',
  });

  if (response.status === 204) return undefined as T;

  const raw = await response.text();
  let payload: unknown = null;
  if (raw) {
    try {
      payload = JSON.parse(raw) as unknown;
    } catch {
      payload = raw;
    }
  }

  if (!response.ok) {
    const err = (payload ?? {}) as { code?: string; message?: string; detail?: unknown };
    throw new ApiError(
      response.status,
      err.code ?? 'http_error',
      err.message ?? response.statusText,
      err.detail,
    );
  }

  return payload as T;
}

const json = (body: unknown): RequestInit => ({ body: JSON.stringify(body) });

export const api = {
  health: () => apiFetch<HealthResponse>('/healthz'),
  ready: () => apiFetch<ReadyResponse>('/healthz/ready'),

  // ---- 鉴权 ----
  register: (email: string, password: string, displayName: string) =>
    apiFetch<User>('/api/v1/auth/register', { method: 'POST', ...json({ email, password, display_name: displayName }) }),
  login: (email: string, password: string) =>
    apiFetch<User>('/api/v1/auth/login', { method: 'POST', ...json({ email, password }) }),
  logout: () => apiFetch<void>('/api/v1/auth/logout', { method: 'POST', ...json({}) }),
  me: () => apiFetch<User>('/api/v1/auth/me'),

  // ---- 地点 ----
  listPlaces: (params?: { bbox?: string; q?: string }) =>
    apiFetch<Place[]>('/api/v1/places' + buildQuery(params)),
  getPlace: (id: string) => apiFetch<Place>('/api/v1/places/' + id),
  createPlace: (payload: { name?: string; kind?: PlaceKind; lat: number; lng: number; address?: string }) =>
    apiFetch<Place>('/api/v1/places', { method: 'POST', ...json(payload) }),
  updatePlace: (id: string, payload: Partial<Place>) =>
    apiFetch<Place>('/api/v1/places/' + id, { method: 'PATCH', ...json(payload) }),
  deletePlace: (id: string, deletePhotos = false) =>
    apiFetch<void>('/api/v1/places/' + id + buildQuery({ delete_photos: deletePhotos }), { method: 'DELETE' }),
  assignPhotos: (placeId: string, photoIds: string[]) =>
    apiFetch<Place>('/api/v1/places/' + placeId + '/assign', { method: 'POST', ...json({ photo_ids: photoIds }) }),
  mergePlace: (placeId: string, targetId: string) =>
    apiFetch<Place>('/api/v1/places/' + placeId + '/merge-into', { method: 'POST', ...json({ target_id: targetId }) }),

  // ---- 照片 ----
  listPhotos: (params?: PhotoQuery) => apiFetch<Photo[]>('/api/v1/photos' + buildQuery(params)),
  createUploadUrl: (payload: {
    filename: string;
    content_type: string;
    content_hash: string;
    byte_size: number;
    preview?: { content_type: string; suffix: string } | null;
    thumb?: { content_type: string; suffix: string } | null;
  }) => apiFetch<UploadUrlResponse>('/api/v1/photos/upload-url', { method: 'POST', ...json(payload) }),
  createPhoto: (payload: PhotoCreatePayload) =>
    apiFetch<Photo>('/api/v1/photos', { method: 'POST', ...json(payload) }),
  updatePhoto: (id: string, payload: { caption?: string; place_id?: string; clear_place?: boolean; lat?: number; lng?: number }) =>
    apiFetch<Photo>('/api/v1/photos/' + id, { method: 'PATCH', ...json(payload) }),
  deletePhoto: (id: string) => apiFetch<void>('/api/v1/photos/' + id, { method: 'DELETE' }),

  // ---- 笔记 ----
  listNotes: (placeId?: string) => apiFetch<Note[]>('/api/v1/notes' + buildQuery({ place_id: placeId })),
  createNote: (payload: { place_id: string; title?: string; body_md?: string; happened_on?: string | null }) =>
    apiFetch<Note>('/api/v1/notes', { method: 'POST', ...json(payload) }),
  updateNote: (id: string, payload: { title?: string; body_md?: string; happened_on?: string | null }) =>
    apiFetch<Note>('/api/v1/notes/' + id, { method: 'PATCH', ...json(payload) }),
  deleteNote: (id: string) => apiFetch<void>('/api/v1/notes/' + id, { method: 'DELETE' }),

  // ---- 地名 ----
  reverseGeocode: (lat: number, lng: number) =>
    apiFetch<GeoReverse>('/api/v1/geo/reverse' + buildQuery({ lat, lng })),
  searchPlaces: (q: string, limit = 6) =>
    apiFetch<GeoSearchItem[]>('/api/v1/geo/search' + buildQuery({ q, limit })),
};
