import sys
from pathlib import Path

import uvicorn
from django.conf import settings
from django.core.management import call_command


def test_trove_starts_web_and_one_ox_worker(monkeypatch):
    seen = {}

    class FakeWorker:
        def terminate(self):
            seen["terminated"] = True

        def wait(self):
            seen["waited"] = True

    def fake_popen(argv, *args, **kwargs):
        seen["argv"] = argv
        seen["kwargs"] = kwargs
        return FakeWorker()

    monkeypatch.setattr("trove.management.commands.trove.get_core", lambda: None)
    monkeypatch.setattr("trove.management.commands.trove.reap_abandoned", lambda: 0)
    monkeypatch.setattr("trove.management.commands.trove.reconcile", lambda: 0)
    monkeypatch.setattr(
        "trove.management.commands.trove.announce_setup", lambda stdout: None
    )
    monkeypatch.setattr(
        "trove.management.commands.trove.ensure_wall_client", lambda: None
    )
    monkeypatch.setattr("subprocess.Popen", fake_popen)
    monkeypatch.setattr(uvicorn, "run", lambda *a, **k: seen.__setitem__("web", True))

    call_command("trove")

    assert seen.get("web") is True
    argv = seen["argv"]
    assert argv[0] == sys.executable
    assert argv[1] == str(Path(settings.BASE_DIR) / "manage.py")
    assert argv[2:] == [
        "ox_worker",
        "--processes",
        "1",
        "--concurrency",
        str(settings.TROVE["WORKER_CONCURRENCY"]),
        "--database",
        "data",
    ]
    assert seen["kwargs"].get("shell") is not True
    assert seen.get("terminated") is True
    assert seen.get("waited") is True
