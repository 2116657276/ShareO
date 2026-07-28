from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    model_config = {"env_prefix": "SHAREO_AI_"}

    redis_url: str = "redis://localhost:6379/0"
    qdrant_url: str = "http://localhost:6333"
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


settings = Settings()
