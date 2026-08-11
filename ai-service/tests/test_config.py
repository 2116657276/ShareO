import pytest
from pydantic import ValidationError

from app.config import Settings


def test_settings_reject_unsafe_runtime_ranges():
    with pytest.raises(ValidationError):
        Settings(agent_max_parallel_tools=0)
    with pytest.raises(ValidationError):
        Settings(chunk_overlap=400, chunk_size=400)
    with pytest.raises(ValidationError):
        Settings(agent_timeout_seconds=0.5)


def test_settings_injects_escaped_postgres_password():
    settings = Settings(
        database_url="postgresql://shareo_ai@127.0.0.1:5432/shareo?sslmode=disable",
        db_password="p@ss word",
    )
    assert "shareo_ai:p%40ss%20word@127.0.0.1:5432/shareo" in settings.database_url
