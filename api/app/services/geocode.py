"""地名服务代理（可插拔）。

为什么要经后端转发：浏览器直连这些服务无法声明合规的 User-Agent，
且会遇到跨域问题。后端只做转发，**不落盘缓存**——缓存责任交给浏览器
（响应带 Cache-Control），这样服务端保持无状态。

目前支持两个提供方：
* photon（默认）：OSM 数据，公开实例无需 key，原生返回中文地名；
* nominatim：OSM 官方服务，使用条款要求每秒最多 1 次并声明 UA。

注意：在中国大陆网络下 nominatim 与 OSM 瓦片通常不可达，故默认用 photon。
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any, Protocol

import httpx

from app.core.config import Settings

logger = logging.getLogger(__name__)

# 各提供方的最小请求间隔（秒），纯粹为遵守对方的使用条款，不属于业务状态
_MIN_INTERVAL = {"photon": 0.15, "nominatim": 1.05}

_NAME_PRIORITY = (
    "tourism", "attraction", "historic", "leisure", "natural",
    "city", "town", "village", "municipality", "hamlet",
    "suburb", "neighbourhood", "quarter", "city_district",
    "county", "state_district", "state", "province", "country",
)

_CITY_KEYS = {"city", "town", "village", "municipality", "hamlet"}
_SPOT_KEYS = {"tourism", "attraction", "historic", "leisure", "natural", "amenity", "building"}


class GeocodeProvider(Protocol):
    async def reverse(self, client: httpx.AsyncClient, lat: float, lng: float) -> dict[str, Any]: ...

    async def search(
        self, client: httpx.AsyncClient, query: str, limit: int
    ) -> list[dict[str, Any]]: ...


def _classify_from_keys(*, osm_key: str = "", osm_value: str = "") -> str:
    if osm_key in {"tourism", "historic", "leisure", "natural", "amenity", "shop", "office"}:
        return "spot"
    if osm_key == "place" and osm_value in _CITY_KEYS:
        return "city"
    if osm_key == "boundary" and osm_value == "administrative":
        return "region"
    if osm_key in {"highway", "railway", "waterway"}:
        return "spot"
    return "other"


class PhotonProvider:
    """photon.komoot.io —— 无需 key，中文友好。"""

    async def reverse(self, client: httpx.AsyncClient, lat: float, lng: float) -> dict[str, Any]:
        resp = await client.get("/reverse", params={"lat": lat, "lon": lng})
        resp.raise_for_status()
        features = resp.json().get("features") or []
        props = dict(features[0].get("properties") or {}) if features else {}
        coords = (features[0].get("geometry") or {}).get("coordinates") if features else None
        if coords:
            lng, lat = float(coords[0]), float(coords[1])
        return _normalize_photon(props, lat, lng)

    async def search(
        self, client: httpx.AsyncClient, query: str, limit: int
    ) -> list[dict[str, Any]]:
        resp = await client.get("/api", params={"q": query, "limit": limit})
        resp.raise_for_status()
        out: list[dict[str, Any]] = []
        for feature in resp.json().get("features") or []:
            coords = (feature.get("geometry") or {}).get("coordinates") or []
            if len(coords) < 2:
                continue
            out.append(_normalize_photon(dict(feature.get("properties") or {}), float(coords[1]), float(coords[0])))
        return out


class NominatimProvider:
    """nominatim.openstreetmap.org —— 官方服务，注意使用条款。"""

    async def reverse(self, client: httpx.AsyncClient, lat: float, lng: float) -> dict[str, Any]:
        resp = await client.get(
            "/reverse",
            params={"format": "jsonv2", "lat": lat, "lon": lng, "zoom": 14, "addressdetails": 1},
        )
        resp.raise_for_status()
        data = resp.json()
        address = data.get("address") or {}
        short = pick_short_name(address)
        country = address.get("country", "")
        return {
            "lat": lat,
            "lng": lng,
            "name": f"{short} · {country}" if short and country and country not in short else (short or country),
            "short_name": short,
            "display_name": data.get("display_name") or "",
            "kind": classify(address),
            "country": country,
            "address": address,
        }

    async def search(
        self, client: httpx.AsyncClient, query: str, limit: int
    ) -> list[dict[str, Any]]:
        resp = await client.get(
            "/search",
            params={"format": "jsonv2", "q": query, "limit": limit, "addressdetails": 1},
        )
        resp.raise_for_status()
        results = []
        for item in resp.json():
            address = item.get("address") or {}
            results.append(
                {
                    "lat": float(item["lat"]),
                    "lng": float(item["lon"]),
                    "name": item.get("name") or pick_short_name(address) or item.get("display_name", ""),
                    "display_name": item.get("display_name") or "",
                    "kind": classify(address),
                    "category": item.get("category", ""),
                    "type": item.get("type", ""),
                }
            )
        return results


def pick_short_name(address: dict[str, Any]) -> str:
    for key in _NAME_PRIORITY:
        value = address.get(key)
        if value:
            return str(value)
    return ""


def classify(address: dict[str, Any]) -> str:
    if any(address.get(k) for k in _SPOT_KEYS):
        return "spot"
    if any(address.get(k) for k in _CITY_KEYS):
        return "city"
    if address.get("state") or address.get("country"):
        return "region"
    return "other"


def _normalize_photon(props: dict[str, Any], lat: float, lng: float) -> dict[str, Any]:
    name = str(props.get("name") or "")
    city = str(props.get("city") or "")
    district = str(props.get("district") or "")
    locality = str(props.get("locality") or "")
    state = str(props.get("state") or "")
    country = str(props.get("country") or "")

    # 简称优先取要素本身的名字，其次逐级放大到城市
    short = name or city or district or locality or state or country

    # 完整地名：逐级拼接并去重，便于在详情面板里展示层级
    parts: list[str] = []
    for value in (name, locality, district, city, state, country):
        if value and value not in parts:
            parts.append(value)

    # 荒野等无人区可能没有任何名称，退化为坐标，保证前端总有东西可显示
    if not short:
        short = f"{lat:.4f}, {lng:.4f}"

    return {
        "lat": lat,
        "lng": lng,
        "name": f"{short} · {country}" if country and country not in short else short,
        "short_name": short,
        "display_name": ", ".join(parts) or short,
        "kind": _classify_from_keys(
            osm_key=str(props.get("osm_key") or ""), osm_value=str(props.get("osm_value") or "")
        ),
        "country": country,
        # 搜索结果模型需要这两个字段
        "category": str(props.get("osm_key") or ""),
        "type": str(props.get("osm_value") or props.get("type") or ""),
        "address": {
            "name": name,
            "locality": locality,
            "district": district,
            "city": city,
            "state": state,
            "country": country,
            "postcode": str(props.get("postcode") or ""),
            "street": str(props.get("street") or ""),
            "osm_type": str(props.get("osm_type") or ""),
            "osm_value": str(props.get("osm_value") or ""),
        },
    }


class Geocoder:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._provider_name = settings.geocode_provider
        self._base_url = (
            settings.photon_base_url if self._provider_name == "photon" else settings.nominatim_base_url
        ).rstrip("/")
        self._provider: PhotonProvider | NominatimProvider = (
            PhotonProvider() if self._provider_name == "photon" else NominatimProvider()
        )
        headers = {
            "User-Agent": settings.nominatim_user_agent,
            "Accept": "application/json",
        }
        # 注意：Photon 一旦收到 Accept-Language 就会切换到翻译后的英文名，
        # 而我们要的正是「本地语言地名」（中文/日文）。所以只给 Nominatim 发这个头。
        if self._provider_name == "nominatim":
            headers["Accept-Language"] = settings.nominatim_language

        self._client = httpx.AsyncClient(
            base_url=self._base_url,
            headers=headers,
            timeout=settings.geocode_timeout_seconds,
        )
        self._interval = _MIN_INTERVAL.get(self._provider_name, 0.5)
        self._lock = asyncio.Lock()
        self._last_call = 0.0

    async def _throttle(self) -> None:
        async with self._lock:
            loop = asyncio.get_running_loop()
            wait = self._interval - (loop.time() - self._last_call)
            if wait > 0:
                await asyncio.sleep(wait)
            self._last_call = loop.time()

    async def reverse(self, lat: float, lng: float) -> dict[str, Any]:
        await self._throttle()
        return await self._provider.reverse(self._client, lat, lng)

    async def search(self, query: str, limit: int = 6) -> list[dict[str, Any]]:
        await self._throttle()
        return await self._provider.search(self._client, query, limit)

    async def aclose(self) -> None:
        await self._client.aclose()
