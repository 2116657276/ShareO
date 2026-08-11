"""Small async PostgreSQL connection wrapper used by the vector stores."""

from collections.abc import AsyncIterator, Sequence
from contextlib import asynccontextmanager
from typing import Any

from pgvector.psycopg import register_vector_async
from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool


async def _configure_connection(connection: Any) -> None:
    """Register pgvector's adapter on every pooled connection."""

    await register_vector_async(connection)


class PostgresVectorDatabase:
    """A deliberately small database boundary for the image/text stores.

    The AI role is granted access only to the ``ai`` schema.  DDL is therefore
    kept in the checked-in PostgreSQL migrations and this class only executes
    reads and idempotent writes.
    """

    def __init__(self, dsn: str, *, min_size: int = 1, max_size: int = 4) -> None:
        self.pool = AsyncConnectionPool(
            conninfo=dsn,
            min_size=min_size,
            max_size=max_size,
            open=False,
            configure=_configure_connection,
        )
        self._opened = False

    async def open(self) -> None:
        if self._opened:
            return
        await self.pool.open()
        await self.pool.wait()
        self._opened = True

    async def close(self) -> None:
        if self._opened:
            await self.pool.close()
            self._opened = False

    async def ping(self) -> None:
        await self.fetch_one("SELECT 1 AS ok")

    async def fetch_one(self, query: str, params: Sequence[Any] = ()) -> dict[str, Any] | None:
        async with self.pool.connection() as connection:
            async with connection.cursor(row_factory=dict_row) as cursor:
                await cursor.execute(query, params)
                row = await cursor.fetchone()
                return dict(row) if row is not None else None

    async def fetch_all(self, query: str, params: Sequence[Any] = ()) -> list[dict[str, Any]]:
        async with self.pool.connection() as connection:
            async with connection.cursor(row_factory=dict_row) as cursor:
                await cursor.execute(query, params)
                rows = await cursor.fetchall()
                return [dict(row) for row in rows]

    async def execute(self, query: str, params: Sequence[Any] = ()) -> None:
        async with self.pool.connection() as connection:
            async with connection.cursor() as cursor:
                await cursor.execute(query, params)

    @asynccontextmanager
    async def transaction(self) -> AsyncIterator[Any]:
        async with self.pool.connection() as connection:
            async with connection.transaction():
                yield connection
