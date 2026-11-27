import plistlib
from pathlib import Path

IOS = Path(__file__).resolve().parents[1] / "ios"
CLIENT = (IOS / "Shared" / "IngestClient.swift").read_text()
SETTINGS = (IOS / "Trove" / "SettingsView.swift").read_text()
SHARE_VIEW = (IOS / "Share" / "ShareRootView.swift").read_text()


def _plist(relative):
    return plistlib.loads((IOS / relative).read_bytes())


def test_share_extension_accepts_image_and_file():
    info = _plist("Share/Info.plist")
    extension = info["NSExtension"]
    assert extension["NSExtensionPointIdentifier"] == "com.apple.share-services"
    assert "ShareViewController" in extension["NSExtensionPrincipalClass"]
    rule = extension["NSExtensionAttributes"]["NSExtensionActivationRule"]
    assert rule["NSExtensionActivationSupportsImageWithMaxCount"] == 1
    assert rule["NSExtensionActivationSupportsFileWithMaxCount"] == 1


def test_app_and_extension_share_an_app_group():
    app = _plist("Trove/Trove.entitlements")
    share = _plist("Share/Share.entitlements")
    group = "group.com.joshmcarthur.trove"
    assert app["com.apple.security.application-groups"] == [group]
    assert share["com.apple.security.application-groups"] == [group]


def test_client_posts_multipart_file_to_ingest_with_bearer_token():
    assert "/ingest/" in CLIENT
    assert 'httpMethod = "POST"' in CLIENT
    assert "Bearer" in CLIENT
    assert 'name=\\"file\\"' in CLIENT
    assert "write:capture" in SETTINGS
    assert "IngestClient(config:" in SHARE_VIEW


def test_host_app_configures_url_and_token_without_oauth():
    assert "IngestSettings.save" in SETTINGS
    assert "ASWebAuthenticationSession" not in SETTINGS
    assert "ASWebAuthenticationSession" not in CLIENT
    assert "oauth" not in SETTINGS.lower()
    sources = "\n".join(
        path.read_text()
        for path in IOS.rglob("*.swift")
        if "Trove.xcodeproj" not in path.parts
    )
    assert "mobile_app" not in sources
    assert "webhook" not in sources.lower()
