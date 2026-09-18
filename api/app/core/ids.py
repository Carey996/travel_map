"""主键生成：UUID v7（时间有序）。

选 UUID v7 而不是自增整数，是为了让无状态服务可以各自本地生成主键，
无需向数据库申请序列，同时保持索引插入的时间局部性。
"""

from __future__ import annotations

import os
import time
import uuid

try:  # Python 3.14+
    from uuid import uuid7 as _stdlib_uuid7

    def new_id() -> uuid.UUID:
        return _stdlib_uuid7()

except ImportError:  # pragma: no cover - 仅为 3.13 及以下提供等价实现

    def new_id() -> uuid.UUID:
        """按 RFC 9562 生成 UUID v7。"""
        unix_ms = int(time.time() * 1000) & ((1 << 48) - 1)
        rand_a = int.from_bytes(os.urandom(2), "big") & 0x0FFF
        rand_b = int.from_bytes(os.urandom(8), "big") & ((1 << 62) - 1)
        value = (unix_ms << 80) | (0x7 << 76) | (rand_a << 64) | (0b10 << 62) | rand_b
        return uuid.UUID(int=value)
