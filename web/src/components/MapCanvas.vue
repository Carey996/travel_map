<script setup lang="ts">
/**
 * 地图画布。
 *
 * 直接使用 Leaflet 而非 Vue 封装库：大头针的 HTML 结构、缩放与照片点的联动、
 * 以及坐标系纠偏都需要细粒度控制，一层薄封装反而更清晰。
 *
 * 坐标系约定：组件外部传入的坐标**一律是 WGS-84**，
 * 这里依据底图源的 crs 决定渲染前是否转成 GCJ-02；地图上点到的坐标也会转回 WGS-84 再抛出。
 */
import L from 'leaflet';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo, Place } from '@/api/client';
import { fromMapCoords, outOfChina, toMapCoords } from '@/lib/coords';
import { findProvider } from '@/lib/tiles';

export type MapFocus = { lat: number; lng: number; zoom?: number; token: number };

const props = defineProps<{
  places: Place[];
  photos: Photo[];
  selectedPlaceId: string | null;
  tileProviderId: string;
  showPhotoDots: boolean;
  placingMode: boolean;
  focus: MapFocus | null;
  fitToken: number;
}>();

const emit = defineEmits<{
  (event: 'select-place', id: string | null): void;
  (event: 'map-click', lat: number, lng: number): void;
  (event: 'photo-click', photo: Photo): void;
}>();

const { t } = useI18n();

const PHOTO_DOT_MIN_ZOOM = 14;
const MAX_PHOTO_DOTS = 400;

const container = ref<HTMLDivElement | null>(null);
const zoomLevel = ref(4);

// Leaflet 实例刻意不放进响应式系统：它内部有大量自引用，被 Proxy 包裹会出问题
let map: L.Map | null = null;
let tileLayer: L.TileLayer | null = null;
let placeLayer: L.LayerGroup | null = null;
let photoLayer: L.LayerGroup | null = null;
let resizeObserver: ResizeObserver | null = null;

const provider = computed(() => findProvider(props.tileProviderId));

/** 底图只覆盖中国大陆时（高德），视图跑到境外就浮一条说明，免得看到一片空白以为坏了 */
const outsideCoverage = ref(false);

function updateCoverage(): void {
  if (!map) return;
  const center = map.getCenter();
  outsideCoverage.value = provider.value.crs === 'GCJ02' && outOfChina(center.lat, center.lng);
}

