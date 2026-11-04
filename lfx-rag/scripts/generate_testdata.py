"""Generate deterministic synthetic testdata files for the RAG pipeline.

Writes small, reproducible PDFs (with and without a text layer), an EPUB, a
DjVu and a DOCX into ``testdata/`` following the folder conventions the seed
script expects (``<Tipo>/<Subject>/<file>``, ``Autor - Ano - Titulo.ext``).

Synthetic content is used so the committed test corpus is free of personal or
copyrighted material; what matters for the pipeline is format + structure +
language, not the actual text.

Usage:
    uv run python scripts/generate_testdata.py
    uv run python scripts/generate_testdata.py --large 800 --out /tmp/large.pdf

The ``--large`` PDF is generated on demand (not committed) for scale checks.
"""

from __future__ import annotations

import argparse
import html
import re
import shutil
import subprocess
import zipfile
from collections.abc import Callable
from pathlib import Path
from typing import cast

import pymupdf
from ebooklib import epub

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parents[1]
TESTDATA_DIR = REPO_ROOT / "testdata"

_ES_PARAGRAPHS = [
    "En este documento se describe la teoría de la física cuántica "
    "y sus aplicaciones en el campo de la investigación.",
    "Los resultados de los experimentos muestran que las partículas subatómicas "
    "se comportan de una manera que no es del todo intuitiva para el observador.",
    "Para entender estos fenómenos, es necesario analizar las ecuaciones y las leyes "
    "que rigen el comportamiento de la materia a escalas muy pequeñas.",
    "El estudio de estos sistemas permite desarrollar nuevas tecnologías "
    "que benefician a la sociedad en su conjunto.",
]

_EN_PARAGRAPHS = [
    "This document describes the theory of quantum entanglement "
    "and its applications in the field of research.",
    "The results of the experiments show that subatomic particles behave "
    "in a way that is not entirely intuitive to the observer.",
    "To understand these phenomena, it is necessary to analyze the equations "
    "and the laws that govern the behavior of matter at very small scales.",
    "The study of these systems enables the development of new technologies "
    "that benefit society as a whole.",
]

_DOCX_CONTENT_TYPES = (
    '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
    '<Default Extension="rels" '
    'ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
    '<Default Extension="xml" ContentType="application/xml"/>'
    '<Override PartName="/word/document.xml" '
    'ContentType="application/vnd.openxmlformats-officedocument.'
    'wordprocessingml.document.main+xml"/>'
    "</Types>"
)

_DOCX_RELS = (
    '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" '
    'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/'
    'officeDocument" Target="word/document.xml"/>'
    "</Relationships>"
)

_ID_PATTERN = re.compile(rb"/ID(\s*)\[[^\]]{0,120}\]")
_FIXED_ID = b"<00000000000000000000000000000000><00000000000000000000000000000000>"
_FIXED_MTIME = (1980, 1, 1, 0, 0, 0)
_DCTERMS_PATTERN = re.compile(rb'<meta property="dcterms:modified">[^<]*</meta>')
_DCTERMS_FIXED = b'<meta property="dcterms:modified">2000-01-01T00:00:00Z</meta>'


def _normalize_pdf_id(path: Path) -> None:
    """Replace pymupdf's random trailer /ID with a fixed value.

    Keeps generated PDFs byte-deterministic. PyMuPDF may write the /ID as a
    hex string (``<...>``) or a literal string (``(...)``) and with or without
    whitespace after ``/ID``; the replacement preserves the original
    whitespace and the exact byte length, so xref offsets stay valid.
    """

    def _fix(match: re.Match[bytes]) -> bytes:
        return b"/ID" + match.group(1) + b"[" + _FIXED_ID + b"]"

    data = _ID_PATTERN.sub(_fix, path.read_bytes())
    path.write_bytes(data)


