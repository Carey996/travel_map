"""旅行笔记的请求/响应模型。"""

from __future__ import annotations

import uuid
from datetime import date, datetime

from pydantic import BaseModel, ConfigDict, Field


class NoteCreate(BaseModel):
    place_id: uuid.UUID
    title: str = Field(default="", max_length=200)
    body_md: str = ""
    happened_on: date | None = None


class NoteUpdate(BaseModel):
    title: str | None = Field(default=None, max_length=200)
    body_md: str | None = None
    happened_on: date | None = None


class NoteOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID
    place_id: uuid.UUID
    title: str
    body_md: str
    happened_on: date | None
    created_at: datetime
    updated_at: datetime
