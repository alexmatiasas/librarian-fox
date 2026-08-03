.PHONY: sync lint format typecheck test check seed generate-schema clean

sync:
	uv sync

seed:
	uv run python scripts/generate_testdata.py
	uv run python scripts/load_testdata.py

seed-large:
	uv run python scripts/generate_testdata.py --large 800 --large-out /tmp/lfx_rag_large.pdf

lint:
	uv run ruff check src/ tests/

format:
	uv run ruff format src/ tests/ --check

format-fix:
	uv run ruff format src/ tests/

typecheck:
	uv run pyright src/

test:
	uv run pytest -v

generate-schema:
	uv run python scripts/generate_config_schema.py

check: lint format typecheck test

clean:
	rm -rf .ruff_cache .pytest_cache __pycache__
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
