import { computed, ref } from 'vue';
import { defineStore } from 'pinia';

import { api, type Note, type Photo, type Place, type PlaceKind, type Trip } from '@/api/client';

/** 单次刷新最多补多少条地理编码，避免一次打开就打出几十个请求 */
const GEOCODE_BATCH = 40;

/**
 * 从反向地理编码的 address 里取行政区划。
 * Photon 给 state/city/district，Nominatim 还给 province/county/town，都认一下。
 */
function pickAdmin(address: Record<string, unknown> | undefined) {
  const pick = (...keys: string[]): string => {
    for (const key of keys) {
      const value = address?.[key];
      if (typeof value === 'string' && value.trim()) return value.trim();
    }
    return '';
  };
  return {
    country: pick('country'),
    province: pick('state', 'province'),
    city: pick('city', 'town', 'municipality', 'village'),
    // 区/县一级：Photon 有 county 时用它；OSM 对国内很多地方只到街道，才退到 district
    district: pick('county', 'city_district', 'district', 'suburb'),
  };
}

/**
 * 相册主状态：地点、照片、笔记，以及当前选中项。
 *
 * 数据量不大（个人旅行博客），所以一次性全量拉取后在内存里派生各种视图，
 * 比给每个视图单独发请求更简单也更快。每次写操作后调用 refresh() 重新同步。
 */
