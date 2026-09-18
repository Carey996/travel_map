"""地理编码代理接口。

响应带 Cache-Control：缓存责任交给浏览器与 CDN，后端不保存任何结果。
"""

from __future__ import annotations

import httpx
from fastapi import APIRouter, Query, Request, Response

from app.api.deps import CurrentUser
from app.core.errors import ApiError
from app.schemas.geo import ReverseResult, SearchResult

router = APIRouter(prefix="/geo", tags=["geo"])

CACHE_SECONDS = 60 * 60 * 24


@router.get("/reverse", response_model=ReverseResult, summary="经纬度 -> 地名")
async def reverse(
    request: Request,
    response: Response,
    _user: CurrentUser,
    lat: float = Query(ge=-90, le=90),
    lng: float = Query(ge=-180, le=180),
) -> ReverseResult:
    geocoder = request.app.state.geocoder
    try:
        data = await geocoder.reverse(lat, lng)
    except httpx.HTTPError as exc:
        raise ApiError(502, "geocode_upstream_error", "反向地理编码服务暂时不可用") from exc
    response.headers["Cache-Control"] = f"private, max-age={CACHE_SECONDS}"
    return ReverseResult(**data)


@router.get("/search", response_model=list[SearchResult], summary="地名搜索")
async def search(
    request: Request,
    response: Response,
    _user: CurrentUser,
    q: str = Query(min_length=2, max_length=160),
    limit: int = Query(default=6, ge=1, le=20),
) -> list[SearchResult]:
    geocoder = request.app.state.geocoder
    try:
        items = await geocoder.search(q, limit=limit)
    except httpx.HTTPError as exc:
        raise ApiError(502, "geocode_upstream_error", "地名搜索服务暂时不可用") from exc
    response.headers["Cache-Control"] = f"private, max-age={CACHE_SECONDS}"
    return [SearchResult(**item) for item in items]
