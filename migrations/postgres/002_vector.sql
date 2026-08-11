-- pgvector schema for derived image and text embeddings.
CREATE EXTENSION IF NOT EXISTS vector;
CREATE SCHEMA IF NOT EXISTS ai;

CREATE TABLE IF NOT EXISTS ai.image_embeddings (
    image_id BIGINT PRIMARY KEY,
    post_id BIGINT NOT NULL,
    object_key VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    model_revision VARCHAR(255) NOT NULL DEFAULT '',
    embedding vector(512) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_image_embeddings_post ON ai.image_embeddings(post_id);
CREATE INDEX IF NOT EXISTS idx_ai_image_embeddings_embedding
    ON ai.image_embeddings USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS ai.post_chunk_embeddings (
    id UUID PRIMARY KEY,
    post_id BIGINT NOT NULL,
    chunk_id VARCHAR(255) NOT NULL,
    chunk_text TEXT NOT NULL,
    model_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    embedding vector(512) NOT NULL,
    CONSTRAINT uk_ai_post_chunk_embeddings_chunk UNIQUE (chunk_id)
);
CREATE INDEX IF NOT EXISTS idx_ai_post_chunk_embeddings_post ON ai.post_chunk_embeddings(post_id);
CREATE INDEX IF NOT EXISTS idx_ai_post_chunk_embeddings_embedding
    ON ai.post_chunk_embeddings USING hnsw (embedding vector_cosine_ops);
