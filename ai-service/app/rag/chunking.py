"""Deterministic Chinese-first character chunking."""

import re

BOUNDARY_RE = re.compile(r"[。！？!?；;\n]")


def chunk_text(content: str, target_size: int = 400, overlap: int = 80) -> list[str]:
    if target_size < 1:
        raise ValueError("target_size must be positive")
    if overlap < 0 or overlap >= target_size:
        raise ValueError("overlap must be between zero and target_size")

    text = re.sub(r"[ \t]+", " ", content.replace("\r\n", "\n").replace("\r", "\n")).strip()
    if not text:
        return []

    boundaries = {match.end() for match in BOUNDARY_RE.finditer(text)}
    chunks: list[str] = []
    start = 0
    while start < len(text):
        limit = min(start + target_size, len(text))
        candidates = [position for position in boundaries if start < position <= limit]
        end = max(candidates) if candidates else limit
        if end <= start:
            end = limit
        chunk = text[start:end].strip()
        if chunk:
            chunks.append(chunk)
        if end >= len(text):
            break
        start = max(start + 1, end - overlap)
    return chunks
