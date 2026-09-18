"""FastAPI 应用入口。

本服务刻意保持无状态：进程内不保存会话、不写本地磁盘、不做内存缓存。
所有持久化都落在外部共享存储（Postgres / Redis / S3），
因此任意实例都能处理任意请求，可以随时重启与水平扩展。
"""

from __future__ import annotations

import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.v1 import health
from app.api.v1.router import api_router
from app.core.config import get_settings
from app.core.errors import register_error_handlers
from app.core.middleware import CsrfHeaderMiddleware
from app.core.redis import create_redis
from app.db.session import create_engine, create_session_factory
from app.services.geocode import Geocoder
from app.services.session_store import build_session_store
from app.services.storage import S3Storage

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)-8s %(name)s | %(message)s",
)
logger = logging.getLogger("travelblog")


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings = get_settings()
    app.state.settings = settings

    engine = create_engine()
    app.state.engine = engine
    app.state.session_factory = create_session_factory(engine)
    app.state.redis = create_redis()
    app.state.storage = S3Storage(settings)
    app.state.sessions = build_session_store(settings, app.state.redis, app.state.session_factory)
    app.state.geocoder = Geocoder(settings)
    try:
        await app.state.storage.ensure_ready()
    except Exception:  # noqa: BLE001 - 对象存储暂时不可用不应阻止 API 启动
        logger.warning("对象存储暂不可用，跳过后台校验", exc_info=True)

    logger.info("启动 %s %s (env=%s)", settings.app_name, settings.app_version, settings.environment)
    try:
        yield
    finally:
        await app.state.geocoder.aclose()
        await app.state.redis.aclose()
        await engine.dispose()
        logger.info("已释放数据库与 Redis 连接")


def create_app() -> FastAPI:
    settings = get_settings()
    app = FastAPI(
        title=settings.app_name,
        version=settings.app_version,
        description="旅行地图博客的无状态后端 API",
        lifespan=lifespan,
        openapi_url="/openapi.json",
        docs_url="/docs",
        redoc_url=None,
    )

    # 中间件按「后添加者更靠外」生效：CORS 必须在 CSRF 之外，
    # 这样跨站请求在预检阶段就被拒绝，不会走到业务逻辑。
    app.add_middleware(
        CsrfHeaderMiddleware,
        header_name=settings.csrf_header_name,
        header_value=settings.csrf_header_value,
        exempt_paths=frozenset({"/healthz", "/healthz/ready"}),
    )
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # 探针挂在根路径：不属于版本化业务 API，便于容器与网关直接访问。
    app.include_router(health.router)
    register_error_handlers(app)
    app.include_router(api_router, prefix=settings.api_v1_prefix)
    return app


app = create_app()
