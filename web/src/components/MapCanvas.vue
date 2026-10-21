<script setup lang="ts">
/**
 * 地图画布。
 *
 * 直接使用 Leaflet 而非 Vue 封装库：大头针的 HTML 结构、缩放与照片点的联动、
 * 以及坐标系纠偏都需要细粒度控制，一层薄封装反而更清晰。
 *
 * 坐标系约定：组件外部传入的坐标**一律是 WGS-84**，
 * 这里依据底图源的 crs 决定渲染前是否转成 GCJ-02；地图上点到的坐标也会转回 WGS-84 再抛出。
 *
 * 两件和缩放有关的事：
 *  1. **归一**：缩小到一定程度后不再画单个地点，而是按行政区划聚成「省 / 市」一个气泡，
 *     数字是该组里的照片总数；点一下放大到那一组。
 *  2. **旅行框**：同一个 trip 下的地点用同一个颜色的虚线框圈起来，框上带旅行名。
 */
import L from 'leaflet';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import type { Photo, Place, Trip } from '@/api/client';
import { fromMapCoords, outOfChina, toMapCoords } from '@/lib/coords';
import { findProvider } from '@/lib/tiles';
import { tripColor } from '@/lib/trips';

export type MapFocus = { lat: number; lng: number; zoom?: number; token: number };

const props = defineProps<{
  places: Place[];
  photos: Photo[];
  trips: Trip[];
  selectedPlaceId: string | null;
  tileProviderId: string;
  showPhotoDots: boolean;
  placingMode: boolean;
  focus: MapFocus | null;
  fitToken: number;
  /** 来自侧栏的「看这次旅行」请求：token 变化即触发一次 fitBounds */
  focusTrip: { id: string; token: number } | null;
}>();

const emit = defineEmits<{
  (event: 'select-place', id: string | null): void;
  (event: 'map-click', lat: number, lng: number): void;
  (event: 'photo-click', photo: Photo): void;
  (event: 'focus-trip', tripId: string): void;
}>();

const { t } = useI18n();

const PHOTO_DOT_MIN_ZOOM = 14;
const MAX_PHOTO_DOTS = 400;

const container = ref<HTMLDivElement | null>(null);
const zoomLevel = ref(4);

// Leaflet 实例刻意不放进响应式系统：它内部有大量自引用，被 Proxy 包裹会出问题
let map: L.Map | null = null;
let tileLayer: L.TileLayer | null = null;
let tripLayer: L.LayerGroup | null = null;
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

// ---------------------------------------------------------------------------
// 缩放归一：按行政区划把地点聚成一组
// ---------------------------------------------------------------------------

type ClusterLevel = 'country' | 'province' | 'city' | 'place';

/** 缩放分档：越往外归得越粗。改这里就能调整手感。 */
function levelForZoom(zoom: number): ClusterLevel {
  if (zoom <= 5) return 'country';
  if (zoom <= 8) return 'province';
  if (zoom <= 11) return 'city';
  return 'place';
}

/** 点开一个分组后要飞到哪一级 */
function drillZoom(level: ClusterLevel): number {
  switch (level) {
    case 'country':
      return 7;
    case 'province':
      return 10;
    case 'city':
      return 13;
    default:
      return 14;
  }
}

type Cluster = {
  key: string;
  level: ClusterLevel;
  label: string;
  places: Place[];
  photoCount: number;
  lat: number;
  lng: number;
};

function adminLabelOf(place: Place, level: ClusterLevel): string {
  if (level === 'country') return place.country || '';
  if (level === 'province') return place.province || '';
  if (level === 'city') return place.city || '';
  return '';
}

/** 没有行政区划时按网格兜底，避免把相隔上千公里的无名地点并成一个点 */
function gridKeyOf(place: Place, level: ClusterLevel): string {
  const size = level === 'country' ? 10 : level === 'province' ? 2 : 0.5;
  return Math.round(place.lat / size) + ':' + Math.round(place.lng / size);
}

function buildClusters(level: ClusterLevel): Cluster[] {
  const groups = new Map<string, { label: string; places: Place[] }>();

  for (const place of props.places) {
    const label = adminLabelOf(place, level);
    const key = label ? level + '|' + label : level + '|grid|' + gridKeyOf(place, level);
    const existing = groups.get(key);
    if (existing) existing.places.push(place);
    else groups.set(key, { label, places: [place] });
  }

  const out: Cluster[] = [];
  for (const [key, group] of groups) {
    let lat = 0;
    let lng = 0;
    let photoCount = 0;
    for (const place of group.places) {
      lat += place.lat;
      lng += place.lng;
      photoCount += place.photo_count ?? 0;
    }
    out.push({
      key,
      level,
      label: group.label || (group.places.length === 1 ? group.places[0].name : ''),
      places: group.places,
      photoCount,
      lat: lat / group.places.length,
      lng: lng / group.places.length,
    });
  }
  return out;
}

// ---------------------------------------------------------------------------
// 图标
// ---------------------------------------------------------------------------

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

