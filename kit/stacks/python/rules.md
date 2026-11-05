## Python
- Never read or search `.venv/`, `__pycache__/`, `.pytest_cache/`, or lockfiles.
- Run everything inside the project's environment: `uv run` when `uv.lock` exists, otherwise `.venv`. Never install into global Python.
- Tests run once: `pytest -q --tb=short --maxfail=1`.
- Never read or print `.env` files, which hold real secrets.
- Don't add a dependency without saying why in your report.
