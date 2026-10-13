/**
 * 底图源定义。
 *
 * 每个源都带 `crs` 字段，标明它用的是 WGS-84 还是 GCJ-02。
 * 地图渲染与点击取点都依据这个字段决定是否做坐标纠偏。
 *
 * **当前只启用高德**（见 `ENABLED_TILE_PROVIDER_IDS`）：中文标注、国内路网最全。
 * 代价有两条，都已经在别处兜住：
 *
 *   1. 高德是 GCJ-02，渲染前必须纠偏 —— 见 `lib/coords.ts`；
 *   2. 瓦片只覆盖中国大陆，境外会是空白 —— `MapCanvas` 会在这种情况下浮出提示。
 *
 * 其余源的定义保留在下面（附实测数据），想恢复全球底图只需把 id 加回
 * `ENABLED_TILE_PROVIDER_IDS`，TopBar 的底图下拉会自动多出选项。
 *
 * | 源            | 京都 z13 | 上海 z13 | 冰岛 z13 | 说明                       |
 * |---------------|----------|----------|----------|----------------------------|
 * | Esri 街道     | 33 KB    | 46 KB    | 26 KB    | 全球可用，WGS-84           |
 * | Esri 影像     | 25 KB    | 22 KB    | 22 KB    | 全球可用，WGS-84           |
 * | 高德 矢量     | 179 B ⚠  | 24 KB    | 179 B ⚠  | 仅中国大陆有数据，GCJ-02   |
 * | 腾讯          | 169 B ⚠  | 169 B ⚠  | 169 B ⚠  | 本网络下全是空白           |
 * | OSM 官方      | 不可达   | 不可达   | 不可达   | 本网络下完全连不上         |
 *
 * 判据：空白瓦片是约 179 字节的单色 PNG，而真实瓦片是几万字节。
 */

export type Crs = 'WGS84' | 'GCJ02';

export type TileProvider = {
  id: string;
  labelZh: string;
  labelEn: string;
  url: string;
  attribution: string;
  minZoom: number;
  maxZoom: number;
  subdomains: string[];
  crs: Crs;
};

const ESRI = 'https://server.arcgisonline.com/ArcGIS/rest/services';

export const TILE_PROVIDERS: TileProvider[] = [
  {
    id: 'amap-vector',
    labelZh: '高德标准图',
    labelEn: 'Amap standard',
    url: 'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
    attribution: '© 高德地图',
    minZoom: 3,
    maxZoom: 18,
    subdomains: ['1', '2', '3', '4'],
    crs: 'GCJ02',
  },
  {
    id: 'amap-satellite',
    labelZh: '高德卫星图',
    labelEn: 'Amap satellite',
    url: 'https://webst0{s}.is.autonavi.com/appmaptile?style=6&x={x}&y={y}&z={z}',
    attribution: '© 高德地图',
    minZoom: 3,
    maxZoom: 18,
    subdomains: ['1', '2', '3', '4'],
    crs: 'GCJ02',
  },
  {
    id: 'esri-street',
    labelZh: '街道（全球）',
    labelEn: 'Streets (global)',
    url: ESRI + '/World_Street_Map/MapServer/tile/{z}/{y}/{x}',
    attribution: '© Esri, HERE, Garmin, OpenStreetMap contributors',
    minZoom: 2,
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
    minZoom: 2,
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
    minZoom: 2,
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
  {
    id: 'osm',
    labelZh: 'OpenStreetMap',
    labelEn: 'OpenStreetMap',
    url: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
    attribution: '© OpenStreetMap contributors',
    minZoom: 2,
    maxZoom: 19,
    subdomains: [],
    crs: 'WGS84',
  },
];

/** 界面里实际提供的底图源。顺序即下拉里的顺序。 */
export const ENABLED_TILE_PROVIDER_IDS = ['amap-vector', 'amap-satellite'];

export const AVAILABLE_TILE_PROVIDERS: TileProvider[] = TILE_PROVIDERS.filter((provider) =>
  ENABLED_TILE_PROVIDER_IDS.includes(provider.id),
);

/** 默认底图，必须出现在 ENABLED_TILE_PROVIDER_IDS 里。 */
export const DEFAULT_TILE_PROVIDER = 'amap-vector';

export function findProvider(id: string): TileProvider {
  return (
    TILE_PROVIDERS.find((provider) => provider.id === id) ??
    TILE_PROVIDERS.find((provider) => provider.id === DEFAULT_TILE_PROVIDER) ??
    TILE_PROVIDERS[0]
  );
}