/** 归一后的分组气泡：照片数 + 行政区名；没有名字时只显示数字 */
function clusterIcon(cluster: Cluster): L.DivIcon {
  const count = cluster.photoCount || cluster.places.length;
  const label = cluster.label
    ? '<span class="cluster-label">' + escapeHtml(cluster.label) + '</span>'
    : '';
  return L.divIcon({
    className: 'cluster-wrap',
    html:
      '<div class="cluster cluster-' + cluster.level + '">' +
      '<span class="cluster-count">' + count + '</span>' + label +
      '</div>',
    iconSize: [72, 40],
    iconAnchor: [36, 20],
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

// ---------------------------------------------------------------------------
// 图层渲染
// ---------------------------------------------------------------------------

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

  const level = levelForZoom(zoomLevel.value);

  if (level === 'place') {
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
    return;
  }

  // 归一：一个分组一个气泡，点一下放大到这一组
  for (const cluster of buildClusters(level)) {
    const [lat, lng] = toMapCoords(cluster.lat, cluster.lng, provider.value.crs);
    const marker = L.marker([lat, lng], {
      icon: clusterIcon(cluster),
      keyboard: false,
      riseOnHover: true,
      // 原生 title：悬停时同时给出地点数与照片数，避免「那个数字到底数什么」的歧义
      title: [
        cluster.label,
        t('place.count', { n: cluster.places.length }),
        t('trip.photoCount', { n: cluster.photoCount }),
      ]
        .filter(Boolean)
        .join(' · '),
    });
    marker.on('click', (event) => {
      L.DomEvent.stopPropagation(event);
      if (!map) return;
      const points = cluster.places.map((place) => {
        const [plat, plng] = toMapCoords(place.lat, place.lng, provider.value.crs);
        return L.latLng(plat, plng);
      });
      map.fitBounds(L.latLngBounds(points), { padding: [80, 80], maxZoom: drillZoom(level) });
    });
    marker.addTo(placeLayer);
  }
}

/** 旅行框：同一 trip 的地点用同色虚线框圈起来，框上带旅行名 */

function renderTrips(): void {
  if (!tripLayer) return;
  tripLayer.clearLayers();

  for (const trip of props.trips) {
    const members = props.places.filter((place) => place.trip_id === trip.id);
    if (members.length === 0) continue;

    const color = tripColor(trip.id);
    const points = members.map((place) => toMapCoords(place.lat, place.lng, provider.value.crs));
    const lats = points.map((point) => point[0]);
    const lngs = points.map((point) => point[1]);
    const minLat = Math.min(...lats);
    const maxLat = Math.max(...lats);
    const minLng = Math.min(...lngs);
    const maxLng = Math.max(...lngs);
    // 只有一个地点时也要有可见的框，所以给一个最小半径（约 2~3 km）
    const padLat = Math.max(0.015, (maxLat - minLat) * 0.15);
    const padLng = Math.max(0.015, (maxLng - minLng) * 0.15);

    const rect = L.rectangle(
      [
        [minLat - padLat, minLng - padLng],
        [maxLat + padLat, maxLng + padLng],
      ],
      {
        color,
        weight: 2,
        opacity: 0.85,
        fillColor: color,
        fillOpacity: 0.08,
        dashArray: '7 5',
      },
    );
    rect.bindTooltip(
      '<span class="trip-chip" style="background:' + color + '">' +
        escapeHtml(trip.title || t('trip.untitled')) + '</span>',
      { permanent: true, direction: 'top', className: 'trip-tip', opacity: 1, offset: [0, -2] },
    );
    rect.on('click', (event) => {
      L.DomEvent.stopPropagation(event);
      emit('focus-trip', trip.id);
    });
    rect.addTo(tripLayer);
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
  // 顺序即层级：旅行框在最下面，其次是地点气泡，最后是单张照片点
  tripLayer = L.layerGroup().addTo(map);
  placeLayer = L.layerGroup().addTo(map);
  photoLayer = L.layerGroup().addTo(map);

  map.on('click', (event: L.LeafletMouseEvent) => {
    const [lat, lng] = fromMapCoords(event.latlng.lat, event.latlng.lng, provider.value.crs);
    emit('map-click', lat, lng);
  });
  map.on('zoomend', () => {
    // 缩放级别只用于决定要不要显示单张照片点、以及聚合到哪一级，因此留在组件内部，
    // 不向上层抛事件——缩放是高频事件，回写选中状态会把刚打开的面板立刻关掉
    zoomLevel.value = map ? map.getZoom() : zoomLevel.value;
    updateCoverage();
  });
  map.on('moveend', updateCoverage);

  resizeObserver = new ResizeObserver(() => map?.invalidateSize());
  resizeObserver.observe(container.value);

  renderTile();
  renderTrips();
  renderPlaces();
  updateCoverage();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
  map?.remove();
  map = null;
  tileLayer = null;
  tripLayer = null;
  placeLayer = null;
  photoLayer = null;
});

watch(() => props.places, () => {
  renderTrips();
  renderPlaces();
});
watch(() => props.trips, renderTrips);
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
  renderTrips();
  renderPlaces();
  renderPhotoDots();
  updateCoverage();
});
watch([() => props.showPhotoDots, zoomLevel], () => {
  renderPlaces();
  renderPhotoDots();
});

watch(
  () => props.focus,
  (focus) => {
    if (!map || !focus) return;
    const [lat, lng] = toMapCoords(focus.lat, focus.lng, provider.value.crs);
    map.flyTo([lat, lng], focus.zoom ?? Math.max(map.getZoom(), 12), { duration: 0.8 });
  },
);

watch(
  () => props.focusTrip,
  (request) => {
    if (!map || !request) return;
    const members = props.places.filter((place) => place.trip_id === request.id);
    if (members.length === 0) return;
    const points = members.map((place) => {
      const [lat, lng] = toMapCoords(place.lat, place.lng, provider.value.crs);
      return L.latLng(lat, lng);
    });
    map.fitBounds(L.latLngBounds(points), { padding: [90, 90], maxZoom: 12 });
  },
);

watch(
  () => props.fitToken,
  (token) => {
    if (!map || token === 0) return;
    const points: L.LatLng[] = props.places.map((place) => {
      const [lat, lng] = toMapCoords(place.lat, place.lng, provider.value.crs);
      return L.latLng(lat, lng);
    });
    if (points.length === 0) return;
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
