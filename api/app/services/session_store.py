"""会话存储：同一个接口，两种后端。

* RedisSessionStore（默认）：会话存 Redis，靠 TTL 过期，撤销即时生效，
  多实例天然共享 —— 服务进程本身不持有任何会话状态。
* DbSessionStore：会话回落 Postgres，适合零额外组件起步；
  代价是每个请求多一次查询。
"""

from __future__ import annotations

import uuid
from datetime import datetime, timedelta, timezone
from typing import Protocol

from redis.asyncio import Redis
from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.core.security import hash_token, new_session_token
from app.db.models import UserSession


class SessionStore(Protocol):
    async def create(
        self, user_id: uuid.UUID, *, user_agent: str = "", ip: str = ""
    ) -> tuple[str, int]: ...

    async def resolve(self, sid: str) -> uuid.UUID | None: ...

    async def revoke(self, sid: str) -> None: ...

    async def revoke_all_for_user(self, user_id: uuid.UUID) -> None: ...


class RedisSessionStore:
    def __init__(self, redis: Redis, ttl_seconds: int, prefix: str = "tb:sess:") -> None:
        self._redis = redis
        self._ttl = ttl_seconds
        self._prefix = prefix

    def _key(self, sid: str) -> str:
        return self._prefix + sid

    def _user_key(self, user_id: uuid.UUID | str) -> str:
        # 反向索引，用于「退出所有设备」
        return f"tb:user:{user_id}:sess"

    async def create(
        self, user_id: uuid.UUID, *, user_agent: str = "", ip: str = ""
    ) -> tuple[str, int]:
        sid = new_session_token()
        pipe = self._redis.pipeline()
        pipe.set(self._key(sid), str(user_id), ex=self._ttl)
        pipe.sadd(self._user_key(user_id), sid)
        pipe.expire(self._user_key(user_id), self._ttl)
        await pipe.execute()
        return sid, self._ttl

    async def resolve(self, sid: str) -> uuid.UUID | None:
        raw = await self._redis.get(self._key(sid))
        if not raw:
            return None
        try:
            return uuid.UUID(raw)
        except (ValueError, AttributeError):
            return None

    async def revoke(self, sid: str) -> None:
        raw = await self._redis.get(self._key(sid))
        pipe = self._redis.pipeline()
        pipe.delete(self._key(sid))
        if raw:
            pipe.srem(self._user_key(raw), sid)
        await pipe.execute()

    async def revoke_all_for_user(self, user_id: uuid.UUID) -> None:
        sids = await self._redis.smembers(self._user_key(user_id))
        keys = [self._key(s) for s in sids]
        if keys:
            await self._redis.delete(*keys)
        await self._redis.delete(self._user_key(user_id))


class DbSessionStore:
    def __init__(
        self, session_factory: async_sessionmaker[AsyncSession], ttl_seconds: int
    ) -> None:
        self._session_factory = session_factory
        self._ttl = ttl_seconds

    async def create(
        self, user_id: uuid.UUID, *, user_agent: str = "", ip: str = ""
    ) -> tuple[str, int]:
        sid = new_session_token()
        async with self._session_factory() as db:
            db.add(
                UserSession(
                    user_id=user_id,
                    token_hash=hash_token(sid),
                    user_agent=user_agent[:400],
                    ip=ip[:64],
                    expires_at=datetime.now(timezone.utc) + timedelta(seconds=self._ttl),
                )
            )
            await db.commit()
        return sid, self._ttl

    async def resolve(self, sid: str) -> uuid.UUID | None:
        async with self._session_factory() as db:
            row = await db.scalar(
                select(UserSession).where(
                    UserSession.token_hash == hash_token(sid),
                    UserSession.revoked_at.is_(None),
                    UserSession.expires_at > datetime.now(timezone.utc),
                )
            )
            return row.user_id if row else None

    async def revoke(self, sid: str) -> None:
        async with self._session_factory() as db:
            await db.execute(
                update(UserSession)
                .where(UserSession.token_hash == hash_token(sid))
                .values(revoked_at=datetime.now(timezone.utc))
            )
            await db.commit()

    async def revoke_all_for_user(self, user_id: uuid.UUID) -> None:
        async with self._session_factory() as db:
            await db.execute(
                update(UserSession)
                .where(UserSession.user_id == user_id, UserSession.revoked_at.is_(None))
                .values(revoked_at=datetime.now(timezone.utc))
            )
            await db.commit()


def build_session_store(settings, redis: Redis, session_factory) -> SessionStore:
    if settings.session_backend == "redis":
        return RedisSessionStore(redis, settings.session_ttl_seconds)
    return DbSessionStore(session_factory, settings.session_ttl_seconds)
