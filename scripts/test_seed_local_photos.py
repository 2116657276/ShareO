import importlib.util
import pathlib


ROOT = pathlib.Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("seed_local_photos", ROOT / "scripts/seed_local_photos.py")
assert SPEC and SPEC.loader
seed = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(seed)


def test_enumerate_images_excludes_ds_store_and_has_expected_count():
    photos = seed.enumerate_images()
    assert len(photos) == 47
    assert all(path.name != ".DS_Store" for path in photos)


def test_caption_manifest_is_sha256_bound_and_complete():
    photos = seed.enumerate_images()
    captions = seed.load_captions(photos)
    assert len(captions) == 47
    assert len({item["caption"] for item in captions.values()}) == 47


def test_expected_object_keys_include_upload_variants():
    posts = [
        {
            "image_url": "/api/v1/images/posts/original/2026/08/11/example.jpg",
            "caption": "海边的晚风",
        }
    ]
    keys = seed.expected_object_keys(posts)
    assert "posts/original/2026/08/11/example.jpg" in keys
    assert "posts/thumb/2026/08/11/example.jpg" in keys
    assert "posts/medium/2026/08/11/example.jpg" in keys
