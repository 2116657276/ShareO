#!/usr/bin/env python3
"""Build a git-ignored post-search evaluation set from the local photo manifest."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / ".local/shareo/local-photo-seed/manifest.json"
OUTPUT = ROOT / ".local/shareo/eval/post_search_v1.jsonl"

QUERY_SPECS = [
    ("exact-01", "春日桃花", "exact", "tune", {"春日桃花开得正好": 2}),
    ("exact-02", "海边这座亭子", "exact", "validation", {"海边这座亭子很出片": 2}),
    ("exact-03", "紫色小花", "exact", "tune", {"紫色小花把房间点亮了": 2}),
    ("exact-04", "晒太阳的小猫", "exact", "validation", {"晒太阳的小猫睡得好香": 2}),
    ("exact-05", "湖上日落", "exact", "tune", {"湖上日落温柔得刚刚好": 2}),
    ("exact-06", "雪后的公园", "exact", "validation", {"雪后的公园像童话世界": 2}),
    ("exact-07", "小熊蛋糕", "exact", "tune", {"生日桌上的小熊蛋糕": 2}),
    ("exact-08", "海鲜", "exact", "tune", {"这一桌海鲜看着就满足": 2}),
    (
        "synonym-01",
        "黄昏时浪漫的运动场",
        "synonym",
        "validation",
        {"傍晚的操场有点浪漫": 2, "傍晚的球场清爽又开阔": 1},
    ),
    (
        "synonym-02",
        "湖里铺满绿色荷叶",
        "synonym",
        "tune",
        {"荷叶铺满湖面太清凉": 2, "夏天的荷塘自带滤镜": 1},
    ),
    (
        "synonym-03",
        "湖边金色的傍晚",
        "synonym",
        "tune",
        {"晚风把湖面吹成金色": 2, "湖上日落温柔得刚刚好": 1},
    ),
    (
        "synonym-04",
        "海岸边温柔的夕阳",
        "synonym",
        "validation",
        {"海边的橘色日落太治愈": 2, "落日和海风都在身边": 2, "傍晚的海岸线太温柔": 1},
    ),
    (
        "synonym-05",
        "自由飞翔的鸟群",
        "synonym",
        "tune",
        {"抬头遇见一群自由的鸟": 2, "海面上的飞鸟好自由": 2, "海风把海鸥都吹来了": 1},
    ),
    (
        "synonym-06",
        "可爱猫咪休息",
        "synonym",
        "validation",
        {"晒太阳的小猫睡得好香": 2, "路边小黑猫正在发呆": 1, "树下的小花猫很乖": 1},
    ),
    ("synonym-07", "古老建筑", "synonym", "tune", {"阳光下的老建筑真耐看": 2}),
    (
        "synonym-08",
        "甜点让人开心",
        "synonym",
        "tune",
        {"这份水果甜品太治愈了": 2, "生日桌上的小熊蛋糕": 1},
    ),
    (
        "natural-01",
        "想看傍晚湖面被夕阳照亮的作品",
        "natural",
        "validation",
        {"湖上日落温柔得刚刚好": 2, "晚风把湖面吹成金色": 2, "傍晚坐船看湖光": 1},
    ),
    (
        "natural-02",
        "有没有适合夏天看的荷塘照片",
        "natural",
        "tune",
        {"夏天的荷塘自带滤镜": 2, "荷叶铺满湖面太清凉": 2},
    ),
    (
        "natural-03",
        "找一些看起来很治愈的小动物",
        "natural",
        "tune",
        {"这只小狗笑得太开心了": 2, "晒太阳的小猫睡得好香": 2, "树下的小花猫很乖": 1},
    ),
    (
        "natural-04",
        "想看粉色晚霞和云朵",
        "natural",
        "validation",
        {"操场上空飘着粉色云朵": 2, "海面被晚霞染成了粉色": 2, "今天的晚霞像打翻了颜料": 1},
    ),
    (
        "natural-05",
        "寻找有历史感的墙和建筑",
        "natural",
        "tune",
        {"路过一面很有故事的墙": 2, "阳光下的老建筑真耐看": 1},
    ),
    ("natural-06", "适合生日庆祝的甜品", "natural", "validation", {"生日桌上的小熊蛋糕": 2}),
    (
        "multi-01",
        "蓝天下面的钟楼建筑",
        "multi",
        "tune",
        {"抬头就是蓝天和钟楼": 2, "阳光下的老建筑真耐看": 1},
    ),
    (
        "multi-02",
        "海边橘色日落和晚风",
        "multi",
        "validation",
        {"海边的橘色日落太治愈": 2, "落日和海风都在身边": 2},
    ),
    (
        "multi-03",
        "雨后清爽的白色花朵",
        "multi",
        "tune",
        {"雨后的白花清清爽爽": 2, "紫色小花把房间点亮了": 1},
    ),
    (
        "multi-04",
        "安静草地和春风",
        "multi",
        "tune",
        {"春风吹过安静的草地": 2, "雪后的公园像童话世界": 1},
    ),
    (
        "short-01",
        "猫",
        "short",
        "validation",
        {"晒太阳的小猫睡得好香": 2, "路边小黑猫正在发呆": 2, "树下的小花猫很乖": 2},
    ),
    (
        "short-02",
        "晚霞",
        "short",
        "tune",
        {"海面被晚霞染成了粉色": 2, "今天的晚霞像打翻了颜料": 2, "晚霞前的云也很漂亮": 2},
    ),
    ("no-match-01", "月球上的宇航员", "no-match", "tune", {}),
    ("no-match-02", "地铁站里的列车", "no-match", "validation", {}),
    ("no-match-03", "雪山上滑雪的人", "no-match", "tune", {}),
    ("no-match-04", "电脑屏幕上的程序代码", "no-match", "validation", {}),
]


def main() -> None:
    if not MANIFEST.is_file():
        raise SystemExit(f"local photo manifest missing: {MANIFEST}")
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    caption_to_id = {
        str(item["caption"]): int(item["post_id"]) for item in manifest.get("posts", [])
    }
    rows = []
    for query_id, query, category, split, labels in QUERY_SPECS:
        missing = set(labels) - set(caption_to_id)
        if missing:
            raise SystemExit(f"{query_id} references missing captions: {sorted(missing)}")
        relevance = {str(caption_to_id[caption]): grade for caption, grade in labels.items()}
        rows.append(
            {
                "id": query_id,
                "query": query,
                "category": category,
                "split": split,
                "relevance": relevance,
                "expect_no_results": not relevance,
                "label_status": "complete",
            }
        )
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(
        "".join(json.dumps(item, ensure_ascii=False) + "\n" for item in rows),
        encoding="utf-8",
    )
    print(f"[PASS] wrote {len(rows)} post-search queries to {OUTPUT}")


if __name__ == "__main__":
    main()
