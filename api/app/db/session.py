"""数据库引擎与会话工厂。

注意：连接池属于基础设施，不是业务状态——进程本身仍是无状态的，
任意实例都能处理任意请求。
"""

from __future__ import annotations

from collections.abc import AsyncIterator

from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase

from app.core.config import get_settings


class Base(DeclarativeBase):
    """所有 ORM 模型的基类。"""


def create_engine() -> AsyncEngine:
    settings = get_settings()
    return create_async_engine(
        settings.database_url,
        echo=settings.db_echo,
        pool_pre_ping=True,
        future=True,
    )


def create_session_factory(engine: AsyncEngine) -> async_sessionmaker[AsyncSession]:
    return async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)


async def get_session(request_session_factory: async_sessionmaker[AsyncSession]) -> AsyncIterator[AsyncSession]:
    """在依赖中使用：见 app/api/deps.py"""
    async with request_session_factory() as session:
        yield session
