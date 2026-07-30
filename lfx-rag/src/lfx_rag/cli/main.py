import typer

app = typer.Typer()


@app.command()
def index():
    """Index documents into the vector store."""
    print("Index not implemented yet")


@app.command()
def query():
    """Query the RAG system."""
    print("Query not implemented yet")


if __name__ == "__main__":
    app()