def _fix_epub_dcterms(path: Path) -> None:
    """Pin ebooklib's ``dcterms:modified`` timestamp in content.opf.

    ebooklib stamps the current UTC time into the OPF metadata, which breaks
    byte-determinism. Replace it with a fixed value.
    """
    tmp = path.with_suffix(".tmp")
    with zipfile.ZipFile(path) as src, zipfile.ZipFile(tmp, "w") as dst:
        for info in src.infolist():
            data = src.read(info.filename)
            if info.filename == "EPUB/content.opf":
                data = _DCTERMS_PATTERN.sub(_DCTERMS_FIXED, data)
            dst.writestr(info, data)
    tmp.replace(path)


def _normalize_zip_times(path: Path) -> None:
    """Rewrite a zip with fixed entry timestamps.

    ``zipfile`` stamps entries with the current time, which breaks
    byte-determinism. Re-stamps every entry (preserving order, compression
    and the uncompressed leading ``mimetype`` entry required by EPUB).
    """
    tmp = path.with_suffix(".tmp")
    with zipfile.ZipFile(path) as src, zipfile.ZipFile(tmp, "w") as dst:
        for info in src.infolist():
            data = src.read(info.filename)
            info.date_time = _FIXED_MTIME
            info.comment = b""
            dst.writestr(info, data)
    tmp.replace(path)


def _write_text_pdf(path: Path, title: str, pages: int, paragraphs: list[str]) -> None:
    doc = pymupdf.open()
    for i in range(pages):
        page = doc.new_page()
        page.insert_text((72, 72), f"{title} - page {i + 1}")
        y = 100
        for paragraph in paragraphs:
            page.insert_text((72, y), paragraph)
            y += 20
    doc.save(str(path))
    doc.close()
    _normalize_pdf_id(path)


def _write_scan_pdf(path: Path, pages: int) -> None:
    src = pymupdf.open()
    src_page = src.new_page()
    src_page.insert_text((72, 72), "Scanned page content awaiting OCR.")
    pix = src_page.get_pixmap(dpi=72)
    png: bytes = pix.tobytes("png")
    width = cast(int, pix.width)
    height = cast(int, pix.height)
    src.close()

    doc = pymupdf.open()
    for _ in range(pages):
        page = doc.new_page(width=width, height=height)
        page.insert_image(page.rect, stream=png)
    for page in doc:
        text = page.get_text()
        if isinstance(text, str) and text.strip():
            raise RuntimeError("scan PDF unexpectedly has a text layer")
    doc.save(str(path))
    doc.close()
    _normalize_pdf_id(path)


def _write_epub(path: Path, title: str, author: str, lang: str) -> None:
    book = epub.EpubBook()
    book.set_identifier(f"lfx-rag-test-{lang}-{title.lower().replace(' ', '-')}")
    book.set_title(title)
    book.set_language(lang)
    book.add_author(author)

    chapters = []
    for i, paragraph in enumerate(_EN_PARAGRAPHS):
        chapter = epub.EpubHtml(
            title=f"Chapter {i + 1}",
            file_name=f"chap_{i + 1}.xhtml",
            lang=lang,
        )
        chapter.content = f"<h1>Chapter {i + 1}</h1><p>{paragraph}</p>"
        book.add_item(chapter)
        chapters.append(chapter)
        book.toc.append(chapter)

    book.add_item(epub.EpubNcx())
    book.add_item(epub.EpubNav())
    book.spine = ["nav", *chapters]
    epub.write_epub(str(path), book)
    _fix_epub_dcterms(path)
    _normalize_zip_times(path)


def _write_pbm(width: int, height: int, path: Path) -> None:
    row_bytes = (width + 7) // 8
    rows = bytearray()
    for y in range(height):
        for x in range(row_bytes):
            byte = 0
            for bit in range(8):
                if ((x * 8 + bit) + y * 17) % 7 == 0:
                    byte |= 1 << (7 - bit)
            rows.append(byte)
    header = f"P4\n{width} {height}\n".encode()
    path.write_bytes(header + bytes(rows))


