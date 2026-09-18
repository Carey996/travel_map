"""FastAPI 依赖注入。

所有依赖都从 request.app.state 取共享资源——它们由 lifespan 创建，
属于进程级基础设施（连接池、客户端），而不是业务状态。
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Request
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import Settings, get_settings
from app.core.errors import ApiError
from app.db.models import User
from app.services.session_store import SessionStore
from app.services.storage import StorageBackend


async def get_db(request: Request) -> AsyncIterator[AsyncSession]:
    async with request.app.state.session_factory() as session:
        yield session


def get_app_settings() -> Settings:
    return get_settings()


def get_storage(request: Request) -> StorageBackend:
    return request.app.state.storage


def get_session_store(request: Request) -> SessionStore:
    return request.app.state.sessions


def read_session_id(request: Request) -> str | None:
    return request.cookies.get(get_settings().session_cookie_name)


async def get_current_user(
    request: Request,
    db: Annotated[AsyncSession, Depends(get_db)],
) -> User:
    sid = read_session_id(request)
    if not sid:
        raise ApiError(401, "not_authenticated", "请先登录")

    user_id = await request.app.state.sessions.resolve(sid)
    if user_id is None:
        raise ApiError(401, "session_expired", "登录已失效，请重新登录")

    user = await db.get(User, user_id)
    if user is None or not user.is_active:
        raise ApiError(401, "account_inactive", "账号不可用")
    return user


async def get_optional_user(
    request: Request,
    db: Annotated[AsyncSession, Depends(get_db)],
) -> User | None:
    sid = read_session_id(request)
    if not sid:
        return None
    user_id = await request.app.state.sessions.resolve(sid)
    if user_id is None:
        return None
    user = await db.get(User, user_id)
    return user if user and user.is_active else None


DbSession = Annotated[AsyncSession, Depends(get_db)]
CurrentUser = Annotated[User, Depends(get_current_user)]
OptionalUser = Annotated[User | None, Depends(get_optional_user)]
Storage = Annotated[StorageBackend, Depends(get_storage)]
Sessions = Annotated[SessionStore, Depends(get_session_store)]
AppSettings = Annotated[Settings, Depends(get_app_settings)]
