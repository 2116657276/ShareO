"""Deterministic fingerprint of the Python source loaded by the AI process."""

import hashlib
from datetime import datetime, timezone
from pathlib import Path


def compute_source_fingerprint(app_root: Path | None = None) -> str:
    root = app_root or Path(__file__).resolve().parents[1]
    digest = hashlib.sha256()
    for path in sorted(root.rglob("*.py")):
        if "__pycache__" in path.parts:
            continue
        relative = path.relative_to(root).as_posix().encode()
        digest.update(len(relative).to_bytes(4, "big"))
        digest.update(relative)
        content = path.read_bytes()
        digest.update(len(content).to_bytes(8, "big"))
        digest.update(content)
    return digest.hexdigest()


PROCESS_STARTED_AT = datetime.now(timezone.utc).isoformat()
PROCESS_SOURCE_FINGERPRINT = compute_source_fingerprint()
