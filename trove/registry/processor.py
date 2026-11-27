from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

REGISTRY = "*"  # any registered type this processor does not consume


@dataclass
class Processor:
    name: str
    version: int
    consumes: str
    provides: list[str] | str
    run: Callable[..., Any]

    @property
    def agent(self) -> str:
        return f"{self.name}@{self.version}"
