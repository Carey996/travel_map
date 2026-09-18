"""健康检查：存活探针与就绪探针。

存活探针不接触依赖，用于判断进程是否需要重启；
就绪探针检查 Postgres 与 Redis 连通性，用于判断能否开始接流量。
"""

from __future__ import annotations

from fastapi import APIRouter, Request, Response, status
from sqlalchemy import text

router = APIRouter(tags=["health"])


@router.get("/healthz", summary="存活探针")
async def liveness(request: Request) -> dict[str, str]:
    settings = request.app.state.settings
    return {
        "status": "ok",
        "app": settings.app_name,
        "version": settings.app_version,
        "environment": settings.environment,
    }


@router.get("/healthz/ready", summary="就绪探针：检查依赖连通性")
async def readiness(request: Request, response: Response) -> dict[str, object]:
    checks: dict[str, str] = {}

    try:
        async with request.app.state.engine.connect() as conn:
            await conn.execute(text("SELECT 1"))
        checks["database"] = "ok"
    except Exception as exc:  # noqa: BLE001 - 探针需要把任何异常转成状态字符串
        checks["database"] = "error: " + type(exc).__name__ + ": " + str(exc)

    try:
        await request.app.state.redis.ping()
        checks["redis"] = "ok"
    except Exception as exc:  # noqa: BLE001
        checks["redis"] = "error: " + type(exc).__name__ + ": " + str(exc)

    try:
        await request.app.state.storage.ensure_ready()
        checks["storage"] = "ok"
    except Exception as exc:  # noqa: BLE001
        checks["storage"] = "error: " + type(exc).__name__ + ": " + str(exc)

    healthy = all(value == "ok" for value in checks.values())
    if not healthy:
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
    return {"status": "ok" if healthy else "degraded", "checks": checks}
