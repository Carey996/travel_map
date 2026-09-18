"""对象存储适配层。

关键设计：后端**不接收图片字节流**。浏览器拿到预签名 URL 后直传对象存储，
因此 API 进程既不需要本地磁盘，也不会被大文件拖垮内存——
这是「无状态」在文件上传场景下的具体落法。

StorageBackend 是协议接口，生产用 S3 兼容存储（MinIO / R2 / AWS S3），
测试时可替换为内存实现。
"""

from __future__ import annotations

import asyncio
import logging
from typing import Protocol

import boto3
from botocore.client import Config as BotoConfig
from botocore.exceptions import ClientError

from app.core.config import Settings

logger = logging.getLogger(__name__)


class StorageBackend(Protocol):
    async def ensure_ready(self) -> None: ...

    def presign_put(self, key: str, content_type: str, expires_in: int | None = None) -> str: ...

    def presign_get(self, key: str, expires_in: int | None = None) -> str: ...

    async def delete(self, keys: list[str]) -> None: ...


def _client(settings: Settings, endpoint_url: str):
    return boto3.client(
        "s3",
        endpoint_url=endpoint_url,
        aws_access_key_id=settings.s3_access_key,
        aws_secret_access_key=settings.s3_secret_key,
        region_name=settings.s3_region,
        config=BotoConfig(
            signature_version="s3v4",
            # MinIO 与自建网关一般走 path-style
            s3={"addressing_style": "path"},
            retries={"max_attempts": 3, "mode": "standard"},
        ),
    )


class S3Storage:
    """基于 S3 协议的对象存储实现。"""

    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._bucket = settings.s3_bucket
        # admin 客户端走容器内网地址，用于建桶与删除
        self._admin = _client(settings, settings.s3_endpoint_url)
        # presign 客户端走浏览器可达地址 —— 签名是纯计算，不发网络请求
        self._presign = (
            self._admin
            if settings.s3_browser_endpoint == settings.s3_endpoint_url
            else _client(settings, settings.s3_browser_endpoint)
        )

    # ---- 生命周期 ----

    async def ensure_ready(self) -> None:
        await asyncio.to_thread(self._ensure_bucket_sync)

    def _ensure_bucket_sync(self) -> None:
        try:
            self._admin.head_bucket(Bucket=self._bucket)
            logger.info("对象存储 bucket 已存在：%s", self._bucket)
            return
        except ClientError as exc:
            code = exc.response.get("Error", {}).get("Code", "")
            if code not in {"404", "NoSuchBucket", "NotFound"}:
                raise

        self._admin.create_bucket(Bucket=self._bucket)
        logger.info("已创建对象存储 bucket：%s", self._bucket)

    # ---- 预签名 ----

    def presign_put(self, key: str, content_type: str, expires_in: int | None = None) -> str:
        return self._presign.generate_presigned_url(
            "put_object",
            Params={"Bucket": self._bucket, "Key": key, "ContentType": content_type},
            ExpiresIn=expires_in or self._settings.s3_presign_expires,
        )

    def presign_get(self, key: str, expires_in: int | None = None) -> str:
        return self._presign.generate_presigned_url(
            "get_object",
            Params={"Bucket": self._bucket, "Key": key},
            ExpiresIn=expires_in or self._settings.s3_presign_expires,
        )

    # ---- 删除 ----

    async def delete(self, keys: list[str]) -> None:
        keys = [k for k in keys if k]
        if not keys:
            return
        await asyncio.to_thread(self._delete_sync, keys)

    def _delete_sync(self, keys: list[str]) -> None:
        for start in range(0, len(keys), 1000):
            chunk = keys[start : start + 1000]
            self._admin.delete_objects(
                Bucket=self._bucket,
                Delete={"Objects": [{"Key": k} for k in chunk], "Quiet": True},
            )
