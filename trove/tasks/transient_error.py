class TransientError(Exception):
    """Raise from a processor for errors worth retrying (429, 5xx, timeouts)."""

    def __init__(self, msg="", retry_after: float | None = None):
        super().__init__(msg)
        self.retry_after = retry_after