export const useGalleryStore = defineStore('gallery', () => {
  const places = ref<Place[]>([]);
  const photos = ref<Photo[]>([]);
  const trips = ref<Trip[]>([]);
  const unplacedPhotos = ref<Photo[]>([]);
  const notes = ref<Note[]>([]);

  const loading = ref(false);
  const notesLoading = ref(false);
  const submitting = ref(false);

  const selectedPlaceId = ref<string | null>(null);
  const selectedPhotoId = ref<string | null>(null);

  // 已经尝试过自动补名的地点，避免反复请求
  const geocodeAttempted = new Set<string>();

  const selectedPlace = computed(
    () => places.value.find((place) => place.id === selectedPlaceId.value) ?? null,
  );
  const selectedPhoto = computed(
    () => photos.value.find((photo) => photo.id === selectedPhotoId.value) ?? null,
  );
  const locatedPhotos = computed(() =>
    photos.value.filter((photo) => photo.lat !== null && photo.lng !== null),
  );
  const photosOfSelectedPlace = computed(() =>
    photos.value.filter((photo) => photo.place_id === selectedPlaceId.value),
  );

  async function refresh(): Promise<void> {
    loading.value = true;
    try {
      const [nextPlaces, nextPhotos, nextUnplaced, nextTrips] = await Promise.all([
        api.listPlaces(),
        api.listPhotos({ limit: 2000 }),
        api.listPhotos({ unplaced: true, limit: 2000 }),
        api.listTrips(),
      ]);
      places.value = nextPlaces;
      photos.value = nextPhotos;
      unplacedPhotos.value = nextUnplaced;
      trips.value = nextTrips;
    } finally {
      loading.value = false;
    }
  }

  async function loadNotes(placeId: string | null): Promise<void> {
    if (!placeId) {
      notes.value = [];
      return;
    }
    notesLoading.value = true;
    try {
      notes.value = await api.listNotes(placeId);
    } finally {
      notesLoading.value = false;
    }
  }

  async function selectPlace(placeId: string | null): Promise<void> {
    selectedPlaceId.value = placeId;
    await loadNotes(placeId);
  }

  /**
   * 给地点补反向地理编码结果：缺名字的补名字，缺行政区划的补行政区划。
   * 失败静默——用户可以手动改名，地图也会退回按距离分组。
   */
  async function fillMissingNames(): Promise<boolean> {
    const pending = places.value
      .filter((place) => (!place.geocoded || !place.province) && !geocodeAttempted.has(place.id))
      .slice(0, GEOCODE_BATCH);
    if (pending.length === 0) return false;

    let changed = false;
    for (const place of pending) {
      geocodeAttempted.add(place.id);
      try {
        const info = await api.reverseGeocode(place.lat, place.lng);
        const admin = pickAdmin(info.address);
        const payload: Record<string, unknown> = {
          address: info.display_name,
          country: admin.country,
          province: admin.province,
          city: admin.city,
          district: admin.district,
        };
        // 只在还没名字时改名字，避免覆盖用户自己起的名字
        if (!place.geocoded) {
          payload.name = info.name;
          payload.kind = info.kind;
        }
        await api.updatePlace(place.id, payload);
        changed = true;
      } catch {
        // 地名服务不可用时保持原样，等待下次刷新重试
        geocodeAttempted.delete(place.id);
      }
    }
    return changed;
  }

  // ---- 地点 ----

  async function createPlace(payload: {
    name?: string;
    kind?: PlaceKind;
    lat: number;
    lng: number;
    address?: string;
  }): Promise<Place> {
    const place = await api.createPlace(payload);
    await refresh();
    await selectPlace(place.id);
    return place;
  }

  async function updatePlace(id: string, patch: Partial<Place>): Promise<void> {
    await api.updatePlace(id, patch);
    await refresh();
  }

  async function deletePlace(id: string, deletePhotos: boolean): Promise<void> {
    await api.deletePlace(id, deletePhotos);
    if (selectedPlaceId.value === id) await selectPlace(null);
    await refresh();
  }

  async function mergePlace(id: string, targetId: string): Promise<void> {
    await api.mergePlace(id, targetId);
    await selectPlace(targetId);
    await refresh();
  }

  /**
   * 把「当前这一张」照片移入指定地点。
   * 灯箱「设置位置」走这里；侧栏「把未定位照片放到地图上」是批量语义，走 assignUnplacedTo。
   */
  async function assignPhotoTo(placeId: string, photoId: string): Promise<void> {
    await api.assignPhotos(placeId, [photoId]);
    await refresh();
  }

  /** 把「全部未定位照片」移入指定地点——只给侧栏的批量入口用 */
  async function assignUnplacedTo(placeId: string): Promise<void> {
    const ids = unplacedPhotos.value.map((photo) => photo.id);
    if (ids.length === 0) return;
    await api.assignPhotos(placeId, ids);
    await refresh();
  }

  // ---- 行程 ----

  async function createTrip(payload: { title?: string; date_from?: string | null; date_to?: string | null }): Promise<Trip> {
    const trip = await api.createTrip(payload);
    await refresh();
    return trip;
  }

  async function renameTrip(id: string, title: string): Promise<void> {
    await api.updateTrip(id, { title });
    await refresh();
  }

  async function deleteTrip(id: string): Promise<void> {
    await api.deleteTrip(id);
    await refresh();
  }

  /** 把地点放进某个行程；tripId 传 null 表示移出 */
  async function setPlaceTrip(placeId: string, tripId: string | null): Promise<void> {
    await api.updatePlace(placeId, tripId ? { trip_id: tripId } : { clear_trip: true });
    await refresh();
  }

  /** 某个行程下的地点 */
  function placesOfTrip(tripId: string): Place[] {
    return places.value.filter((place) => place.trip_id === tripId);
  }

  // ---- 照片 ----

  async function updatePhoto(
    id: string,
    patch: { caption?: string; place_id?: string; clear_place?: boolean; lat?: number; lng?: number },
  ): Promise<void> {
    await api.updatePhoto(id, patch);
    await refresh();
  }

  async function deletePhoto(id: string): Promise<void> {
    await api.deletePhoto(id);
    if (selectedPhotoId.value === id) selectedPhotoId.value = null;
    await refresh();
  }

  // ---- 笔记 ----

  async function createNote(payload: {
    title: string;
    body_md: string;
    happened_on: string | null;
  }): Promise<void> {
    if (!selectedPlaceId.value) return;
    await api.createNote({ place_id: selectedPlaceId.value, ...payload });
    await Promise.all([loadNotes(selectedPlaceId.value), refreshPlacesOnly()]);
  }

  async function updateNote(
    id: string,
    patch: { title?: string; body_md?: string; happened_on?: string | null },
  ): Promise<void> {
    await api.updateNote(id, patch);
    await loadNotes(selectedPlaceId.value);
  }

  async function deleteNote(id: string): Promise<void> {
    await api.deleteNote(id);
    await Promise.all([loadNotes(selectedPlaceId.value), refreshPlacesOnly()]);
  }

  /** 只刷新地点列表（笔记数量会影响它），避免每次都重拉全部照片 */
  async function refreshPlacesOnly(): Promise<void> {
    places.value = await api.listPlaces();
  }

  function reset(): void {
    places.value = [];
    photos.value = [];
    trips.value = [];
    unplacedPhotos.value = [];
    notes.value = [];
    selectedPlaceId.value = null;
    selectedPhotoId.value = null;
    geocodeAttempted.clear();
  }

  return {
    places,
    photos,
    trips,
    unplacedPhotos,
    notes,
    loading,
    notesLoading,
    submitting,
    selectedPlaceId,
    selectedPhotoId,
    selectedPlace,
    selectedPhoto,
    locatedPhotos,
    photosOfSelectedPlace,
    refresh,
    loadNotes,
    selectPlace,
    fillMissingNames,
    createPlace,
    updatePlace,
    deletePlace,
    mergePlace,
    assignPhotoTo,
    assignUnplacedTo,
    updatePhoto,
    deletePhoto,
    createNote,
    updateNote,
    deleteNote,
    createTrip,
    renameTrip,
    deleteTrip,
    setPlaceTrip,
    placesOfTrip,
    reset,
  };
});