def _write_djvu(path: Path, width: int = 512, height: int = 512) -> None:
    c44 = shutil.which("c44")
    if c44 is None:
        raise RuntimeError("c44 (djvulibre) not found; cannot generate DjVu")
    pbm = path.parent / f"{path.stem}.pbm"
    _write_pbm(width, height, pbm)
    try:
        subprocess.run([c44, str(pbm), str(path)], check=True, capture_output=True)
    finally:
        pbm.unlink(missing_ok=True)


def _write_docx(path: Path, paragraphs: list[str]) -> None:
    body = "".join(
        f'<w:p><w:r><w:t xml:space="preserve">{html.escape(p)}</w:t></w:r></w:p>'
        for p in paragraphs
    )
    document = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<w:document xmlns:w="http://schemas.openxmlformats.org/'
        'wordprocessingml/2006/main">'
        f"<w:body>{body}</w:body></w:document>"
    )
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("[Content_Types].xml", _DOCX_CONTENT_TYPES)
        z.writestr("_rels/.rels", _DOCX_RELS)
        z.writestr("word/document.xml", document)
    _normalize_zip_times(path)


_SYNTHETIC_SPECS: list[tuple[str, Callable[[Path], None]]] = [
    (
        "Papers/Physics/Author - 2021 - Synthetic paper on quantum entanglement.pdf",
        lambda p: _write_text_pdf(p, "Synthetic Paper on Quantum Entanglement", 8, _EN_PARAGRAPHS),
    ),
    (
        "Libros/Computer Science/A. Author - 2020 - Introduction to Machine Learning.pdf",
        lambda p: _write_text_pdf(p, "Introduction to Machine Learning", 20, _EN_PARAGRAPHS),
    ),
    (
        "Notas/Physics/Notas de clase.pdf",
        lambda p: _write_text_pdf(p, "Notas de clase", 5, _ES_PARAGRAPHS),
    ),
    (
        "Manuales/Statistics/Manual de R.pdf",
        lambda p: _write_text_pdf(p, "Manual de R", 6, _ES_PARAGRAPHS),
    ),
    (
        "Documentos Personales/Carta de motivación.pdf",
        lambda p: _write_text_pdf(p, "Carta de motivación", 2, _ES_PARAGRAPHS),
    ),
    (
        "Libros/Physics/Scan sin texto.pdf",
        lambda p: _write_scan_pdf(p, 2),
    ),
    (
        "Libros/Grammar/English grammar guide.epub",
        lambda p: _write_epub(p, "English Grammar Guide", "A. Grammarian", "en"),
    ),
    (
        "Libros/Physics/Libro antiguo.djvu",
        lambda p: _write_djvu(p),
    ),
    (
        "Reportes/Finanzas/Reporte trimestral.docx",
        lambda p: _write_docx(p, _ES_PARAGRAPHS),
    ),
]


def generate_synthetic_testdata(target_dir: Path) -> list[Path]:
    """Write all synthetic files under ``target_dir``.

    Deterministic: same input always produces byte-identical files. Returns
    the list of written paths. Real (committed) files are left untouched.
    """
    written: list[Path] = []
    for rel, writer in _SYNTHETIC_SPECS:
        path = target_dir / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        writer(path)
        written.append(path)
    return written


def generate_large_pdf(path: Path, pages: int) -> None:
    _write_text_pdf(path, "Large Synthetic Book", pages, _EN_PARAGRAPHS)


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate synthetic testdata files.")
    parser.add_argument("--out", type=Path, default=TESTDATA_DIR, help="Output directory")
    parser.add_argument(
        "--large",
        type=int,
        default=0,
        metavar="N",
        help="Also generate an N-page PDF for scale checks",
    )
    parser.add_argument(
        "--large-out",
        type=Path,
        default=Path("/tmp/lfx_rag_large.pdf"),
        help="Output path for the large PDF (default: /tmp/lfx_rag_large.pdf)",
    )
    args = parser.parse_args()

    written = generate_synthetic_testdata(args.out)
    print(f"Generated {len(written)} synthetic files under {args.out}")

    if args.large > 0:
        generate_large_pdf(args.large_out, args.large)
        print(f"Generated large PDF: {args.large_out} ({args.large} pages)")


if __name__ == "__main__":
    main()
