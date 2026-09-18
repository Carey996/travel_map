"""照片元数据的登记、查询与维护。

后端**不接收图片字节流**：客户端先取预签名 URL 直传对象存储，
再把 EXIF 解析结果与对象键提交到这里。
"""

from __future__ import annotations

import mimetypes
import uuid
from datetime import date, datetime, time
from pathlib import Path

from fastapi import APIRouter, Query, status
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession, Storage
from app.api.query import parse_bbox
from app.core.errors import ApiError
from app.db.models import Photo
from app.schemas.photo import (
    PhotoCreate,
    PhotoOut,
    PhotoUpdate,
    SignedUpload,
    UploadUrlRequest,
    UploadUrlResponse,
    photo_to_out,
)
from app.services.places import assign_photo

router = APIRouter(prefix="/photos", tags=["photos"])

_ALLOWED_EXT = {".jpg", ".jpeg", ".png", ".webp", ".gif", ".tif", ".tiff", ".heic", ".heif", ".avif", ".bmp"}


def _extension(filename: str, content_type: str) -> str:
    suffix = Path(filename).suffix.lower()
    if suffix in _ALLOWED_EXT:
        return ".jpg" if suffix == ".jpeg" else suffix
    guessed = mimetypes.guess_extension(content_type or "") or ".jpg"
    return ".jpg" if guessed == ".jpe" else guessed


def _signed(storage: Storage, key: str, content_type: str) -> SignedUpload:
    return SignedUpload(
        key=key,
        url=storage.presign_put(key, content_type),
        headers={"Content-Type": content_type},
    )


@router.post("/upload-url", response_model=UploadUrlResponse, summary="签发直传用的预签名 URL")
async def create_upload_url(
    payload: UploadUrlRequest, user: CurrentUser, db: DbSession, storage: Storage
) -> UploadUrlResponse:
    # 幂等：同一用户重复上传同一张图时，不浪费带宽，直接返回已有记录
    duplicate = await db.scalar(
        select(Photo).where(Photo.user_id == user.id, Photo.content_hash == payload.content_hash)
    )

    base = f"u/{user.id}/{payload.content_hash}"
    ext = _extension(payload.filename, payload.content_type)
    original_key = f"{base}{ext}"
    preview_key = f"{base}.preview.jpg" if payload.preview else None
    thumb_key = f"{base}.thumb.jpg" if payload.thumb else None

    return UploadUrlResponse(
        duplicate=photo_to_out(duplicate, storage) if duplicate else None,
        original=_signed(storage, original_key, payload.content_type),
        preview=(
            _signed(storage, preview_key, payload.preview.content_type)
            if preview_key and payload.preview
            else None
        ),
        thumb=(
            _signed(storage, thumb_key, payload.thumb.content_type)
            if thumb_key and payload.thumb
            else None
        ),
    )


@router.post("", response_model=PhotoOut, status_code=status.HTTP_201_CREATED, summary="登记照片元数据")
async def create_photo(
    payload: PhotoCreate, user: CurrentUser, db: DbSession, storage: Storage
) -> PhotoOut:
    existing = await db.scalar(
        select(Photo).where(Photo.user_id == user.id, Photo.content_hash == payload.content_hash)
    )
    if existing is not None:
        return photo_to_out(existing, storage)

    data = payload.model_dump(exclude={"place_id"})
    photo = Photo(user_id=user.id, place_id=payload.place_id, **data)
    db.add(photo)
    await db.flush()

    # 有 GPS 就自动归入最近地点（1.5 km 内），否则新建一个待命名的地点
    await assign_photo(db, user.id, photo)

    await db.commit()
    await db.refresh(photo)
    return photo_to_out(photo, storage)


@router.get("", response_model=list[PhotoOut], summary="查询照片")
async def list_photos(
    user: CurrentUser,
    db: DbSession,
    storage: Storage,
    place_id: uuid.UUID | None = None,
    unplaced: bool = Query(default=False, description="只返回没有归属地点的照片"),
    located: bool | None = Query(default=None, description="true=有 GPS，false=无 GPS"),
    bbox: str | None = Query(default=None, description="minLng,minLat,maxLng,maxLat"),
    date_from: date | None = None,
    date_to: date | None = None,
    q: str | None = Query(default=None, max_length=120),
    limit: int = Query(default=500, ge=1, le=2000),
    offset: int = Query(default=0, ge=0),
) -> list[PhotoOut]:
    stmt = select(Photo).where(Photo.user_id == user.id)
    if place_id is not None:
        stmt = stmt.where(Photo.place_id == place_id)
    if unplaced:
        stmt = stmt.where(Photo.place_id.is_(None))
    if located is True:
        stmt = stmt.where(Photo.lat.is_not(None))
    elif located is False:
        stmt = stmt.where(Photo.lat.is_(None))

    bounds = parse_bbox(bbox)
    if bounds:
        min_lng, min_lat, max_lng, max_lat = bounds
        stmt = stmt.where(
            Photo.lng.is_not(None),
            Photo.lng >= min_lng,
            Photo.lng <= max_lng,
            Photo.lat.is_not(None),
            Photo.lat >= min_lat,
            Photo.lat <= max_lat,
        )
    if date_from is not None:
        stmt = stmt.where(Photo.taken_at >= datetime.combine(date_from, time.min))
    if date_to is not None:
        stmt = stmt.where(Photo.taken_at <= datetime.combine(date_to, time.max))
    if q:
        stmt = stmt.where(Photo.caption.ilike(f"%{q.strip()}%") | Photo.original_name.ilike(f"%{q.strip()}%"))

    stmt = stmt.order_by(Photo.taken_at.desc().nulls_last(), Photo.created_at.desc())
    stmt = stmt.limit(limit).offset(offset)
    rows = (await db.execute(stmt)).scalars().all()
    return [photo_to_out(p, storage) for p in rows]


@router.patch("/{photo_id}", response_model=PhotoOut, summary="更新照片（说明/归属/坐标）")
async def update_photo(
    photo_id: uuid.UUID, payload: PhotoUpdate, user: CurrentUser, db: DbSession, storage: Storage
) -> PhotoOut:
    photo = await db.get(Photo, photo_id)
    if photo is None or photo.user_id != user.id:
        raise ApiError(404, "photo_not_found", "照片不存在")

    data = payload.model_dump(exclude_unset=True)
    if data.pop("clear_place", False):
        photo.place_id = None
    if "place_id" in data and data["place_id"] is not None:
        photo.place_id = data.pop("place_id")
    else:
        data.pop("place_id", None)

    for field, value in data.items():
        setattr(photo, field, value)

    # 手动修正坐标后，重新计算归属
    if ("lat" in data or "lng" in data) and photo.lat is not None and photo.lng is not None:
        await assign_photo(db, user.id, photo)

    await db.commit()
    await db.refresh(photo)
    return photo_to_out(photo, storage)


@router.delete("/{photo_id}", status_code=status.HTTP_204_NO_CONTENT, summary="删除照片")
async def delete_photo(
    photo_id: uuid.UUID, user: CurrentUser, db: DbSession, storage: Storage
) -> None:
    photo = await db.get(Photo, photo_id)
    if photo is None or photo.user_id != user.id:
        raise ApiError(404, "photo_not_found", "照片不存在")
    keys = [k for k in (photo.storage_key, photo.preview_key, photo.thumb_key) if k]
    await db.delete(photo)
    await db.commit()
    await storage.delete(keys)
