"""Bundled prompt file loader.

Resolves prompt markdown files from the package's bundled prompts/ sub-package
using importlib.resources. CWD-independent — works from any working directory.
"""

from importlib.resources import files


def load_prompt(filename: str) -> str:
    """Load a bundled prompt file by filename.

    Args:
        filename: The filename of the prompt (e.g., "reverse-engineering-prompt.md").

    Returns:
        The full text content of the prompt file.

    Raises:
        FileNotFoundError: If the bundled file is not found (corrupt installation).
    """
    try:
        resource = files("reverse_engineer.prompts").joinpath(filename)
        return resource.read_text(encoding="utf-8")
    except (FileNotFoundError, ModuleNotFoundError):
        raise FileNotFoundError(
            f"Bundled prompt file not found: {filename!r}. "
            "The package installation may be corrupt."
        )
