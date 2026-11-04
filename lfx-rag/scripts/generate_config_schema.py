"""Generate JSON Schema for lfx-rag configuration.

Usage:
    uv run python scripts/generate_config_schema.py

This produces config/lfx-rag.schema.json, which provides
IDE autocompletion for YAML config files when associated
via VS Code's yaml.schemas setting or redhat YAML extension.
"""

from __future__ import annotations

import json
import logging
from pathlib import Path

from lfx_rag.config.schema import LfxRagConfig

logger = logging.getLogger(__name__)

SCHEMA_PATH = Path("config/lfx-rag.schema.json")


def main() -> None:
    schema = LfxRagConfig.model_json_schema()
    schema["$schema"] = "http://json-schema.org/draft-07/schema#"
    schema["title"] = "lfx-rag Configuration"
    schema["description"] = "JSON Schema for lfx-rag YAML configuration files"

    SCHEMA_PATH.parent.mkdir(parents=True, exist_ok=True)
    SCHEMA_PATH.write_text(json.dumps(schema, indent=2, ensure_ascii=False) + "\n")

    logger.info("Schema written to %s", SCHEMA_PATH)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(message)s")
    main()