function escapeHtml(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function placeIcon(place: Place, cover: Photo | null, selected: boolean): L.DivIcon {
  const label = place.name || t('place.unnamed');
  const count = place.photo_count ?? 0;
  const badge = count > 1 ? '<span class="pin-count">' + count + '</span>' : '';
  const inner = cover
    ? '<img src="' + cover.thumb_url + '" alt="" loading="lazy" />'
    : '<span class="pin-placeholder">' + (place.kind === 'city' ? '🏙' : '📍') + '</span>';
  const classes = ['pin'];
  if (selected) classes.push('is-selected');
  if (!place.geocoded) classes.push('is-draft');

  return L.divIcon({
    className: 'pin-wrap',
    html:
      '<div class="' + classes.join(' ') + '">' +
      '<div class="pin-bubble">' + inner + badge + '</div>' +
      '<div class="pin-label">' + escapeHtml(label) + '</div>' +
      '</div>',
    iconSize: [64, 64],
    iconAnchor: [32, 32],
  });
}

function photoIcon(photo: Photo, selected: boolean): L.DivIcon {
  return L.divIcon({
    className: 'photo-dot-wrap',
    html:
      '<div class="photo-dot' + (selected ? ' is-selected' : '') + '">' +
      '<img src="' + photo.thumb_url + '" alt="" loading="lazy" /></div>',
    iconSize: [44, 44],
    iconAnchor: [22, 22],
  });
}

function renderTile(): void {
  if (!map) return;
  if (tileLayer) {
    map.removeLayer(tileLayer);
    tileLayer = null;
  }
  const current = provider.value;
  tileLayer = L.tileLayer(current.url, {
    maxZoom: current.maxZoom,
    minZoom: current.minZoom,
    subdomains: current.subdomains.length ? current.subdomains : 'abc',
    attribution: current.attribution,
    // 刻意不设 crossOrigin：不需要读取瓦片像素，
    // 少一个 CORS 依赖就少一类「底图静默失败」的可能
  });
  tileLayer.addTo(map);
}

function renderPlaces(): void {
  if (!placeLayer) return;
  placeLayer.clearLayers();

  const covers = new Map<string, Photo>();
  for (const photo of props.photos) {
    if (photo.place_id && !covers.has(photo.place_id)) covers.set(photo.place_id, photo);
  }

  for (const place of props.places) {
    const [lat, lng] = toMapCoords(place.lat, place.lng, provider.value.crs);
    const marker = L.marker([lat, lng], {
      icon: placeIcon(place, covers.get(place.id) ?? null, place.id === props.selectedPlaceId),
      keyboard: false,
      riseOnHover: true,
      zIndexOffset: place.id === props.selectedPlaceId ? 1000 : 0,
    });
    marker.on('click', (event) => {
      L.DomEvent.stopPropagation(event);
      emit('select-place', place.id);
    });
    marker.addTo(placeLayer);
  }
}

function renderPhotoDots(): void {
  if (!photoLayer) return;
  photoLayer.clearLayers();
  if (!props.showPhotoDots || zoomLevel.value < PHOTO_DOT_MIN_ZOOM) return;

  let rendered = 0;
  for (const photo of props.photos) {
    if (photo.lat === null || photo.lng === null) continue;
    if (rendered >= MAX_PHOTO_DOTS) break;
    rendered += 1;
    const [lat, lng] = toMapCoords(photo.lat, photo.lng, provider.value.crs);
    const marker = L.marker([lat, lng], {
      icon: photoIcon(photo, photo.id === props.selectedPlaceId),
      keyboard: false,
    });
    marker.on('click', (event) => {
      L.DomEvent.stopPropagation(event);
      emit('photo-click', photo);
    });
    marker.addTo(photoLayer);
  }
}

onMounted(() => {
  if (!container.value) return;

  map = L.map(container.value, {
    center: [30, 110],
    zoom: 4,
    worldCopyJump: true,
  });
  placeLayer = L.layerGroup().addTo(map);
  photoLayer = L.layerGroup().addTo(map);

  map.on('click', (event: L.LeafletMouseEvent) => {
    const [lat, lng] = fromMapCoords(event.latlng.lat, event.latlng.lng, provider.value.crs);
    emit('map-click', lat, lng);
  });
  map.on('zoomend', () => {
    // 缩放级别只用于决定要不要显示单张照片点，因此留在组件内部，
    // 不向上层抛事件——缩放是高频事件，回写选中状态会把刚打开的面板立刻关掉
    zoomLevel.value = map ? map.getZoom() : zoomLevel.value;
    updateCoverage();
  });
  map.on('moveend', updateCoverage);

  resizeObserver = new ResizeObserver(() => map?.invalidateSize());
  resizeObserver.observe(container.value);

  renderTile();
  renderPlaces();
  updateCoverage();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
  map?.remove();
  map = null;
  tileLayer = null;
  placeLayer = null;
  photoLayer = null;
});

watch(() => props.places, renderPlaces);
watch(() => props.photos, () => {
  renderPlaces();
  renderPhotoDots();
});
watch(() => props.selectedPlaceId, () => {
  renderPlaces();
  renderPhotoDots();
});
watch(provider, () => {
  renderTile();
  renderPlaces();
  renderPhotoDots();
  updateCoverage();
});
watch([() => props.showPhotoDots, zoomLevel], renderPhotoDots);

watch(
  () => props.focus,
  (focus) => {
    if (!map || !focus) return;
    const [lat, lng] = toMapCoords(focus.lat, focus.lng, provider.value.crs);
    map.flyTo([lat, lng], focus.zoom ?? Math.max(map.getZoom(), 12), { duration: 0.8 });
  },
);

watch(
  () => props.fitToken,
  (token) => {
    if (!map || token === 0 || props.places.length === 0) return;
    const points = props.places.map((place) => {
      const [lat, lng] = toMapCoords(place.lat, place.lng, provider.value.crs);
      return L.latLng(lat, lng);
    });
    map.fitBounds(L.latLngBounds(points), { padding: [70, 70], maxZoom: 13 });
  },
);
</script>

<template>
  <div class="map-shell">
    <div ref="container" class="map-canvas" :class="{ 'is-placing': props.placingMode }" />

    <div v-if="outsideCoverage" class="map-notice">
      <el-icon><Warning /></el-icon>
      <span>{{ t('map.outsideCoverage') }}</span>
    </div>
  </div>
</template>

<style scoped>
.map-shell {
  position: absolute;
  inset: 0;
}

.map-canvas {
  position: absolute;
  inset: 0;
}

.map-notice {
  position: absolute;
  left: 50%;
  bottom: 16px;
  z-index: 700;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--el-bg-color) 92%, transparent);
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: var(--tb-shadow-1);
  color: var(--el-text-color-regular);
  font-size: 12.5px;
  pointer-events: none;
  backdrop-filter: blur(3px);
}
</style>
