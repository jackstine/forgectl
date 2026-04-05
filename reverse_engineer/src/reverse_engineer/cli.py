"""CLI entry point for reverse_engineer."""

from __future__ import annotations

import argparse
import asyncio
import sys

from pydantic import ValidationError

from .runner import execute


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="reverse-engineer",
        description="Run reverse-engineering agent sessions from an execute.json file.",
    )
    parser.add_argument(
        "--execute",
        required=True,
        metavar="PATH",
        help="Path to the execute.json execution file.",
    )
    args = parser.parse_args()

    try:
        asyncio.run(execute(args.execute))
    except FileNotFoundError as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
    except ValidationError as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
    except Exception as exc:  # noqa: BLE001
        print(f"Unexpected error: {exc}", file=sys.stderr)
        sys.exit(1)
