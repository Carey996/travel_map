"""ORM 模型。

设计要点：
* 主键统一使用 UUID v7，服务端本地生成，便于无状态水平扩展；
* 所有表都带 created_at / updated_at，便于排序与增量同步；
* photos.content_hash 唯一索引，使上传天然幂等；
* 「旅行笔记」是独立表而非 places 上的字段，因为一个地点会写多篇。
"""

from __future__ import annotations

import uuid
from datetime import date, datetime, timezone

from sqlalchemy import (
    Boolean,
    CheckConstraint,
    Date,
    DateTime,
    Float,
    ForeignKey,
    Index,
    Integer,
    String,
    Text,
    UniqueConstraint,
    Uuid,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship
from sqlalchemy.types import JSON

from app.core.ids import new_id
from app.db.session import Base


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class TimestampMixin:
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_utcnow, nullable=False)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_utcnow, onupdate=_utcnow, nullable=False
    )


class User(TimestampMixin, Base):
    __tablename__ = "users"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    email: Mapped[str] = mapped_column(String(320), unique=True, index=True, nullable=False)
    display_name: Mapped[str] = mapped_column(String(80), default="", nullable=False)
    password_hash: Mapped[str] = mapped_column(String(255), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)

    places: Mapped[list[Place]] = relationship(back_populates="user", cascade="all, delete-orphan")


class Place(TimestampMixin, Base):
    """地点：地图上的一个大头针，可以是一座城市，也可以是某个具体景点。"""

    __tablename__ = "places"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )
    trip_id: Mapped[uuid.UUID | None] = mapped_column(
        ForeignKey("trips.id", ondelete="SET NULL"), index=True, nullable=True
    )

    name: Mapped[str] = mapped_column(String(160), default="", nullable=False)
    kind: Mapped[str] = mapped_column(String(16), default="spot", nullable=False)  # city | spot | region | other
    lat: Mapped[float] = mapped_column(Float, nullable=False)
    lng: Mapped[float] = mapped_column(Float, nullable=False)

    summary_md: Mapped[str] = mapped_column(Text, default="", nullable=False)
    tags: Mapped[list[str]] = mapped_column(JSON, default=list, nullable=False)
    rating: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    date_from: Mapped[date | None] = mapped_column(Date, nullable=True)
    date_to: Mapped[date | None] = mapped_column(Date, nullable=True)

    # 反向地理编码的原始结果，便于后续重新取名或排查
    address: Mapped[str] = mapped_column(String(400), default="", nullable=False)
    geocoded: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)

    user: Mapped[User] = relationship(back_populates="places")
    photos: Mapped[list[Photo]] = relationship(back_populates="place")
    notes: Mapped[list[Note]] = relationship(back_populates="place", cascade="all, delete-orphan")

    __table_args__ = (
        CheckConstraint("kind in ('city','spot','region','other')", name="ck_places_kind"),
        Index("ix_places_user_created", "user_id", "created_at"),
    )


class Photo(TimestampMixin, Base):
    """照片：二进制存对象存储，这里只保存元数据与键。"""

    __tablename__ = "photos"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )
    place_id: Mapped[uuid.UUID | None] = mapped_column(
        ForeignKey("places.id", ondelete="SET NULL"), index=True, nullable=True
    )

    # 对象存储中的键（不含 endpoint 与 bucket）
    storage_key: Mapped[str] = mapped_column(String(512), nullable=False)
    preview_key: Mapped[str | None] = mapped_column(String(512), nullable=True)
    thumb_key: Mapped[str | None] = mapped_column(String(512), nullable=True)

    original_name: Mapped[str] = mapped_column(String(400), default="", nullable=False)
    mime: Mapped[str] = mapped_column(String(120), default="image/jpeg", nullable=False)
    bytes: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    width: Mapped[int | None] = mapped_column(Integer, nullable=True)
    height: Mapped[int | None] = mapped_column(Integer, nullable=True)

    # EXIF 解析结果（由前端解析后提交）
    lat: Mapped[float | None] = mapped_column(Float, nullable=True)
    lng: Mapped[float | None] = mapped_column(Float, nullable=True)
    altitude: Mapped[float | None] = mapped_column(Float, nullable=True)
    taken_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), index=True, nullable=True)

    camera_make: Mapped[str] = mapped_column(String(120), default="", nullable=False)
    camera_model: Mapped[str] = mapped_column(String(120), default="", nullable=False)
    lens: Mapped[str] = mapped_column(String(160), default="", nullable=False)
    iso: Mapped[int | None] = mapped_column(Integer, nullable=True)
    f_number: Mapped[float | None] = mapped_column(Float, nullable=True)
    exposure: Mapped[str] = mapped_column(String(40), default="", nullable=False)
    focal_length: Mapped[float | None] = mapped_column(Float, nullable=True)
    orientation: Mapped[int | None] = mapped_column(Integer, nullable=True)

    caption: Mapped[str] = mapped_column(Text, default="", nullable=False)
    # 注意是「每个用户唯一」而非全局唯一：两个用户拍到同一张图不该互相冲突
    content_hash: Mapped[str] = mapped_column(String(64), index=True, nullable=False)

    user: Mapped[User] = relationship()
    place: Mapped[Place | None] = relationship(back_populates="photos")

    __table_args__ = (
        UniqueConstraint("user_id", "content_hash", name="uq_photos_user_content"),
        Index("ix_photos_user_taken", "user_id", "taken_at"),
        Index("ix_photos_user_place", "user_id", "place_id"),
    )


