"""Audit and optionally repair the Qdrant image index."""

import argparse
import asyncio
import json
from collections import defaultdict

import httpx
from redis.asyncio import Redis

from app.config import settings
from app.core.vectorstore import ImageVectorStore
from app.rag.chunking import chunk_text
from app.rag.vectorstore import TextVectorStore

STREAM_INDEX_POST = "shareo:stream:index_post"


def calculate_diff(
    desired: dict[int, set[int]], inventory: list[dict], revision: str
) -> tuple[list[int], list[int]]:
    current: dict[int, set[int]] = defaultdict(set)
    revisions: dict[int, set[str]] = defaultdict(set)
    for point in inventory:
        post_id = int(point["post_id"])
        current[post_id].add(int(point["image_id"]))
        revisions[post_id].add(str(point["model_revision"]))
    repair_posts = sorted(
        post_id
        for post_id, image_ids in desired.items()
        if current.get(post_id, set()) != image_ids or revisions.get(post_id, set()) != {revision}
    )
    stale_posts = sorted(post_id for post_id in current if post_id not in desired)
    return repair_posts, stale_posts


def calculate_text_diff(
    desired: dict[int, set[str]], inventory: list[dict], model_name: str
) -> tuple[list[int], list[int]]:
    current: dict[int, set[str]] = defaultdict(set)
    models: dict[int, set[str]] = defaultdict(set)
    for point in inventory:
        post_id = int(point["post_id"])
        current[post_id].add(str(point["chunk_id"]))
        models[post_id].add(str(point["model_name"]))
    repair_posts = sorted(
        post_id
        for post_id, chunk_ids in desired.items()
        if current.get(post_id, set()) != chunk_ids or models.get(post_id, set()) != {model_name}
    )
    stale_posts = sorted(post_id for post_id in current if post_id not in desired)
    return repair_posts, stale_posts


async def fetch_desired(
    client: httpx.AsyncClient,
) -> tuple[dict[int, set[int]], dict[int, set[str]]]:
    if not settings.internal_token:
        raise RuntimeError("SHAREO_AI_INTERNAL_TOKEN is not configured")
    desired_images: dict[int, set[int]] = defaultdict(set)
    desired_chunks: dict[int, set[str]] = defaultdict(set)
    after_id = 0
    while True:
        response = await client.get(
            f"{settings.go_base_url.rstrip('/')}/internal/posts/index-payloads",
            params={"after_id": after_id, "limit": 200},
            headers={"X-Internal-Token": settings.internal_token},
        )
        response.raise_for_status()
        page = response.json()["data"]
        items = page["items"]
        for item in items:
            image_ids = {int(image["image_id"]) for image in item.get("images", [])}
            if image_ids:
                desired_images[int(item["post_id"])].update(image_ids)
            chunks = chunk_text(
                str(item.get("content", "")),
                target_size=settings.chunk_size,
                overlap=settings.chunk_overlap,
            )
            if chunks:
                post_id = int(item["post_id"])
                desired_chunks[post_id].update(
                    f"{post_id}:{chunk_no}" for chunk_no in range(len(chunks))
                )
        if len(items) < 200:
            break
        after_id = int(page["next_after_id"])
    return dict(desired_images), dict(desired_chunks)


async def reconcile(apply: bool) -> dict:
    vector_store = ImageVectorStore(settings.qdrant_url, settings.image_collection)
    text_vector_store = TextVectorStore(settings.qdrant_url, settings.text_collection)
    redis = Redis.from_url(settings.redis_url, decode_responses=True)
    async with httpx.AsyncClient(timeout=10.0) as client:
        try:
            desired_images, desired_chunks = await fetch_desired(client)
            image_inventory = await vector_store.inventory()
            text_inventory = await text_vector_store.inventory()
            image_repair, image_stale = calculate_diff(
                desired_images, image_inventory, settings.embedding_revision
            )
            text_repair, text_stale = calculate_text_diff(
                desired_chunks, text_inventory, settings.text_embedding_model
            )
            result = {
                "mode": "apply" if apply else "dry-run",
                "images": {
                    "desired_posts": len(desired_images),
                    "desired_points": sum(len(items) for items in desired_images.values()),
                    "indexed_points": len(image_inventory),
                    "repair_posts": image_repair,
                    "stale_posts": image_stale,
                },
                "post_chunks": {
                    "desired_posts": len(desired_chunks),
                    "desired_points": sum(len(items) for items in desired_chunks.values()),
                    "indexed_points": len(text_inventory),
                    "repair_posts": text_repair,
                    "stale_posts": text_stale,
                },
            }
            if apply:
                for post_id in image_stale:
                    await vector_store.delete_post(post_id)
                for post_id in text_stale:
                    await text_vector_store.delete_post(post_id)
                for post_id in sorted(set(image_repair) | set(text_repair)):
                    await redis.xadd(
                        STREAM_INDEX_POST,
                        {"action": "upsert", "post_id": str(post_id)},
                    )
            return result
        finally:
            await redis.aclose()
            await vector_store.close()
            await text_vector_store.close()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="repair differences")
    args = parser.parse_args()
    print(json.dumps(asyncio.run(reconcile(args.apply)), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
