#!/usr/bin/env python3
"""Build a git-ignored graded image-search dataset from the reviewed local photos."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / ".local/shareo/local-photo-seed/manifest.json"
OUTPUT = ROOT / ".local/shareo/eval/image_search_local_v1.jsonl"

QUERY_SPECS = [
    ("object-01", "墙上挂着的小熊挂件", "object", {"门口偶遇一只小熊": 2}),
    ("object-02", "阳光下盛开的粉色桃花", "object", {"春日桃花开得正好": 2}),
    ("object-03", "木桌上的水果甜品拼盘", "object", {"这份水果甜品太治愈了": 2}),
    (
        "object-04",
        "草地上睡觉的三花猫",
        "object",
        {"晒太阳的小猫睡得好香": 2, "树下的小花猫很乖": 1},
    ),
    ("object-05", "花瓶旁边的紫色小花", "object", {"紫色小花把房间点亮了": 2}),
    ("object-06", "绿叶中盛开的白色花朵", "object", {"雨后的白花清清爽爽": 2}),
    ("object-07", "黑白条纹足球球衣", "object", {"今天也要为主队加油": 2}),
    ("object-08", "生日聚会的小熊造型蛋糕", "object", {"生日桌上的小熊蛋糕": 2}),
    ("object-09", "摆满贝类和螃蟹的海鲜餐桌", "object", {"这一桌海鲜看着就满足": 2}),
    (
        "object-10",
        "地面上开心微笑的棕色小狗",
        "object",
        {"这只小狗笑得太开心了": 2, "地板上趴着一只小可爱": 1},
    ),
    ("scene-01", "蓝天下海边的中式亭子", "scene", {"海边这座亭子很出片": 2}),
    ("scene-02", "树荫旁高耸的欧式城堡建筑", "scene", {"阳光下的老建筑真耐看": 2}),
    ("scene-03", "从山坡俯瞰红屋顶城市", "scene", {"屋顶之间藏着一座城": 2}),
    (
        "scene-04",
        "湖面铺满荷叶和荷花",
        "scene",
        {"荷叶铺满湖面太清凉": 2, "夏天的荷塘自带滤镜": 2},
    ),
    ("scene-05", "湖面上一群鸭子游过", "scene", {"湖边看鸭子慢慢游过": 2}),
    (
        "scene-06",
        "远山前湖面上的夕阳倒影",
        "scene",
        {"湖上日落温柔得刚刚好": 2, "晚风把湖面吹成金色": 1, "傍晚坐船看湖光": 1},
    ),
    ("scene-07", "夜色树林中远处亮灯的塔", "scene", {"夜色里的塔灯太好看": 2}),
    ("scene-08", "古寺入口和来往的人群", "scene", {"古寺门口的春天很热闹": 2}),
    ("scene-09", "蓝天草地上的纪念墓园", "scene", {"春风吹过安静的草地": 2}),
    ("scene-10", "晴朗天空下安静的海边沙滩", "scene", {"海边散步的快乐很简单": 2}),
    (
        "scene-11",
        "海岸线上的橙色日落",
        "scene",
        {"海边的橘色日落太治愈": 2, "落日和海风都在身边": 2, "傍晚的海岸线太温柔": 1},
    ),
    ("scene-12", "街道上巨大的彩色拱门和老建筑", "scene", {"街角拱门拍照真的很绝": 2}),
    ("scene-13", "公园里排列整齐的户外画展", "scene", {"公园里的小型艺术展": 2}),
    (
        "color-01",
        "粉色晚霞映在海面上",
        "color",
        {"海面被晚霞染成了粉色": 2, "今天的晚霞像打翻了颜料": 1},
    ),
    (
        "color-02",
        "橙红色火烧云和黑色树影",
        "color",
        {"今天的晚霞像打翻了颜料": 2, "海边的橘色日落太治愈": 1},
    ),
    (
        "color-03",
        "操场上空的粉色云朵",
        "color",
        {"操场上空飘着粉色云朵": 2, "傍晚的球场清爽又开阔": 2},
    ),
    ("color-04", "白色雪地和红色抽象雕塑", "color", {"雪后的公园像童话世界": 2}),
    ("composition-01", "透过树木框景观看湖面", "composition", {"树影之间看见一片湖": 2}),
    (
        "motion-01",
        "蓝天中成群飞翔的鸟",
        "motion",
        {"抬头遇见一群自由的鸟": 2, "海面上的飞鸟好自由": 2, "海风把海鸥都吹来了": 1},
    ),
    (
        "motion-02",
        "海鸥飞过海面和远处城市",
        "motion",
        {"海风把海鸥都吹来了": 2, "海面上的飞鸟好自由": 2},
    ),
    ("no-match-01", "月球表面的宇航员", "no-match", {}),
    ("no-match-02", "地铁站里驶来的列车", "no-match", {}),
    ("no-match-03", "厨房里正在炒菜的厨师", "no-match", {}),
    ("no-match-04", "雪山上滑雪的人", "no-match", {}),
]


def main() -> None:
    if not MANIFEST.is_file():
        raise SystemExit(f"local photo manifest missing: {MANIFEST}")
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    caption_to_id = {
        str(item["caption"]): int(item["post_id"]) for item in manifest.get("posts", [])
    }
    rows = []
    for query_id, query, category, labels in QUERY_SPECS:
        missing = set(labels) - set(caption_to_id)
        if missing:
            raise SystemExit(f"{query_id} references missing captions: {sorted(missing)}")
        relevance = {str(caption_to_id[caption]): grade for caption, grade in labels.items()}
        rows.append(
            {
                "id": query_id,
                "query": query,
                "category": category,
                "relevance": relevance,
                "relevant_post_ids": [int(value) for value in relevance],
                "expect_no_results": not relevance,
                "label_status": "complete",
            }
        )
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(
        "".join(json.dumps(item, ensure_ascii=False) + "\n" for item in rows),
        encoding="utf-8",
    )
    print(f"[PASS] wrote {len(rows)} graded image-search queries to {OUTPUT}")


if __name__ == "__main__":
    main()
