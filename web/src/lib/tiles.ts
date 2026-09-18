/**
 * 底图源定义。
 *
 * 每个源都带 `crs` 字段，标明它用的是 WGS-84 还是 GCJ-02。
 * 地图渲染与点击取点都依据这个字段决定是否做坐标纠偏。
 *
 * 选型来自实测（本机网络 + 全球覆盖）：
 *
 * | 源            | 京都 z13 | 上海 z13 | 冰岛 z13 | 说明                       |
 * |---------------|----------|----------|----------|----------------------------|
 * | Esri 街道     | 33 KB    | 46 KB    | 26 KB    | 全球可用，WGS-84           |
 * | Esri 影像     | 25 KB    | 22 KB    | 22 KB    | 全球可用，WGS-84           |
 * | 高德 矢量     | 179 B ⚠  | 24 KB    | 179 B ⚠  | 仅中国大陆有数据，GCJ-02   |
 * | 腾讯          | 169 B ⚠  | 169 B ⚠  | 169 B ⚠  | 本网络下全是空白           |
 * | OSM 官方      | 不可达   | 不可达   | 不可达   | 本网络下完全连不上         |
 *
 * 结论：默认用 Esri（全球覆盖 + 无需纠偏），高德作为「只在国内用」的可选项保留，
 * 其 GCJ-02 偏移由 lib/coords.ts 处理。部署到能访问 OSM 的网络时可切回 osm。
 */

export type Crs = 'WGS84' | 'GCJ02';

export type TileProvider = {
  id: string;
  labelZh: string;
  labelEn: string;
  url: string;
  attribution: string;
  maxZoom: number;
  subdomains: string[];
  crs: Crs;
};

const ESRI = 'https://server.arcgisonline.com/ArcGIS/rest/services';

export const TILE_PROVIDERS: TileProvider[] = [
  {
    id: 'esri-street',
    labelZh: '街道（全球）',
    labelEn: 'Streets (global)',
    url: ESRI + '/World_Street_Map/MapServer/tile/{z}/{y}/{x}',
    attribution: '© Esri, HERE, Garmin, OpenStreetMap contributors',
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
  {
    id: 'esri-imagery',
    labelZh: '卫星影像（全球）',
    labelEn: 'Imagery (global)',
    url: ESRI + '/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    attribution: '© Esri, Maxar, Earthstar Geographics',
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
  {
    id: 'esri-topo',
    labelZh: '地形（全球）',
    labelEn: 'Topographic (global)',
    url: ESRI + '/World_Topo_Map/MapServer/tile/{z}/{y}/{x}',
    attribution: '© Esri, HERE, Garmin, USGS, NGA',
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
  {
    id: 'amap-vector',
    labelZh: '高德（仅中国大陆）',
    labelEn: 'Amap (mainland China only)',
    url: 'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
    attribution: '© 高德地图',
    maxZoom: 18,
    subdomains: ['1', '2', '3', '4'],
    crs: 'GCJ02',
  },
  {
    id: 'amap-satellite',
    labelZh: '高德卫星（仅中国大陆）',
    labelEn: 'Amap satellite (mainland China only)',
    url: 'https://webst0{s}.is.autonavi.com/appmaptile?style=6&x={x}&y={y}&z={z}',
    attribution: '© 高德地图',
    maxZoom: 18,
    subdomains: ['1', '2', '3', '4'],
    crs: 'GCJ02',
  },
  {
    id: 'osm',
    labelZh: 'OpenStreetMap',
    labelEn: 'OpenStreetMap',
    url: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
    attribution: '© OpenStreetMap contributors',
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
];

export const DEFAULT_TILE_PROVIDER = 'esri-street';

export function findProvider(id: string): TileProvider {
  return TILE_PROVIDERS.find((p) => p.id === id) ?? TILE_PROVIDERS[0];
}
