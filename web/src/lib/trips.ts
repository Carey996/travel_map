/**
 * 行程（旅行）的展示逻辑：固定配色 + 从 id 取色。
 *
 * 颜色按 id 哈希取，保证同一次旅行在列表、地图框、标签上的颜色始终一致，
 * 且不依赖后端存储——服务端保持无状态，颜色纯粹是表现层的事。
 */

export const TRIP_COLORS = [
  '#3b82f6',
  '#f97316',
  '#10b981',
  '#a855f7',
  '#06b6d4',
  '#ec4899',
  '#84cc16',
  '#f59e0b',
];

export function tripColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) hash = (hash * 31 + id.charCodeAt(i)) % 100000;
  return TRIP_COLORS[hash % TRIP_COLORS.length];
}
