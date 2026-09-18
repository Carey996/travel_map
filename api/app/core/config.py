"""应用配置：全部来自环境变量，进程内不持有业务状态。"""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from typing import Annotated, Literal

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, NoDecode, SettingsConfigDict

# api/app/core/config.py -> core -> app -> api -> 项目根
_PROJECT_ROOT = Path(__file__).resolve().parents[3]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        # 使用绝对路径，避免相对 CWD 的写法在不同启动方式下失效
        env_file=(_PROJECT_ROOT / ".env", _PROJECT_ROOT / ".env.local"),
        env_file_encoding="utf-8",
        extra="ignore",
        case_sensitive=False,
    )

    # ---- 基础 ----
    app_name: str = "Travel Blog API"
    app_version: str = "0.1.0"
    environment: Literal["dev", "prod", "test"] = "dev"
    api_v1_prefix: str = "/api/v1"

    # ---- 数据库 ----
    database_url: str = "postgresql+asyncpg://travelblog:travelblog@localhost:5432/travelblog"
    db_echo: bool = False

    # ---- Redis / 会话 ----
    redis_url: str = "redis://localhost:6379/0"
    session_backend: Literal["redis", "db"] = "redis"

    # ---- 会话 Cookie（CSRF 关键配置，改动需同步前端）----
    session_cookie_name: str = "tb_session"
    session_ttl_seconds: int = 60 * 60 * 24 * 30
    cookie_secure: bool = False
    cookie_samesite: Literal["lax", "strict", "none"] = "lax"
    # 变更类请求必须携带该头，配合 CORS 白名单抵御 CSRF
    csrf_header_name: str = "x-requested-with"
    csrf_header_value: str = "XMLHttpRequest"

    # ---- CORS ----
    # NoDecode：跳过 pydantic-settings 对复杂类型的 JSON 预解析，
    # 从而支持 CORS_ORIGINS=a,b,c 这种更顺手的逗号分隔写法。
    cors_origins: Annotated[list[str], NoDecode] = Field(
        default_factory=lambda: ["http://localhost:5173", "http://127.0.0.1:5173"]
    )

    # ---- 对象存储 ----
    s3_endpoint_url: str = "http://localhost:9000"
    s3_public_endpoint_url: str | None = None
    s3_access_key: str = "minioadmin"
    s3_secret_key: str = "minioadmin"
    s3_bucket: str = "travelblog"
    s3_region: str = "us-east-1"
    s3_presign_expires: int = 3600

    # ---- 地理编码 ----
    # photon: OSM 数据、免 key、中文友好（默认）
    # nominatim: OSM 官方，国内网络通常不可达
    geocode_provider: Literal["photon", "nominatim"] = "photon"
    photon_base_url: str = "https://photon.komoot.io"
    nominatim_base_url: str = "https://nominatim.openstreetmap.org"
    nominatim_user_agent: str = "travel-blog/0.1 (personal project)"
    nominatim_language: str = "zh-CN,zh,en"
    geocode_timeout_seconds: float = 8.0

    @field_validator("cors_origins", mode="before")
    @classmethod
    def _split_origins(cls, value: object) -> object:
        """允许 CORS_ORIGINS=a,b,c 这种逗号分隔写法。"""
        if isinstance(value, str):
            return [item.strip() for item in value.split(",") if item.strip()]
        return value

    @property
    def s3_browser_endpoint(self) -> str:
        """浏览器直传使用的 endpoint（部署时可能是公网地址）。"""
        return self.s3_public_endpoint_url or self.s3_endpoint_url

    @property
    def is_dev(self) -> bool:
        return self.environment == "dev"


@lru_cache
def get_settings() -> Settings:
    return Settings()
