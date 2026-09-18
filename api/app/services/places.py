"""地点归属逻辑。

上传的照片若带 GPS，会自动归入最近的地点；1.5 km 内没有已有地点时，
就地新建一个「未命名地点」，由前端随后调反向地理编码补上名字。
"""

from __future__ import annotations

import uuid

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.geo import haversine_m
from app.db.models import Photo, Place

# 同一个地点内的照片通常相距很近；1.5 km 能覆盖一座小城中心或一个景区
AUTO_ASSIGN_RADIUS_M = 1500.0


async def nearest_place(
    db: AsyncSession, user_id: uuid.UUID, lat: float, lng: float, *, max_distance_m: float
) -> tuple[Place | None, float]:
    rows = (await db.execute(select(Place).where(Place.user_id == user_id))).scalars().all()
    best: Place | None = None
    best_distance = float("inf")
    for place in rows:
        distance = haversine_m(lat, lng, place.lat, place.lng)
        if distance < best_distance:
            best, best_distance = place, distance
    if best is not None and best_distance <= max_distance_m:
        return best, best_distance
    return None, best_distance


async def assign_photo(
    db: AsyncSession,
    user_id: uuid.UUID,
    photo: Photo,
    *,
    radius_m: float = AUTO_ASSIGN_RADIUS_M,
) -> Place | None:
    """把照片归入地点；必要时新建。返回最终归属的地点（无 GPS 时为 None）。"""
    if photo.lat is None or photo.lng is None:
        return None
    if photo.place_id is not None:
        place = await db.get(Place, photo.place_id)
        if place is not None:
            _extend_date_range(place, photo)
        return place

    place, _ = await nearest_place(db, user_id, photo.lat, photo.lng, max_distance_m=radius_m)
    if place is None:
        place = Place(
            user_id=user_id,
            name="",
            kind="spot",
            lat=photo.lat,
            lng=photo.lng,
            geocoded=False,
        )
        db.add(place)
        await db.flush()

    photo.place_id = place.id
    _extend_date_range(place, photo)
    return place


def _extend_date_range(place: Place, photo: Photo) -> None:
    """让地点的时间范围自动覆盖其照片的拍摄日期，前端列表因此能显示真实行程日期。"""
    if photo.taken_at is None:
        return
    taken = photo.taken_at.date() if hasattr(photo.taken_at, "date") else photo.taken_at
    if place.date_from is None or taken < place.date_from:
        place.date_from = taken
    if place.date_to is None or taken > place.date_to:
        place.date_to = taken
