"""密码哈希与会话令牌。

会话令牌是**不透明随机串**而非 JWT：服务端不保存明文，
只保存 sha256 摘要（Redis 直接以令牌为键，Postgres 存摘要），
因此即使数据库泄露也无法直接冒用会话。
"""

from __future__ import annotations

import hashlib
import secrets

import bcrypt

# bcrypt 只取前 72 字节，超长密码必须先截断，否则底层会抛错
_BCRYPT_MAX_BYTES = 72


def _prepare(password: str) -> bytes:
    return password.encode("utf-8")[:_BCRYPT_MAX_BYTES]


def hash_password(password: str) -> str:
    return bcrypt.hashpw(_prepare(password), bcrypt.gensalt()).decode("ascii")


def verify_password(password: str, password_hash: str) -> bool:
    try:
        return bcrypt.checkpw(_prepare(password), password_hash.encode("ascii"))
    except (ValueError, TypeError):
        return False


def new_session_token() -> str:
    """256 位熵的不透明会话 id。"""
    return secrets.token_urlsafe(32)


def hash_token(token: str) -> str:
    return hashlib.sha256(token.encode("utf-8")).hexdigest()
