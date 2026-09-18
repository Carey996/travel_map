import { computed, ref } from 'vue';
import { defineStore } from 'pinia';

import { api, type Note, type Photo, type Place, type PlaceKind } from '@/api/client';

/**
 * 相册主状态：地点、照片、笔记，以及当前选中项。
 *
 * 数据量不大（个人旅行博客），所以一次性全量拉取后在内存里派生各种视图，
 * 比给每个视图单独发请求更简单也更快。每次写操作后调用 refresh() 重新同步。
 */
export const useGalleryStore = defineStore('gallery', () => {
  const places = ref<Place[]>([]);
  const photos = ref<Photo[]>([]);
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
      const [nextPlaces, nextPhotos, nextUnplaced] = await Promise.all([
        api.listPlaces(),
        api.listPhotos({ limit: 2000 }),
        api.listPhotos({ unplaced: true, limit: 2000 }),
      ]);
      places.value = nextPlaces;
      photos.value = nextPhotos;
      unplacedPhotos.value = nextUnplaced;
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

  /** 给还没有名字的地点补一个反向地理编码结果。失败静默——用户可以手动改名。 */
  async function fillMissingNames(): Promise<boolean> {
    const pending = places.value.filter(
      (place) => !place.geocoded && !geocodeAttempted.has(place.id),
    );
    if (pending.length === 0) return false;

    let changed = false;
    for (const place of pending) {
      geocodeAttempted.add(place.id);
      try {
        const info = await api.reverseGeocode(place.lat, place.lng);
        await api.updatePlace(place.id, {
          name: info.name,
          kind: info.kind,
          address: info.display_name,
        });
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

  async function assignUnplacedTo(placeId: string): Promise<void> {
    const ids = unplacedPhotos.value.map((photo) => photo.id);
    if (ids.length === 0) return;
    await api.assignPhotos(placeId, ids);
    await refresh();
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
    unplacedPhotos.value = [];
    notes.value = [];
    selectedPlaceId.value = null;
    selectedPhotoId.value = null;
    geocodeAttempted.clear();
  }

  return {
    places,
    photos,
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
    assignUnplacedTo,
    updatePhoto,
    deletePhoto,
    createNote,
    updateNote,
    deleteNote,
    reset,
  };
});
