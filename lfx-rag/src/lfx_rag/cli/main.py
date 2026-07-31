from __future__ import annotations

import logging
from pathlib import Path

import typer
from dotenv import load_dotenv

from lfx_rag.config import load_config

logger = logging.getLogger(__name__)

app = typer.Typer()


def _version_callback(value: bool) -> None:
    if value:
        from lfx_rag import __version__

        typer.echo(f"lfx-rag {__version__}")
        raise typer.Exit()


@app.callback()
def main(
    ctx: typer.Context,
    config: Path | None = typer.Option(  # noqa: B008
        None,
        "--config",
        "-c",
        help="Path to YAML config file",
        exists=True,
        file_okay=True,
        dir_okay=False,
        readable=True,
    ),
    verbose: bool = typer.Option(
        False,
        "--verbose",
        "-v",
        help="Enable verbose (DEBUG) logging",
    ),
    version: bool = typer.Option(  # noqa: ARG001
        False,
        "--version",
        "-V",
        help="Show version and exit",
        callback=_version_callback,
        is_eager=True,
    ),
) -> None:
    """lfx-rag — RAG on personal document library."""
    load_dotenv()

    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(levelname)s %(name)s: %(message)s",
    )

    ctx.ensure_object(dict)
    ctx.obj["config"] = load_config(config)


@app.command()
def index(ctx: typer.Context) -> None:
    """Index documents into the vector store."""
    cfg = ctx.obj["config"]
    logger.info("Library dir: %s", cfg.document_source.library_dir)
    typer.echo("Index not implemented yet")


@app.command()
def query() -> None:
    """Query the RAG system."""
    typer.echo("Query not implemented yet")


if __name__ == "__main__":
    app()
