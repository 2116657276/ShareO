import pytest

from app.rag.chunking import chunk_text


def test_empty_content_has_no_chunks():
    assert chunk_text(" \n\t ") == []


def test_chinese_boundaries_are_preferred():
    chunks = chunk_text("第一句。第二句！第三句？", target_size=8, overlap=2)
    assert chunks[0][-1] in "。！？!?；;"
    assert all(len(item) <= 8 for item in chunks)


def test_long_content_is_bounded_and_overlapped():
    chunks = chunk_text("甲" * 25, target_size=10, overlap=3)
    assert chunks == ["甲" * 10, "甲" * 10, "甲" * 10, "甲" * 4]
    assert chunks[0][-3:] == chunks[1][:3]


def test_invalid_overlap_is_rejected():
    with pytest.raises(ValueError):
        chunk_text("内容", target_size=10, overlap=10)
