"""Redis 客户端管理。

Redis 用于存放会话（session），属于共享存储而非进程内状态，
因此应用实例可以随时重启或水平扩展。
"""

from __future__ import annotations

from redis.asyncio import Redis, from_url

from app.core.config import get_settings


def create_redis() -> Redis:
    settings = get_settings()
    return from_url(settings.redis_url, encoding="utf-8", decode_responses=True)
