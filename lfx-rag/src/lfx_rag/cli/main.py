from __future__ import annotations

import json
import logging
import shutil
from pathlib import Path

import httpx
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
    """Print effective configuration and check common paths."""
    cfg = ctx.obj["config"]
    data = cfg.model_dump(mode="json")
    typer.echo(json.dumps(data, indent=2))
    typer.echo("\n--- checks ---")

    def chk(path_val: Path | str | None, label: str) -> None:
        if path_val is None:
            typer.echo(f"[SKIP] {label}: None")
            return
        pp = Path(path_val).expanduser()
        typer.echo(f"[{'OK' if pp.exists() else 'MISS'}] {label}: {pp}")

    chk(cfg.document_source.manifest_path, "document_source.manifest_path")
    chk(cfg.document_source.library_dir, "document_source.library_dir")
    chk(cfg.vector_store.uri, "vector_store.uri")

    try:
        resp = httpx.get(f"{cfg.generation.local.base_url}/api/tags", timeout=2)
        typer.echo(f"[OK]   ollama {cfg.generation.local.base_url}: {resp.status_code}")
    except Exception as e:
        typer.echo(f"[MISS] ollama {cfg.generation.local.base_url}: {e}")


@app.command(name="init-config")
def init_config(
    force: bool = typer.Option(False, "--force", help="Overwrite existing config"),
) -> None:
    """Copy default config to ~/.config/lfx-rag/config.yaml."""
    target = Path.home() / ".config" / "lfx-rag" / "config.yaml"
    target.parent.mkdir(parents=True, exist_ok=True)

    if target.exists() and not force:
        typer.echo(f"exists: {target} (use --force to overwrite)")
        raise typer.Exit()

    src = Path(__file__).resolve().parents[2] / "config" / "default.yaml"
    if not src.exists():
        src = Path.cwd() / "config" / "default.yaml"

    if not src.exists():
        typer.echo("default.yaml not found")
        raise typer.Exit(code=1)

    shutil.copy(src, target)
    typer.echo(f"wrote: {target}")


if __name__ == "__main__":
    app()
