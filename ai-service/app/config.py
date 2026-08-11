from pydantic_settings import BaseSettings
from pydantic import model_validator
from urllib.parse import quote, urlsplit, urlunsplit


class Settings(BaseSettings):
    model_config = {"env_prefix": "SHAREO_AI_"}

    redis_url: str = "redis://localhost:6379/0"
    database_url: str = "postgresql://shareo_ai@127.0.0.1:5432/shareo?sslmode=disable"
    db_user: str = "shareo_ai"
    db_password: str = ""
    internal_token: str = ""  # X-Internal-Token for Go ↔ Python communication
    go_base_url: str = "http://127.0.0.1:8080"
    embedding_model: str = "OFA-Sys/chinese-clip-vit-base-patch16"
    embedding_revision: str = "36e679e65c2a2fead755ae21162091293ad37834"
    model_cache_dir: str = ""
    embedding_device: str = "auto"
    embedding_batch_size: int = 8
    embedding_concurrency: int = 1
    embedding_warmup: bool = True
    image_collection: str = "images"
    text_embedding_model: str = "BAAI/bge-small-zh-v1.5"
    text_model_cache_dir: str = ""
    text_collection: str = "post_chunks"
    chunk_size: int = 400
    chunk_overlap: int = 80
    rag_top_k: int = 15
    rag_max_sources: int = 10
    llm_base_url: str = ""
    llm_api_key: str = ""
    llm_model: str = "deepseek-chat"
    llm_timeout_seconds: float = 30.0
    bot_reclaim_interval_ms: int = 10_000
    bot_min_idle_time_ms: int = 30_000
    rag_prompt_version: str = "rag-v1"
    agent_enabled: bool = True
    agent_max_rounds: int = 4
    agent_max_tool_calls: int = 6
    agent_max_parallel_tools: int = 2
    agent_max_observation_chars: int = 12_000
    agent_search_candidate_multiplier: int = 3
    agent_search_max_candidates: int = 20
    agent_image_search_max_candidates: int = 24
    agent_timeout_seconds: float = 45.0
    agent_prompt_version: str = "agent-v1"
    agent_trace_version: str = "agent-trace-v1"
    agent_eval_isolated_history: bool = False
    search_max_limit: int = 20
    search_candidate_multiplier: int = 3
    search_max_candidates: int = 100
    log_level: str = "INFO"

    @model_validator(mode="after")
    def validate_runtime_ranges(self) -> "Settings":
        if self.db_password:
            parsed = urlsplit(self.database_url)
            username = parsed.username or self.db_user
            host = parsed.hostname or "127.0.0.1"
            if parsed.port:
                host = f"{host}:{parsed.port}"
            userinfo = f"{quote(username, safe='')}:{quote(self.db_password, safe='')}"
            self.database_url = urlunsplit(
                (
                    parsed.scheme or "postgresql",
                    f"{userinfo}@{host}",
                    parsed.path,
                    parsed.query,
                    parsed.fragment,
                )
            )
        ranges = {
            "embedding_batch_size": (1, 128),
            "embedding_concurrency": (1, 32),
            "chunk_size": (32, 2000),
            "rag_top_k": (1, 50),
            "rag_max_sources": (1, 20),
            "bot_reclaim_interval_ms": (1000, 600_000),
            "bot_min_idle_time_ms": (1000, 3_600_000),
            "agent_max_rounds": (1, 12),
            "agent_max_tool_calls": (1, 32),
            "agent_max_parallel_tools": (1, 8),
            "agent_max_observation_chars": (1000, 100_000),
            "agent_search_candidate_multiplier": (1, 10),
            "agent_search_max_candidates": (1, 100),
            "agent_image_search_max_candidates": (1, 100),
            "search_max_limit": (1, 100),
            "search_candidate_multiplier": (1, 10),
            "search_max_candidates": (1, 500),
        }
        for name, (lower, upper) in ranges.items():
            value = getattr(self, name)
            if not lower <= value <= upper:
                raise ValueError(f"{name} must be between {lower} and {upper}")
        if self.chunk_overlap < 0 or self.chunk_overlap >= self.chunk_size:
            raise ValueError("chunk_overlap must be between 0 and chunk_size - 1")
        if not 0.1 <= self.llm_timeout_seconds <= 300:
            raise ValueError("llm_timeout_seconds must be between 0.1 and 300")
        if not 1.0 <= self.agent_timeout_seconds <= 300:
            raise ValueError("agent_timeout_seconds must be between 1 and 300")
        return self


settings = Settings()
