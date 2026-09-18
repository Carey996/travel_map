"""跨域与会话 Cookie 相关的安全中间件。

选用 Cookie 承载会话（而非 Authorization 头）就必须配套 CSRF 防护。
这里的策略是三重的：
  1. Cookie 设置 SameSite=Lax，跨站发起的表单 POST 不会带上会话 Cookie；
  2. CORS 严格白名单，跨站 fetch 携带自定义头会触发预检并被拒绝；
  3. 所有变更类请求必须带 X-Requested-With 头——HTML 表单无法设置自定义头。
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable

from fastapi import Request, Response, status
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

SAFE_METHODS = frozenset({"GET", "HEAD", "OPTIONS", "TRACE"})


class CsrfHeaderMiddleware(BaseHTTPMiddleware):
    """要求变更类请求携带约定的自定义头。"""

    def __init__(self, app, *, header_name: str, header_value: str, exempt_paths: frozenset[str] = frozenset()):
        super().__init__(app)
        self.header_name = header_name.lower()
        self.header_value = header_value
        self.exempt_paths = exempt_paths

    async def dispatch(
        self,
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        if request.method.upper() in SAFE_METHODS or request.url.path in self.exempt_paths:
            return await call_next(request)

        provided = request.headers.get(self.header_name)
        if provided != self.header_value:
            return JSONResponse(
                status_code=status.HTTP_403_FORBIDDEN,
                content={
                    "code": "csrf_header_missing",
                    "message": (
                        "变更类请求必须携带头部 "
                        f"{self.header_name}: {self.header_value}"
                    ),
                    "detail": {"path": request.url.path, "method": request.method},
                },
            )
        return await call_next(request)
