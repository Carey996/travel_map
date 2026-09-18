"""查询参数解析助手。"""

from __future__ import annotations

from app.core.errors import ApiError


def parse_bbox(raw: str | None) -> tuple[float, float, float, float] | None:
    """解析 minLng,minLat,maxLng,maxLat 形式的视口范围。"""
    if not raw:
        return None
    parts = [p.strip() for p in raw.split(",")]
    if len(parts) != 4:
        raise ApiError(422, "invalid_bbox", "bbox 需要 4 个数字：minLng,minLat,maxLng,maxLat")
    try:
        min_lng, min_lat, max_lng, max_lat = (float(p) for p in parts)
    except ValueError as exc:
        raise ApiError(422, "invalid_bbox", "bbox 含有无法解析的数字") from exc
    if min_lng > max_lng or min_lat > max_lat:
        raise ApiError(422, "invalid_bbox", "bbox 的最小值必须小于最大值")
    return min_lng, min_lat, max_lng, max_lat
