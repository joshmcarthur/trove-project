from dataclasses import dataclass, field


@dataclass
class Emit:
    type: str
    key: str
    claims: dict = field(default_factory=dict)
    blob: bytes | None = None
    content_type: str = "application/octet-stream"
