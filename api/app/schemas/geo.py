"""地理编码接口的响应模型。"""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel


class ReverseResult(BaseModel):
    lat: float
    lng: float
    name: str
    short_name: str
    display_name: str
    kind: str
    country: str
    address: dict[str, Any]


class SearchResult(BaseModel):
    lat: float
    lng: float
    name: str
    display_name: str
    kind: str
    category: str
    type: str
