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
import { fromMapCoords, toMapCoords } from '@/lib/coords';
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

/** 视图跑到中国大陆以外时自动换用的底图（高德只覆盖大陆，境外会是一片空白） */
const FALLBACK_PROVIDER_ID = 'esri-street';

/** 是否已切到境外底图 */
const outsideChina = ref(false);

const provider = computed(() =>
  findProvider(outsideChina.value ? FALLBACK_PROVIDER_ID : props.tileProviderId),
);

/**
 * 判断视图中心是否已经离开中国大陆，据此自动切换底图。
 *
 * 带滞回：出去时按原始范围判断，回来时要求更深地进入境内（内缩 1°），
 * 否则在边界附近来回平移会让底图反复横跳。
 */
const CHINA_BOX = { minLng: 73.5, maxLng: 135.1, minLat: 18.1, maxLat: 53.6 };
const CHINA_MARGIN_DEG = 1;

function outsideMainland(lat: number, lng: number, currentlyOutside: boolean): boolean {
  const inside = (margin: number): boolean =>
    lat > CHINA_BOX.minLat + margin &&
    lat < CHINA_BOX.maxLat - margin &&
    lng > CHINA_BOX.minLng + margin &&
    lng < CHINA_BOX.maxLng - margin;
  // 已经在境外时，要求进到境内 1° 以内才切回高德；否则一离开就算境外
  return currentlyOutside ? !inside(CHINA_MARGIN_DEG) : !inside(0);
}

function updateCoverage(): void {
  if (!map) return;
  const center = map.getCenter();
  outsideChina.value = outsideMainland(center.lat, center.lng, outsideChina.value);
}

// ---------------------------------------------------------------------------
// 缩放归一：按行政区划把地点聚成一组
// ---------------------------------------------------------------------------

type ClusterLevel = 'country' | 'province' | 'city' | 'district' | 'place';

/**
 * 缩放分档：越往外归得越粗。改这里就能调整手感。
 *
 *   国家(z≤5) → 省(6-7) → 市(8-9) → 区/县(10-11) → 地点(≥12)
 */
function levelForZoom(zoom: number): ClusterLevel {
  if (zoom <= 5) return 'country';
  if (zoom <= 7) return 'province';
  if (zoom <= 9) return 'city';
  if (zoom <= 11) return 'district';
  return 'place';
}

