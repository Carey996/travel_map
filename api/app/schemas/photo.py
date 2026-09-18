"""照片相关的请求/响应模型。"""

from __future__ import annotations

import uuid
from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field

from app.services.storage import StorageBackend


class UploadVariant(BaseModel):
    """一个派生版本（缩略图 / 预览图）的直传请求。"""

    content_type: str = Field(default="image/jpeg", max_length=120)
    suffix: str = Field(default="", max_length=16)


class UploadUrlRequest(BaseModel):
    filename: str = Field(max_length=400)
    content_type: str = Field(default="image/jpeg", max_length=120)
    content_hash: str = Field(min_length=8, max_length=64, description="原始文件 sha256")
    byte_size: int = Field(default=0, ge=0)
    preview: UploadVariant | None = None
    thumb: UploadVariant | None = None


class SignedUpload(BaseModel):
    key: str
    url: str
    headers: dict[str, str] = Field(default_factory=dict)


class UploadUrlResponse(BaseModel):
    # 若该 content_hash 已存在，则 duplicate 非空，客户端可跳过上传
    duplicate: "PhotoOut | None" = None
    original: SignedUpload
    preview: SignedUpload | None = None
    thumb: SignedUpload | None = None


class PhotoCreate(BaseModel):
    content_hash: str = Field(min_length=8, max_length=64)
    storage_key: str = Field(max_length=512)
    preview_key: str | None = Field(default=None, max_length=512)
    thumb_key: str | None = Field(default=None, max_length=512)

    original_name: str = Field(default="", max_length=400)
    mime: str = Field(default="image/jpeg", max_length=120)
    bytes: int = Field(default=0, ge=0)
    width: int | None = None
    height: int | None = None

    lat: float | None = Field(default=None, ge=-90, le=90)
    lng: float | None = Field(default=None, ge=-180, le=180)
    altitude: float | None = None
    taken_at: datetime | None = None

    camera_make: str = Field(default="", max_length=120)
    camera_model: str = Field(default="", max_length=120)
    lens: str = Field(default="", max_length=160)
    iso: int | None = None
    f_number: float | None = None
    exposure: str = Field(default="", max_length=40)
    focal_length: float | None = None
    orientation: int | None = None

    place_id: uuid.UUID | None = None

    model_config = ConfigDict(extra="ignore")


class PhotoUpdate(BaseModel):
    caption: str | None = None
    place_id: uuid.UUID | None = None
    clear_place: bool = False
    lat: float | None = Field(default=None, ge=-90, le=90)
    lng: float | None = Field(default=None, ge=-180, le=180)
    taken_at: datetime | None = None


class PhotoOut(BaseModel):
    id: uuid.UUID
    place_id: uuid.UUID | None
    caption: str
    original_name: str
    mime: str
    bytes: int
    width: int | None
    height: int | None
    lat: float | None
    lng: float | None
    altitude: float | None
    taken_at: datetime | None
    camera_make: str
    camera_model: str
    lens: str
    iso: int | None
    f_number: float | None
    exposure: str
    focal_length: float | None
    created_at: datetime
    # 预签名直链，前端直接用于 <img src>
    url: str
    preview_url: str
    thumb_url: str


def photo_to_out(photo, storage: StorageBackend) -> PhotoOut:
    """把 ORM 对象转成带预签名直链的响应。

    签名是纯本地计算（不发网络请求），因此每个请求现签即可，
    既不需要在库里存易过期的 URL，也不依赖任何缓存。
    """
    return PhotoOut(
        id=photo.id,
        place_id=photo.place_id,
        caption=photo.caption,
        original_name=photo.original_name,
        mime=photo.mime,
        bytes=photo.bytes,
        width=photo.width,
        height=photo.height,
        lat=photo.lat,
        lng=photo.lng,
        altitude=photo.altitude,
        taken_at=photo.taken_at,
        camera_make=photo.camera_make,
        camera_model=photo.camera_model,
        lens=photo.lens,
        iso=photo.iso,
        f_number=photo.f_number,
        exposure=photo.exposure,
        focal_length=photo.focal_length,
        created_at=photo.created_at,
        url=storage.presign_get(photo.storage_key),
        preview_url=storage.presign_get(photo.preview_key or photo.storage_key),
        thumb_url=storage.presign_get(photo.thumb_key or photo.preview_key or photo.storage_key),
    )


UploadUrlResponse.model_rebuild()
