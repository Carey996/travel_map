"""旅行笔记 CRUD。"""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Query, status
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.errors import ApiError
from app.db.models import Note, Place
from app.schemas.note import NoteCreate, NoteOut, NoteUpdate

router = APIRouter(prefix="/notes", tags=["notes"])


async def _get_owned(db: DbSession, user_id: uuid.UUID, note_id: uuid.UUID) -> Note:
    note = await db.get(Note, note_id)
    if note is None or note.user_id != user_id:
        raise ApiError(404, "note_not_found", "笔记不存在")
    return note


@router.get("", response_model=list[NoteOut], summary="列出笔记")
async def list_notes(
    user: CurrentUser,
    db: DbSession,
    place_id: uuid.UUID | None = None,
    limit: int = Query(default=200, ge=1, le=1000),
) -> list[Note]:
    stmt = select(Note).where(Note.user_id == user.id)
    if place_id is not None:
        stmt = stmt.where(Note.place_id == place_id)
    stmt = stmt.order_by(Note.happened_on.desc().nulls_last(), Note.created_at.desc()).limit(limit)
    return list((await db.execute(stmt)).scalars().all())


@router.post("", response_model=NoteOut, status_code=status.HTTP_201_CREATED, summary="写一篇笔记")
async def create_note(payload: NoteCreate, user: CurrentUser, db: DbSession) -> Note:
    place = await db.get(Place, payload.place_id)
    if place is None or place.user_id != user.id:
        raise ApiError(404, "place_not_found", "地点不存在")
    note = Note(
        user_id=user.id,
        place_id=payload.place_id,
        title=payload.title.strip(),
        body_md=payload.body_md,
        happened_on=payload.happened_on,
    )
    db.add(note)
    await db.commit()
    await db.refresh(note)
    return note


@router.patch("/{note_id}", response_model=NoteOut, summary="更新笔记")
async def update_note(
    note_id: uuid.UUID, payload: NoteUpdate, user: CurrentUser, db: DbSession
) -> Note:
    note = await _get_owned(db, user.id, note_id)
    for field, value in payload.model_dump(exclude_unset=True).items():
        setattr(note, field, value)
    await db.commit()
    await db.refresh(note)
    return note


@router.delete("/{note_id}", status_code=status.HTTP_204_NO_CONTENT, summary="删除笔记")
async def delete_note(note_id: uuid.UUID, user: CurrentUser, db: DbSession) -> None:
    note = await _get_owned(db, user.id, note_id)
    await db.delete(note)
    await db.commit()
