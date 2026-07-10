#!/usr/bin/env python3
"""Generate unsigned Trove iOS Shortcut source files (XML plist).

Run from repo root:
  python3 examples/ios-shortcuts/generate_unsigned.py

Outputs to examples/ios-shortcuts/unsigned/*.shortcut
"""

from __future__ import annotations

import plistlib
import uuid
from pathlib import Path

OUT_DIR = Path(__file__).resolve().parent / "unsigned"

ICON = {
    "WFWorkflowIconGlyphNumber": 59511,
    "WFWorkflowIconStartColor": 463140863,
}

BASE_META = {
    "WFWorkflowClientVersion": "2302.0.2",
    "WFWorkflowClientRelease": "2.2",
    "WFWorkflowMinimumClientVersion": 900,
    "WFWorkflowMinimumClientVersionString": "900",
    "WFWorkflowIcon": ICON,
    "WFWorkflowOutputContentItemClasses": [],
}

DEFAULT_INGEST_URL = "https://trove.local/ingest/shortcuts"


def uid() -> str:
    return str(uuid.uuid4()).upper()


def text(value: str) -> dict:
    return {
        "Value": {"string": value, "attachmentsByRange": {}},
        "WFSerializationType": "WFTextTokenString",
    }


def output_ref(action_uuid: str, name: str) -> dict:
    return {
        "Value": {
            "string": "￼",
            "attachmentsByRange": {
                "{0, 1}": {
                    "Type": "ActionOutput",
                    "OutputName": name,
                    "OutputUUID": action_uuid,
                }
            },
        },
        "WFSerializationType": "WFTextTokenString",
    }


def shortcut_input() -> dict:
    return {"Type": "ShortcutInput", "OutputName": "Shortcut Input"}


def act(identifier: str, params: dict, action_uuid: str | None = None) -> dict:
    item = {
        "WFWorkflowActionIdentifier": identifier,
        "WFWorkflowActionParameters": params,
    }
    if action_uuid:
        item["UUID"] = action_uuid
    return item


def content_type_header() -> dict:
    return {
        "Value": {
            "WFDictionaryFieldValueItems": [
                {
                    "WFKey": text("Content-Type"),
                    "WFItemType": 0,
                    "WFValue": text("application/json"),
                }
            ]
        },
        "WFSerializationType": "WFDictionaryFieldValue",
    }


def import_question(action_index: int) -> dict:
    return {
        "ActionIndex": action_index,
        "Category": "Parameter",
        "DefaultValue": DEFAULT_INGEST_URL,
        "ParameterKey": "WFURLActionURL",
        "Text": "Trove ingest URL (e.g. https://trove.local/ingest/shortcuts)",
    }


def url_action(action_uuid: str | None = None) -> tuple[dict, str]:
    action_uuid = action_uuid or uid()
    return (
        act(
            "is.workflow.actions.url",
            {"WFURLActionURL": text(DEFAULT_INGEST_URL), "Show-WFURLActionURL": True},
            action_uuid,
        ),
        action_uuid,
    )


def post_body_action(url_uuid: str, body_source: dict, body_name: str) -> dict:
    return act(
        "is.workflow.actions.downloadurl",
        {
            "WFURL": output_ref(url_uuid, "URL"),
            "WFHTTPMethod": "POST",
            "WFHTTPHeaders": content_type_header(),
            "WFHTTPBodyType": "File",
            "WFRequestVariable": output_ref(body_source["UUID"], body_name)
            if "UUID" in body_source
            else body_source,
        },
        uid(),
    )


def dictionary_action(items: list[dict], action_uuid: str | None = None) -> tuple[dict, str]:
    action_uuid = action_uuid or uid()
    return (
        act(
            "is.workflow.actions.dictionary",
            {
                "WFItems": {
                    "Value": {"WFDictionaryFieldValueItems": items},
                    "WFSerializationType": "WFDictionaryFieldValue",
                }
            },
            action_uuid,
        ),
        action_uuid,
    )


def dict_item(key: str, value) -> dict:
    if isinstance(value, str):
        val = text(value)
    else:
        val = value
    return {"WFKey": text(key), "WFItemType": 0, "WFValue": val}


def write_shortcut(filename: str, workflow: dict) -> None:
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    path = OUT_DIR / f"{filename}.shortcut"
    with path.open("wb") as f:
        plistlib.dump(workflow, f, fmt=plistlib.FMT_XML)
    print(f"wrote {path}")


