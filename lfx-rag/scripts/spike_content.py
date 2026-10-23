"""Spike: what text/metadata do real books yield? (task 003)

Read-only. Prints per-file extraction + chunk stats for 3 representative files.
shortcut: uses pymupdf directly instead of load_documents() to avoid walking the
whole library; switch to the loader once the lfx manifest exists.
"""

from __future__ import annotations

import statistics
from pathlib import Path

import pymupdf

from lfx_rag.documents.chunker import chunk_sliding_window

# This is an example of the major data collection that will be in the RAG
LIBRARY = Path("~/Documents/library").expanduser()

FILES: dict[str, str] = {
    "born_digital_pdf": "02-Papers/General-Papers/gasser1984.pdf",
    "scanned_pdf": (
        "05-Learning Languages/Languages/Resources/Ruso/"
        "Brian Kemple. - Essential Russian Grammar.pdf"
    ),
    "epub": (
        "05-Learning Languages/Languages/Resources/Ruso/"
        "[Culture smart! (London England)] Anna King_Shevchenko, Anna K - Russia - "
        "Culture Smart!_ The Essential Guide to Customs & Culture (2008, Distributed in "
        "the United States and Canada by Random House Distribution Services, Kuperard) - "
        "libgen.li.epub"
    ),
}


def main() -> None:
    for label, rel in FILES.items():
        path = LIBRARY / rel
        print(f"\n=== {label} ===")
        print(f"path: {path}")
        if not path.exists():
            print("MISSING")
            continue

        doc = pymupdf.open(str(path))
        pages: list[str] = []
        for page in doc:
            page_text = page.get_text()
            if isinstance(page_text, str):
                pages.append(page_text)
        meta: dict[str, str] = {k: v for k, v in (doc.metadata or {}).items() if v}
        doc.close()

        text = "".join(pages).strip()
        page_chars = [len(t.strip()) for t in pages]
        empty_pages = sum(1 for c in page_chars if c == 0)

        print(f"pages: {len(pages)} empty_pages: {empty_pages}")
        print(f"chars: {len(text)}")
        if page_chars:
            print(
                f"chars/page min={min(page_chars)} median={statistics.median(page_chars):.0f} "
                f"max={max(page_chars)}"
            )
        print(f"pdf_metadata: {meta}")

        chunks = chunk_sliding_window(text, doc_id=label, metadata=meta)
        if chunks:
            sizes = [len(c.text.split()) for c in chunks]
            print(
                f"chunks: {len(chunks)} words/chunk min={min(sizes)} "
                f"median={statistics.median(sizes):.0f} max={max(sizes)}"
            )
            print(f"chunk[0] metadata: {chunks[0].metadata}")
            print(f"chunk[0] preview: {chunks[0].text[:200]!r}")
        else:
            print("chunks: 0 (no extractable text -> OCR required)")


if __name__ == "__main__":
    main()
