"""注册 / 登录 / 登出。

会话令牌放在 HttpOnly Cookie 中，前端 JS 读不到，能有效抵御 XSS 窃取。
代价是引入 CSRF 面，由 CsrfHeaderMiddleware 与严格的 CORS 白名单共同兜住。
"""

from __future__ import annotations

from fastapi import APIRouter, Request, Response, status
from sqlalchemy import select

from app.api.deps import AppSettings, CurrentUser, DbSession, Sessions
from app.core.errors import ApiError
from app.core.security import hash_password, verify_password
from app.db.models import User
from app.schemas.auth import LoginRequest, RegisterRequest, UserOut

router = APIRouter(prefix="/auth", tags=["auth"])


def _set_session_cookie(response: Response, settings, sid: str, ttl: int) -> None:
    response.set_cookie(
        key=settings.session_cookie_name,
        value=sid,
        max_age=ttl,
        httponly=True,
        secure=settings.cookie_secure,
        samesite=settings.cookie_samesite,
        path="/",
    )


def _client_ip(request: Request) -> str:
    return request.client.host if request.client else ""


@router.post("/register", response_model=UserOut, status_code=status.HTTP_201_CREATED, summary="注册并自动登录")
async def register(
    payload: RegisterRequest,
    request: Request,
    response: Response,
    db: DbSession,
    sessions: Sessions,
    settings: AppSettings,
) -> User:
    email = payload.email.lower()
    if await db.scalar(select(User.id).where(User.email == email)):
        raise ApiError(status.HTTP_409_CONFLICT, "email_taken", "该邮箱已被注册")

    user = User(
        email=email,
        display_name=payload.display_name.strip() or email.split("@")[0],
        password_hash=hash_password(payload.password),
    )
    db.add(user)
    await db.commit()
    await db.refresh(user)

    sid, ttl = await sessions.create(
        user.id,
        user_agent=request.headers.get("user-agent", ""),
        ip=_client_ip(request),
    )
    _set_session_cookie(response, settings, sid, ttl)
    return user


@router.post("/login", response_model=UserOut, summary="登录")
async def login(
    payload: LoginRequest,
    request: Request,
    response: Response,
    db: DbSession,
    sessions: Sessions,
    settings: AppSettings,
) -> User:
    user = await db.scalar(select(User).where(User.email == payload.email.lower()))
    # 无论邮箱是否存在都做一次密码校验，避免通过响应时间枚举账号
    if user is None or not verify_password(payload.password, user.password_hash):
        raise ApiError(status.HTTP_401_UNAUTHORIZED, "invalid_credentials", "邮箱或密码不正确")
    if not user.is_active:
        raise ApiError(status.HTTP_403_FORBIDDEN, "account_inactive", "账号已被停用")

    sid, ttl = await sessions.create(
        user.id,
        user_agent=request.headers.get("user-agent", ""),
        ip=_client_ip(request),
    )
    _set_session_cookie(response, settings, sid, ttl)
    return user


@router.post("/logout", status_code=status.HTTP_204_NO_CONTENT, summary="登出当前设备")
async def logout(
    request: Request,
    response: Response,
    sessions: Sessions,
    settings: AppSettings,
) -> Response:
    sid = request.cookies.get(settings.session_cookie_name)
    if sid:
        await sessions.revoke(sid)
    response.delete_cookie(
        key=settings.session_cookie_name,
        path="/",
        httponly=True,
        secure=settings.cookie_secure,
        samesite=settings.cookie_samesite,
    )
    response.status_code = status.HTTP_204_NO_CONTENT
    return response


@router.post("/logout-all", status_code=status.HTTP_204_NO_CONTENT, summary="登出所有设备")
async def logout_all(
    request: Request,
    response: Response,
    user: CurrentUser,
    sessions: Sessions,
    settings: AppSettings,
) -> Response:
    await sessions.revoke_all_for_user(user.id)
    response.delete_cookie(key=settings.session_cookie_name, path="/")
    response.status_code = status.HTTP_204_NO_CONTENT
    return response


@router.get("/me", response_model=UserOut, summary="当前登录用户")
async def me(user: CurrentUser) -> User:
    return user
