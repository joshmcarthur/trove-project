from trove.models import Item
from trove.services.blobs import blob_path


class Context:
    """The narrow surface a processor sees."""

    def __init__(self, options: dict):
        self.options = options

    def values(self, item: Item) -> dict:
        return item.current_values()

    def blob_path(self, sha: str):
        return blob_path(sha)
