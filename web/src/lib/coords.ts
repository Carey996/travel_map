/**
 * WGS-84 <-> GCJ-02 坐标转换。
 *
 * 为什么必须有这个文件：照片 EXIF 里的 GPS 是 WGS-84（国际标准），
 * 而高德/腾讯等国内底图使用 GCJ-02（俗称火星坐标）。两者在国内相差
 * 100~600 米，直接把 WGS-84 的点画在 GCJ-02 底图上会出现肉眼可见的偏移。
 *
 * 约定：**数据库里永远只存 WGS-84**，只在渲染到 GCJ-02 底图的那一刻转换。
 * 这样坐标数据始终是干净、可移植的。
 */

const PI = Math.PI;
const A = 6378245.0; // 克拉索夫斯基椭球长半轴
const EE = 0.00669342162296594323; // 第一偏心率平方

/** 中国大陆粗略范围之外不做偏移（港澳台及境外本身就是 WGS-84 显示） */
export function outOfChina(lat: number, lng: number): boolean {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271;
}

function transformLat(x: number, y: number): number {
  let ret =
    -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x));
  ret += ((20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0) / 3.0;
  ret += ((20.0 * Math.sin(y * PI) + 40.0 * Math.sin((y / 3.0) * PI)) * 2.0) / 3.0;
  ret += ((160.0 * Math.sin((y / 12.0) * PI) + 320 * Math.sin((y * PI) / 30.0)) * 2.0) / 3.0;
  return ret;
}

function transformLng(x: number, y: number): number {
  let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x));
  ret += ((20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0) / 3.0;
  ret += ((20.0 * Math.sin(x * PI) + 40.0 * Math.sin((x / 3.0) * PI)) * 2.0) / 3.0;
  ret += ((150.0 * Math.sin((x / 12.0) * PI) + 300.0 * Math.sin((x / 30.0) * PI)) * 2.0) / 3.0;
  return ret;
}

export function wgs84ToGcj02(lat: number, lng: number): [number, number] {
  if (outOfChina(lat, lng)) return [lat, lng];
  let dLat = transformLat(lng - 105.0, lat - 35.0);
  let dLng = transformLng(lng - 105.0, lat - 35.0);
  const radLat = (lat / 180.0) * PI;
  let magic = Math.sin(radLat);
  magic = 1 - EE * magic * magic;
  const sqrtMagic = Math.sqrt(magic);
  dLat = (dLat * 180.0) / (((A * (1 - EE)) / (magic * sqrtMagic)) * PI);
  dLng = (dLng * 180.0) / ((A / sqrtMagic) * Math.cos(radLat) * PI);
  return [lat + dLat, lng + dLng];
}

/** 迭代反解，精度优于简单做差 */
export function gcj02ToWgs84(lat: number, lng: number): [number, number] {
  if (outOfChina(lat, lng)) return [lat, lng];
  let wgsLat = lat;
  let wgsLng = lng;
  for (let i = 0; i < 6; i += 1) {
    const [gLat, gLng] = wgs84ToGcj02(wgsLat, wgsLng);
    wgsLat += lat - gLat;
    wgsLng += lng - gLng;
  }
  return [wgsLat, wgsLng];
}

/** 按底图坐标系把 WGS-84 坐标转成「画在地图上」的坐标 */
export function toMapCoords(
  lat: number,
  lng: number,
  crs: 'WGS84' | 'GCJ02',
): [number, number] {
  return crs === 'GCJ02' ? wgs84ToGcj02(lat, lng) : [lat, lng];
}

/** 把地图上点到的坐标转回 WGS-84 再入库 */
export function fromMapCoords(
  lat: number,
  lng: number,
  crs: 'WGS84' | 'GCJ02',
): [number, number] {
  return crs === 'GCJ02' ? gcj02ToWgs84(lat, lng) : [lat, lng];
}
