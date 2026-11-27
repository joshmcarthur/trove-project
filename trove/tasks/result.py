from dataclasses import dataclass, field

from .emit import Emit
from .result_link import ResultLink


@dataclass
class Result:
    claims: dict = field(default_factory=dict)
    emits: list[Emit] = field(default_factory=list)
    links: list[ResultLink] = field(default_factory=list)
    cost: float | None = None
    params: dict = field(default_factory=dict)
