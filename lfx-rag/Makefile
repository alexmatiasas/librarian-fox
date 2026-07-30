.PHONY: sync lint format typecheck test check clean

sync:
	uv sync

lint:
	uv run ruff check src/ tests/

format:
	uv run ruff format src/ tests/ --check

typecheck:
	uv run pyright src/

test:
	uv run pytest -v

check: lint format typecheck test

clean:
	rm -rf .ruff_cache .pytest_cache __pycache__
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
