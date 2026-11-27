from dataclasses import dataclass, field


@dataclass
class LoadedPlugin:
    domain: str
    manifest: dict
    status: str = "loaded"
    error: str = ""
    urls: list = field(default_factory=list)
