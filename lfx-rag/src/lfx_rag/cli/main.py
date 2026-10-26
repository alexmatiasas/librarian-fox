from __future__ import annotations

import json
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
    from lfx_rag.rag import index as run_index

    docs, chunks = run_index(ctx.obj["config"])
    typer.echo(f"Indexed {chunks} chunks from {docs} documents")


@app.command()
def query(
    ctx: typer.Context,
    question: str = typer.Argument(..., help="Question to ask"),
    k: int | None = typer.Option(None, "--k", help="Chunks to retrieve"),
) -> None:
    """Query the RAG system."""
    from lfx_rag.rag import answer

    cfg = ctx.obj["config"]
    text, hits = answer(cfg, question, k or cfg.vector_store.search_limit)
    typer.echo(f"\n{text}\n")
    typer.echo("Sources:")
    seen: set[str] = set()
    for i, hit in enumerate(hits, start=1):
        source = Path(hit["source_path"]).name
        typer.echo(f"  [{i}] {hit['title'] or source} ({hit['subject'] or source})")
        seen.add(hit["doc_id"])
    logger.debug("Retrieved chunks from %d documents", len(seen))


@app.command()
def validate(ctx: typer.Context) -> None:
    """Print effective configuration."""
    cfg = ctx.obj["config"]
    data = cfg.model_dump(mode="json")
    typer.echo(json.dumps(data, indent=2))


if __name__ == "__main__":
    app()
