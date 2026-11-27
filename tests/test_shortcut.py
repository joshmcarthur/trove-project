import plistlib
from pathlib import Path

SHORTCUT = Path(__file__).resolve().parents[1] / "ios" / "Capture.plist"


def _workflow():
    return plistlib.loads(SHORTCUT.read_bytes())


def _dict_items(wrapped):
    return wrapped["Value"]["WFDictionaryFieldValueItems"]


def _key_name(item):
    return item["WFKey"]["Value"]["string"]


def test_share_sheet_posts_multipart_file_to_ingest():
    workflow = _workflow()
    assert workflow["WFWorkflowName"] == "Capture"
    assert "Action" in workflow["WFWorkflowTypes"]
    classes = set(workflow["WFWorkflowInputContentItemClasses"])
    assert {
        "WFImageContentItem",
        "WFPhotoMediaContentItem",
        "WFGenericFileContentItem",
    } <= classes

    download = next(
        action
        for action in workflow["WFWorkflowActions"]
        if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.downloadurl"
    )
    params = download["WFWorkflowActionParameters"]
    assert params["WFHTTPMethod"] == "POST"
    assert params["WFHTTPBodyType"] == "Form"
    assert params["WFURL"]["Value"]["string"].endswith("/ingest/")

    headers = {_key_name(item): item for item in _dict_items(params["WFHTTPHeaders"])}
    assert headers["Authorization"]["WFValue"]["Value"]["string"].startswith("Bearer ")

    form = {_key_name(item): item for item in _dict_items(params["WFFormValues"])}
    assert set(form) == {"file", "title", "notes", "url"}
    assert form["file"]["WFItemType"] == 5
    assert form["file"]["WFValue"]["Value"]["Value"]["Type"] == "ExtensionInput"


def test_import_asks_for_url_and_token():
    questions = _workflow()["WFWorkflowImportQuestions"]
    assert [question["ActionIndex"] for question in questions] == [1, 2]
    assert all(question["ParameterKey"] == "WFTextActionText" for question in questions)
    prompts = " ".join(question["Text"] for question in questions).lower()
    assert "url" in prompts
    assert "token" in prompts
