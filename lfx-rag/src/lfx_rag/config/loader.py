from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel

from lfx_rag.config.schema import LfxRagConfig

logger = logging.getLogger(__name__)


def _deep_merge(base: dict[str, Any], override: dict[str, Any]) -> dict[str, Any]:
    result = base.copy()
    for key, value in override.items():
        existing = result.get(key)
        if isinstance(existing, dict) and isinstance(value, dict):
            merged: dict[str, Any] = _deep_merge(existing, value)  # pyright: ignore[reportUnknownArgumentType]
            result[key] = merged
        else:
            result[key] = value
    return result


def _load_yaml(path: Path) -> dict[str, Any]:
    logger.debug("Loading config from %s", path)
    with open(path) as f:
        data = yaml.safe_load(f)
    result: dict[str, Any] = data if isinstance(data, dict) else {}
    return result


def _expand_paths(model: BaseModel) -> BaseModel:
    for field_name in type(model).model_fields:
        value = getattr(model, field_name)
        if isinstance(value, Path):
            setattr(model, field_name, value.expanduser().resolve())
        elif isinstance(value, BaseModel):
            _expand_paths(value)
    return model


def _find_first(paths: list[Path]) -> Path | None:
    for p in paths:
        if p.exists():
            return p
    return None


def _config_hierarchy(config_path: Path | None = None) -> list[Path]:
    paths: list[Path] = []

    global_path = _find_first(
        [
            Path.home() / ".config" / "lfx-rag" / "config.yaml",
            Path.home() / ".config" / "lfx-rag" / "config.yml",
        ]
    )
    if global_path is not None:
        paths.append(global_path)
        logger.debug("Global config found: %s", global_path)

    if config_path is not None:
        if config_path.exists():
            paths.append(config_path)
            logger.debug("Explicit config: %s", config_path)
    else:
        local_path = _find_first(
            [
                Path("config.yaml"),
                Path("config.yml"),
            ]
        )
        if local_path is not None:
            paths.append(local_path)
            logger.debug("Local config found: %s", local_path)
        else:
            default_path = _find_first(
                [
                    Path("config/default.yaml"),
                    Path("config/default.yml"),
                ]
            )
            if default_path is not None:
                paths.append(default_path)
                logger.debug("Default config found: %s", default_path)

    return paths


def load_config(config_path: str | Path | None = None) -> LfxRagConfig:
    """Load configuration with hierarchy: defaults -> ~/.config/ -> local/ -> explicit.

    Args:
        config_path: Optional explicit path to a YAML config file.
                     Overrides all other config files.

    Returns:
        Fully validated LfxRagConfig instance.
    """
    if isinstance(config_path, str):
        config_path = Path(config_path)

    base: dict[str, Any] = LfxRagConfig().model_dump()
    logger.debug("Config hierarchy started")

    for path in _config_hierarchy(config_path):
        try:
            override = _load_yaml(path)
            base = _deep_merge(base, override)
            logger.info("Loaded config: %s", path)
        except Exception as e:
            raise RuntimeError(f"Failed to load config from {path}: {e}") from e

    config = LfxRagConfig.model_validate(base)
    _expand_paths(config)

    logger.debug("Config validation complete")
    return config