class Note(TimestampMixin, Base):
    """旅行笔记：挂在某个地点下，可以按天写多篇。"""

    __tablename__ = "notes"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )
    place_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("places.id", ondelete="CASCADE"), index=True, nullable=False
    )

    title: Mapped[str] = mapped_column(String(200), default="", nullable=False)
    body_md: Mapped[str] = mapped_column(Text, default="", nullable=False)
    happened_on: Mapped[date | None] = mapped_column(Date, index=True, nullable=True)

    place: Mapped[Place] = relationship(back_populates="notes")

    __table_args__ = (Index("ix_notes_place_happened", "place_id", "happened_on"),)


class Trip(TimestampMixin, Base):
    """行程：把多个地点串成一次旅行（M7 才在界面上启用，表结构先备好）。"""

    __tablename__ = "trips"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )

    title: Mapped[str] = mapped_column(String(200), default="", nullable=False)
    slug: Mapped[str] = mapped_column(String(200), unique=True, index=True, nullable=False)
    summary_md: Mapped[str] = mapped_column(Text, default="", nullable=False)
    date_from: Mapped[date | None] = mapped_column(Date, nullable=True)
    date_to: Mapped[date | None] = mapped_column(Date, nullable=True)

    places: Mapped[list[TripPlace]] = relationship(
        back_populates="trip", cascade="all, delete-orphan", order_by="TripPlace.position"
    )


class TripPlace(Base):
    """行程与地点的连接表，带顺序与第几天。"""

    __tablename__ = "trip_places"

    trip_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("trips.id", ondelete="CASCADE"), primary_key=True
    )
    place_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("places.id", ondelete="CASCADE"), primary_key=True
    )
    position: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    day_index: Mapped[int | None] = mapped_column(Integer, nullable=True)

    trip: Mapped[Trip] = relationship(back_populates="places")


class Share(TimestampMixin, Base):
    """公开只读分享链接。"""

    __tablename__ = "shares"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )
    slug: Mapped[str] = mapped_column(String(120), unique=True, index=True, nullable=False)
    target_type: Mapped[str] = mapped_column(String(16), nullable=False)  # place | trip
    target_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)

    title: Mapped[str] = mapped_column(String(200), default="", nullable=False)
    is_public: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
    expires_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    view_count: Mapped[int] = mapped_column(Integer, default=0, nullable=False)

    __table_args__ = (
        CheckConstraint("target_type in ('place','trip')", name="ck_shares_target_type"),
    )


class UserSession(Base):
    """会话记录（SESSION_BACKEND=db 时使用；默认走 Redis）。"""

    __tablename__ = "user_sessions"

    id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True, default=new_id)
    user_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), index=True, nullable=False
    )
    token_hash: Mapped[str] = mapped_column(String(64), unique=True, index=True, nullable=False)
    user_agent: Mapped[str] = mapped_column(String(400), default="", nullable=False)
    ip: Mapped[str] = mapped_column(String(64), default="", nullable=False)
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), index=True, nullable=False)
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_utcnow, nullable=False)
