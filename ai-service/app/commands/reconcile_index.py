"""Audit and optionally repair the Qdrant image index."""

import argparse
import asyncio
import json
from collections import defaultdict

import httpx
from redis.asyncio import Redis

from app.config import settings
from app.core.vectorstore import ImageVectorStore

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


async def fetch_desired(client: httpx.AsyncClient) -> dict[int, set[int]]:
    if not settings.internal_token:
        raise RuntimeError("SHAREO_AI_INTERNAL_TOKEN is not configured")
    desired: dict[int, set[int]] = defaultdict(set)
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
                desired[int(item["post_id"])].update(image_ids)
        if len(items) < 200:
            break
        after_id = int(page["next_after_id"])
    return dict(desired)


async def reconcile(apply: bool) -> dict:
    vector_store = ImageVectorStore(settings.qdrant_url, settings.image_collection)
    redis = Redis.from_url(settings.redis_url, decode_responses=True)
    async with httpx.AsyncClient(timeout=10.0) as client:
        try:
            desired = await fetch_desired(client)
            inventory = await vector_store.inventory()
            repair_posts, stale_posts = calculate_diff(
                desired, inventory, settings.embedding_revision
            )
            current_posts = {int(point["post_id"]) for point in inventory}
            result = {
                "mode": "apply" if apply else "dry-run",
                "desired_posts": len(desired),
                "desired_images": sum(len(images) for images in desired.values()),
                "indexed_posts": len(current_posts),
                "indexed_images": len(inventory),
                "repair_posts": repair_posts,
                "stale_posts": stale_posts,
            }
            if apply:
                for post_id in stale_posts:
                    await vector_store.delete_post(post_id)
                for post_id in repair_posts:
                    await redis.xadd(
                        STREAM_INDEX_POST,
                        {"action": "upsert", "post_id": str(post_id)},
                    )
            return result
        finally:
            await redis.aclose()
            await vector_store.close()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="repair differences")
    args = parser.parse_args()
    print(json.dumps(asyncio.run(reconcile(args.apply)), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