def share_sheet() -> None:
    url_a, url_id = url_action()
    dict_a, dict_id = dictionary_action(
        [
            dict_item("type", "shortcuts.share.saved"),
            dict_item("text", shortcut_input()),
        ]
    )
    post_a = act(
        "is.workflow.actions.downloadurl",
        {
            "WFURL": output_ref(url_id, "URL"),
            "WFHTTPMethod": "POST",
            "WFHTTPHeaders": content_type_header(),
            "WFHTTPBodyType": "JSON",
            "WFJSONValues": output_ref(dict_id, "Dictionary"),
        },
        uid(),
    )
    write_shortcut(
        "trove-share-sheet",
        {
            **BASE_META,
            "WFWorkflowName": "Trove Share Sheet",
            "WFWorkflowTypes": ["ActionExtension"],
            "WFWorkflowInputContentItemClasses": [
                "WFURLContentItem",
                "WFStringContentItem",
                "WFImageContentItem",
                "WFSafariWebPageContentItem",
                "WFArticleContentItem",
            ],
            "WFWorkflowImportQuestions": [import_question(0)],
            "WFWorkflowActions": [url_a, dict_a, post_a],
        },
    )


def quick_note() -> None:
    url_a, url_id = url_action()
    ask_id = uid()
    ask_a = act(
        "is.workflow.actions.ask",
        {
            "WFAskActionPrompt": "Note",
            "WFInputType": "Text",
            "WFAllowsMultiline": True,
        },
        ask_id,
    )
    dict_a, dict_id = dictionary_action(
        [
            dict_item("type", "shortcuts.note.created"),
            dict_item("text", output_ref(ask_id, "Provided Input")),
        ]
    )
    post_a = act(
        "is.workflow.actions.downloadurl",
        {
            "WFURL": output_ref(url_id, "URL"),
            "WFHTTPMethod": "POST",
            "WFHTTPHeaders": content_type_header(),
            "WFHTTPBodyType": "JSON",
            "WFJSONValues": output_ref(dict_id, "Dictionary"),
        },
        uid(),
    )
    write_shortcut(
        "trove-quick-note",
        {
            **BASE_META,
            "WFWorkflowName": "Trove Quick Note",
            "WFWorkflowTypes": [],
            "WFWorkflowImportQuestions": [import_question(0)],
            "WFWorkflowActions": [url_a, ask_a, dict_a, post_a],
        },
    )


def url_bookmark() -> None:
    url_a, url_id = url_action()
    dict_a, dict_id = dictionary_action(
        [
            dict_item("type", "shortcuts.url.saved"),
            dict_item("url", shortcut_input()),
            dict_item("title", ""),
        ]
    )
    post_a = act(
        "is.workflow.actions.downloadurl",
        {
            "WFURL": output_ref(url_id, "URL"),
            "WFHTTPMethod": "POST",
            "WFHTTPHeaders": content_type_header(),
            "WFHTTPBodyType": "JSON",
            "WFJSONValues": output_ref(dict_id, "Dictionary"),
        },
        uid(),
    )
    write_shortcut(
        "trove-url-bookmark",
        {
            **BASE_META,
            "WFWorkflowName": "Trove URL Bookmark",
            "WFWorkflowTypes": ["ActionExtension"],
            "WFWorkflowInputContentItemClasses": [
                "WFURLContentItem",
                "WFSafariWebPageContentItem",
            ],
            "WFWorkflowImportQuestions": [import_question(0)],
            "WFWorkflowActions": [url_a, dict_a, post_a],
        },
    )


def location_checkin() -> None:
    url_a, url_id = url_action()
    loc_id = uid()
    loc_a = act("is.workflow.actions.getcurrentlocation", {}, loc_id)
    ask_id = uid()
    ask_a = act(
        "is.workflow.actions.ask",
        {"WFAskActionPrompt": "Label (optional)", "WFInputType": "Text"},
        ask_id,
    )
    dict_a, dict_id = dictionary_action(
        [
            dict_item("type", "shortcuts.location.checked"),
            dict_item("label", output_ref(ask_id, "Provided Input")),
            dict_item("latitude", output_ref(loc_id, "Current Location")),
        ]
    )
    post_a = act(
        "is.workflow.actions.downloadurl",
        {
            "WFURL": output_ref(url_id, "URL"),
            "WFHTTPMethod": "POST",
            "WFHTTPHeaders": content_type_header(),
            "WFHTTPBodyType": "JSON",
            "WFJSONValues": output_ref(dict_id, "Dictionary"),
        },
        uid(),
    )
    write_shortcut(
        "trove-location-checkin",
        {
            **BASE_META,
            "WFWorkflowName": "Trove Location Check-in",
            "WFWorkflowTypes": [],
            "WFWorkflowImportQuestions": [import_question(0)],
            "WFWorkflowActions": [url_a, loc_a, ask_a, dict_a, post_a],
        },
    )


def main() -> None:
    share_sheet()
    quick_note()
    url_bookmark()
    location_checkin()


if __name__ == "__main__":
    main()
