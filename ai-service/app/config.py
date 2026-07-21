from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    model_config = {"env_prefix": "SHAREO_AI_"}

    redis_url: str = "redis://localhost:6379/0"
    qdrant_url: str = "http://localhost:6333"
    internal_token: str = ""  # X-Internal-Token for Go ↔ Python communication
    go_base_url: str = "http://127.0.0.1:8080"
    embedding_model: str = "OFA-Sys/chinese-clip-vit-base-patch16"
    embedding_device: str = "auto"
    embedding_batch_size: int = 8
    search_max_limit: int = 20
    log_level: str = "INFO"


settings = Settings()
