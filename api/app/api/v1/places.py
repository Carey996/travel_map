"""地点的增删改查与照片归属。"""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Query, status
from sqlalchemy import delete, func, select, update

from app.api.deps import CurrentUser, DbSession, Storage
from app.api.query import parse_bbox
from app.core.errors import ApiError
from app.db.models import Note, Photo, Place
from app.schemas.place import (
    AssignPhotosRequest,
    MergePlaceRequest,
    PlaceCreate,
    PlaceDetailOut,
    PlaceOut,
    PlaceUpdate,
)

router = APIRouter(prefix="/places", tags=["places"])


async def _counts(db: DbSession, user_id: uuid.UUID) -> tuple[dict[uuid.UUID, int], dict[uuid.UUID, int]]:
    photos = dict(
        (
            await db.execute(
                select(Photo.place_id, func.count())
                .where(Photo.user_id == user_id, Photo.place_id.is_not(None))
                .group_by(Photo.place_id)
            )
        ).all()  # type: ignore[arg-type]
    )
    notes = dict(
        (
            await db.execute(
                select(Note.place_id, func.count())
                .where(Note.user_id == user_id)
                .group_by(Note.place_id)
            )
        ).all()  # type: ignore[arg-type]
    )
    return photos, notes


async def _get_owned(db: DbSession, user_id: uuid.UUID, place_id: uuid.UUID) -> Place:
    place = await db.get(Place, place_id)
    if place is None or place.user_id != user_id:
        raise ApiError(404, "place_not_found", "地点不存在")
    return place


@router.get("", response_model=list[PlaceDetailOut], summary="列出地点（支持视口与关键字过滤）")
async def list_places(
    user: CurrentUser,
    db: DbSession,
    bbox: str | None = Query(default=None, description="minLng,minLat,maxLng,maxLat"),
    q: str | None = Query(default=None, max_length=100),
    limit: int = Query(default=1000, ge=1, le=5000),
) -> list[PlaceDetailOut]:
    stmt = select(Place).where(Place.user_id == user.id)
    bounds = parse_bbox(bbox)
    if bounds:
        min_lng, min_lat, max_lng, max_lat = bounds
        stmt = stmt.where(
            Place.lng >= min_lng, Place.lng <= max_lng, Place.lat >= min_lat, Place.lat <= max_lat
        )
    if q:
        pattern = f"%{q.strip()}%"
        stmt = stmt.where(Place.name.ilike(pattern) | Place.summary_md.ilike(pattern))
    stmt = stmt.order_by(Place.updated_at.desc()).limit(limit)

    places = (await db.execute(stmt)).scalars().all()
    photo_counts, note_counts = await _counts(db, user.id)
    return [
        PlaceDetailOut(
            **PlaceOut.model_validate(p).model_dump(),
            photo_count=photo_counts.get(p.id, 0),
            note_count=note_counts.get(p.id, 0),
        )
        for p in places
    ]


@router.post("", response_model=PlaceOut, status_code=status.HTTP_201_CREATED, summary="新建地点")
async def create_place(payload: PlaceCreate, user: CurrentUser, db: DbSession) -> Place:
    place = Place(
        user_id=user.id,
        name=payload.name.strip(),
        kind=payload.kind,
        lat=payload.lat,
        lng=payload.lng,
        address=payload.address,
        geocoded=bool(payload.address),
    )
    db.add(place)
    await db.commit()
    await db.refresh(place)
    return place


@router.get("/{place_id}", response_model=PlaceDetailOut, summary="地点详情")
async def get_place(place_id: uuid.UUID, user: CurrentUser, db: DbSession) -> PlaceDetailOut:
    place = await _get_owned(db, user.id, place_id)
    photo_count = await db.scalar(
        select(func.count()).select_from(Photo).where(Photo.place_id == place.id)
    )
    note_count = await db.scalar(select(func.count()).select_from(Note).where(Note.place_id == place.id))
    return PlaceDetailOut(
        **PlaceOut.model_validate(place).model_dump(),
        photo_count=photo_count or 0,
        note_count=note_count or 0,
    )


@router.patch("/{place_id}", response_model=PlaceOut, summary="更新地点（名称/笔记摘要/标签等）")
async def update_place(
    place_id: uuid.UUID, payload: PlaceUpdate, user: CurrentUser, db: DbSession
) -> Place:
    place = await _get_owned(db, user.id, place_id)
    data = payload.model_dump(exclude_unset=True)
    # 一旦人为改名，就认为已经确认过名称，不再需要自动地理编码
    if "name" in data and data["name"] is not None:
        place.geocoded = True
    if "address" in data and data["address"]:
        place.geocoded = True
    for field, value in data.items():
        setattr(place, field, value)
    await db.commit()
    await db.refresh(place)
    return place


@router.delete("/{place_id}", status_code=status.HTTP_204_NO_CONTENT, summary="删除地点")
async def delete_place(
    place_id: uuid.UUID,
    user: CurrentUser,
    db: DbSession,
    storage: Storage,
    delete_photos: bool = Query(default=False, description="同时删除该地点下的照片及其文件"),
) -> None:
    place = await _get_owned(db, user.id, place_id)

    if delete_photos:
        rows = (
            await db.execute(select(Photo).where(Photo.place_id == place.id))
        ).scalars().all()
        keys: list[str] = []
        for photo in rows:
            keys.extend(k for k in (photo.storage_key, photo.preview_key, photo.thumb_key) if k)
            await db.delete(photo)
        await storage.delete(keys)
    else:
        # 保留照片，仅解除归属（无 GPS 的照片会落到「未定位」托盘）
        await db.execute(update(Photo).where(Photo.place_id == place.id).values(place_id=None))

    await db.execute(delete(Note).where(Note.place_id == place.id))
    await db.delete(place)
    await db.commit()


@router.post("/{place_id}/assign", response_model=PlaceDetailOut, summary="把照片移入该地点")
async def assign_photos(
    place_id: uuid.UUID, payload: AssignPhotosRequest, user: CurrentUser, db: DbSession
) -> PlaceDetailOut:
    place = await _get_owned(db, user.id, place_id)
    rows = (
        await db.execute(
            select(Photo).where(Photo.user_id == user.id, Photo.id.in_(payload.photo_ids))
        )
    ).scalars().all()
    for photo in rows:
        photo.place_id = place.id
    await db.commit()
    await db.refresh(place)

    photo_count = await db.scalar(select(func.count()).select_from(Photo).where(Photo.place_id == place.id))
    note_count = await db.scalar(select(func.count()).select_from(Note).where(Note.place_id == place.id))
    return PlaceDetailOut(
        **PlaceOut.model_validate(place).model_dump(),
        photo_count=photo_count or 0,
        note_count=note_count or 0,
    )


@router.post("/{place_id}/merge-into", response_model=PlaceOut, summary="把该地点整体并入另一个地点")
async def merge_place(
    place_id: uuid.UUID, payload: MergePlaceRequest, user: CurrentUser, db: DbSession
) -> Place:
    if place_id == payload.target_id:
        raise ApiError(422, "merge_into_self", "不能合并到自身")
    source = await _get_owned(db, user.id, place_id)
    target = await _get_owned(db, user.id, payload.target_id)

    await db.execute(update(Photo).where(Photo.place_id == source.id).values(place_id=target.id))
    await db.execute(update(Note).where(Note.place_id == source.id).values(place_id=target.id))
    await db.delete(source)
    await db.commit()
    await db.refresh(target)
    return target