/** 点开一个分组后要飞到哪一级 */
function drillZoom(level: ClusterLevel): number {
  switch (level) {
    case 'country':
      return 7;
    case 'province':
      return 9;
    case 'city':
      return 11;
    case 'district':
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
  if (level === 'district') return place.district || '';
  return '';
}

/** 没有行政区划时按网格兜底，避免把相隔上千公里的无名地点并成一个点 */
function gridKeyOf(place: Place, level: ClusterLevel): string {
  const size = level === 'country' ? 10 : level === 'province' ? 2 : level === 'city' ? 0.5 : 0.1;
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

/**
 * 归一后的分组标记。
 *
 * 有样图时画成「前三张叠放」——和相册 App 的分组卡片一个意思，
 * 一眼能看出这一堆是什么；没有样图（地点还没照片）就退回纯色气泡。
 */
function clusterIcon(cluster: Cluster, samples: string[]): L.DivIcon {
  const count = cluster.photoCount || cluster.places.length;
  const label = cluster.label
    ? '<span class="cluster-label">' + escapeHtml(cluster.label) + '</span>'
    : '';

  if (samples.length === 0) {
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

  // 最上面那张用轮播序号最大的类名，方便 CSS 逐层错位
  const stack = samples
    .map((url, index) => '<img class="cluster-img stack-' + (samples.length - index) + '" src="' + url + '" alt="" loading="lazy" />')
    .join('');

  return L.divIcon({
    className: 'cluster-wrap',
    html:
      '<div class="cluster-card cluster-' + cluster.level + '">' +
      '<div class="cluster-stack">' + stack +
      '<span class="cluster-count">' + count + '</span></div>' +
      label +
      '</div>',
    iconSize: [76, 86],
    iconAnchor: [38, 43],
  });
}

/** 每个地点先取三张缩略图，聚类时拼成叠放样图 */
function buildSampleMap(): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const photo of props.photos) {
    if (!photo.place_id) continue;
    const list = map.get(photo.place_id);
    if (!list) map.set(photo.place_id, [photo.thumb_url]);
    else if (list.length < 3) list.push(photo.thumb_url);
  }
  return map;
}

/**
 * 取最多三张样图，**按地点轮转**：先给每个地点取一张，不够再取第二张。
 * 否则一个地点有 3 张照片时，叠放的全是同一处，看不出这组里还有别的地方。
 */
function collectSamples(cluster: Cluster, map: Map<string, string[]>): string[] {
  const lists = cluster.places.map((place) => map.get(place.id) ?? []).filter((list) => list.length > 0);
  const out: string[] = [];
  for (let round = 0; out.length < 3; round += 1) {
    let picked = false;
    for (const list of lists) {
      if (round < list.length) {
        out.push(list[round]);
        picked = true;
        if (out.length >= 3) break;
      }
    }
    if (!picked) break;
  }
  return out;
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

  // 归一：一个分组一个标记（前三张叠放），点一下放大到这一组
  const sampleMap = buildSampleMap();
  for (const cluster of buildClusters(level)) {
    const [lat, lng] = toMapCoords(cluster.lat, cluster.lng, provider.value.crs);
    const marker = L.marker([lat, lng], {
      icon: clusterIcon(cluster, collectSamples(cluster, sampleMap)),
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

/** 把地理包围盒换算成屏幕像素尺寸（没有地图时给 0） */
function tripScreenSize(bounds: L.LatLngBounds): { w: number; h: number } {
  if (!map) return { w: 0, h: 0 };
  const nw = map.latLngToContainerPoint(bounds.getNorthWest());
  const se = map.latLngToContainerPoint(bounds.getSouthEast());
  return { w: Math.abs(se.x - nw.x), h: Math.abs(se.y - nw.y) };
}

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

    const bounds = L.latLngBounds([
      [minLat - padLat, minLng - padLng],
      [maxLat + padLat, maxLng + padLng],
    ]);

    // 框在屏幕上的实际大小：缩得越远框越小，而标签大小是固定的，
    // 所以在省级/国家级这种缩放下标签会把旁边的分组卡片整个盖住。
    const size = tripScreenSize(bounds);
    const tooSmall = size.w < 26 && size.h < 26;

    const rect = L.rectangle(bounds, {
      color,
      weight: tooSmall ? 1.5 : 2,
      opacity: tooSmall ? 0.6 : 0.85,
      fillColor: color,
      fillOpacity: tooSmall ? 0 : 0.08,
      dashArray: '7 5',
    });

    // 只在框足够大（看得出是一块区域）时才挂常驻标签
    if (size.w >= 40 && size.h >= 40) {
      rect.bindTooltip(
        '<span class="trip-chip" style="background:' + color + '">' +
          escapeHtml(trip.title || t('trip.untitled')) + '</span>',
        // center：框常常比视口还高，靠上对齐会让标签跑到屏幕外
        { permanent: true, direction: 'center', className: 'trip-tip', opacity: 1 },
      );
    }
    rect.on('click', (event) => {
      L.DomEvent.stopPropagation(event);
      emit('focus-trip', trip.id);
    });
    rect.addTo(tripLayer);
  }
}

/**
 * 每个地点被当作封面（大头针圆图）的那张照片。
 * 选取规则必须和 renderPlaces 里一致：props.photos 里该地点的第一张。
 */
function coverPhotoIds(): Set<string> {
  const seenPlaces = new Set<string>();
  const covers = new Set<string>();
  for (const photo of props.photos) {
    if (!photo.place_id || seenPlaces.has(photo.place_id)) continue;
    seenPlaces.add(photo.place_id);
    covers.add(photo.id);
  }
  return covers;
}

function renderPhotoDots(): void {
  if (!photoLayer) return;
  photoLayer.clearLayers();
  if (!props.showPhotoDots || zoomLevel.value < PHOTO_DOT_MIN_ZOOM) return;

  // 封面照片已经画在地点大头针里了；再画一个照片点就是「同一张照片叠两层」
  const covers = coverPhotoIds();

  // 屏幕上已经占用的位置：先放大头针（它就代表「这里有照片」），再放照片点。
  // 同一处的照片在地图上会完全重合，叠成一坨看起来就像「两张一样的图」，
  // 所以同一格只保留一个标记，其余照片在侧栏/抽屉里照样能点开。
  const DOT_MIN_GAP = 40;
  const taken: L.Point[] = [];
  if (map) {
    for (const place of props.places) {
      const [pLat, pLng] = toMapCoords(place.lat, place.lng, provider.value.crs);
      taken.push(map.latLngToContainerPoint([pLat, pLng]));
    }
  }
  const collides = (point: L.Point): boolean =>
    taken.some((other) => Math.abs(other.x - point.x) < DOT_MIN_GAP && Math.abs(other.y - point.y) < DOT_MIN_GAP);

  let rendered = 0;
  for (const photo of props.photos) {
    if (photo.lat === null || photo.lng === null) continue;
    if (covers.has(photo.id)) continue;
    if (rendered >= MAX_PHOTO_DOTS) break;
    const [lat, lng] = toMapCoords(photo.lat, photo.lng, provider.value.crs);
    if (map) {
      const point = map.latLngToContainerPoint([lat, lng]);
      if (collides(point)) continue;
      taken.push(point);
    }
    rendered += 1;
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
  renderTrips();
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

    <div v-if="outsideChina" class="map-notice">
      <el-icon><Globe /></el-icon>
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
