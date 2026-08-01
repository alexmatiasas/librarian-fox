from __future__ import annotations

from lfx_rag.documents.chunker import chunk_sliding_window


def test_empty_text() -> None:
    assert chunk_sliding_window("", doc_id="empty") == []


def test_text_shorter_than_chunk_size() -> None:
    text = "hello world"
    chunks = chunk_sliding_window(text, doc_id="short", chunk_size=512, overlap=128)
    assert len(chunks) == 1
    assert chunks[0].text == text
    assert chunks[0].start_idx == 0
    assert chunks[0].end_idx == 2


def test_exact_fit() -> None:
    tokens = ["word"] * 512
    text = " ".join(tokens)
    chunks = chunk_sliding_window(text, doc_id="exact", chunk_size=512, overlap=128)
    assert len(chunks) == 1
    assert chunks[0].start_idx == 0
    assert chunks[0].end_idx == 512


def test_multiple_chunks_with_overlap() -> None:
    tokens = ["token"] * 1000
    text = " ".join(tokens)
    chunks = chunk_sliding_window(text, doc_id="overlap", chunk_size=512, overlap=128)
    assert len(chunks) > 1, "Should produce multiple chunks"

    for chunk in chunks:
        assert chunk.start_idx < chunk.end_idx
        assert chunk.doc_id == "overlap"

    # Verify overlap: second chunk starts before first chunk ends
    assert chunks[1].start_idx < chunks[0].end_idx, "Should have overlap"


def test_no_overlap() -> None:
    tokens = ["token"] * 1000
    text = " ".join(tokens)
    chunks = chunk_sliding_window(text, doc_id="no_overlap", chunk_size=500, overlap=0)
    assert len(chunks) == 2
    assert chunks[1].start_idx == chunks[0].end_idx, "Should have zero overlap"


def test_overlap_larger_than_chunk_size() -> None:
    tokens = ["token"] * 500
    text = " ".join(tokens)
    chunks = chunk_sliding_window(text, doc_id="big_overlap", chunk_size=100, overlap=200)
    # overlap > chunk_size => step = 1, so we slide token by token
    assert len(chunks) > 1
    assert chunks[1].start_idx == chunks[0].start_idx + 1


def test_metadata_preserved() -> None:
    text = "hello world foo bar"
    meta = {"file_type": "pdf", "language": "en"}
    chunks = chunk_sliding_window(text, doc_id="meta", chunk_size=2, overlap=0, metadata=meta)
    assert len(chunks) == 2
    assert chunks[0].metadata == meta
    assert chunks[1].metadata == meta


def test_doc_id_propagated() -> None:
    text = "a " * 1000
    chunks = chunk_sliding_window(text, doc_id="my_doc", chunk_size=100, overlap=20)
    assert all(c.doc_id == "my_doc" for c in chunks)


def test_single_word() -> None:
    chunks = chunk_sliding_window("hello", doc_id="single", chunk_size=512, overlap=128)
    assert len(chunks) == 1
    assert chunks[0].text == "hello"
    assert chunks[0].start_idx == 0
    assert chunks[0].end_idx == 1


def test_two_words_chunk_size_one() -> None:
    chunks = chunk_sliding_window("hello world", doc_id="two", chunk_size=1, overlap=0)
    assert len(chunks) == 2
    assert chunks[0].text == "hello"
    assert chunks[1].text == "world"
