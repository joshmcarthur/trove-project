"""Re-export the plugin registry so plugins can `from trove.core import Processor`."""

from trove.registry.processor import REGISTRY, Processor
from trove.runtime.core import Core, get_core, reset_core
from trove.runtime.loaded_plugin import LoadedPlugin

__all__ = [
    "REGISTRY",
    "Core",
    "LoadedPlugin",
    "Processor",
    "get_core",
    "reset_core",
]
