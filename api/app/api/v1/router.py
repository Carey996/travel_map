"""v1 版本 API 汇总路由。"""

from __future__ import annotations

from fastapi import APIRouter

from app.api.v1 import auth, geo, notes, photos, places

# 后续业务模块（shares / public）在此挂载。
api_router = APIRouter()
api_router.include_router(auth.router)
api_router.include_router(places.router)
api_router.include_router(photos.router)
api_router.include_router(notes.router)
api_router.include_router(geo.router)
