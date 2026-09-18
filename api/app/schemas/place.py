"""地点相关的请求/响应模型。"""

from __future__ import annotations

import uuid
from datetime import date, datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

PlaceKind = Literal["city", "spot", "region", "other"]


class PlaceCreate(BaseModel):
    name: str = Field(default="", max_length=160)
    kind: PlaceKind = "spot"
    lat: float = Field(ge=-90, le=90)
    lng: float = Field(ge=-180, le=180)
    address: str = Field(default="", max_length=400)


class PlaceUpdate(BaseModel):
    name: str | None = Field(default=None, max_length=160)
    kind: PlaceKind | None = None
    lat: float | None = Field(default=None, ge=-90, le=90)
    lng: float | None = Field(default=None, ge=-180, le=180)
    summary_md: str | None = None
    tags: list[str] | None = None
    rating: int | None = Field(default=None, ge=0, le=5)
    date_from: date | None = None
    date_to: date | None = None
    trip_id: uuid.UUID | None = None


class PlaceOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    name: str
    kind: PlaceKind
    lat: float
    lng: float
    address: str
    geocoded: bool
    summary_md: str
    tags: list[str]
    rating: int
    date_from: date | None
    date_to: date | None
    trip_id: uuid.UUID | None
    created_at: datetime
    updated_at: datetime


class PlaceDetailOut(PlaceOut):
    photo_count: int = 0
    note_count: int = 0


class AssignPhotosRequest(BaseModel):
    photo_ids: list[uuid.UUID] = Field(min_length=1, max_length=1000)
    # 若为 true，则把来源地点中已空的记录一并清理
    prune_empty_places: bool = True


class MergePlaceRequest(BaseModel):
    target_id: uuid.UUID
