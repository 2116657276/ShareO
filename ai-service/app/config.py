from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    model_config = {"env_prefix": "SHAREO_AI_"}

    redis_url: str = "redis://localhost:6379/0"
    qdrant_url: str = "http://localhost:6333"
    internal_token: str = ""  # X-Internal-Token for Go ↔ Python communication
    log_level: str = "INFO"


settings = Settings()
