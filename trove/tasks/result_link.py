from __future__ import annotations

from dataclasses import dataclass

from trove.models import Item

from .emit import Emit


@dataclass
class ResultLink:
    source: Item | Emit
    target: Item | Emit
    rel: str
