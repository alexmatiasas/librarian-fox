from __future__ import annotations

from pathlib import Path

import pytest

from scripts.load_testdata import REPO_ROOT, build_fixture

FIXTURE_DB = REPO_ROOT / "lfx-rag" / "tests" / "fixtures" / "manifest.db"
TESTDATA_DIR = REPO_ROOT / "testdata"


def _is_stale() -> bool:
    if not FIXTURE_DB.exists():
        return True
    db_mtime = FIXTURE_DB.stat().st_mtime
    for path in TESTDATA_DIR.rglob("*"):
        if path.is_file() and path.stat().st_mtime > db_mtime:
            return True
    return False


@pytest.fixture(scope="session", autouse=True)
def ensure_fixture_manifest() -> Path:
    """Rebuild the fixture manifest when missing or stale.

    Keeps tests reproducible against a fresh checkout: `make seed` is
    equivalent, but running the suite without it just works.
    """
    if not TESTDATA_DIR.exists():
        raise FileNotFoundError(
            f"{TESTDATA_DIR} not found. Synthetic testdata is generated with "
            "`uv run python scripts/generate_testdata.py` (or `make seed`)."
        )
    if _is_stale():
        build_fixture(FIXTURE_DB, TESTDATA_DIR)
    return FIXTURE_DB
