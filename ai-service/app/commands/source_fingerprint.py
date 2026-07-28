"""Print the current AI source fingerprint for local runtime verification."""

from app.core.source_fingerprint import compute_source_fingerprint


def main() -> None:
    print(compute_source_fingerprint())


if __name__ == "__main__":
    main()
