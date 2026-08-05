import base64
import hashlib
import hmac
import json
from collections.abc import Iterator
from typing import Any

import pytest
import requests

from zwppt_mcp.client import ZhiwenApiError, ZhiwenClient, ZhiwenTimeouts
from zwppt_mcp.credentials import Credentials


EXPECTED_ENDPOINTS = {
    "get_theme_list": ("GET", "/api/ppt/v2/template/list"),
    "create_ppt_task": ("POST", "/api/ppt/v2/create"),
    "get_task_progress": ("GET", "/api/ppt/v2/progress"),
    "create_outline": ("POST", "/api/ppt/v2/createOutline"),
    "create_outline_by_doc": ("POST", "/api/ppt/v2/createOutlineByDoc"),
    "create_ppt_by_outline": ("POST", "/api/ppt/v2/createPptByOutline"),
}


class RecordingResponse:
    def __init__(self, status_code: int = 200, payload: Any = None) -> None:
        self.status_code = status_code
        self.payload = {"code": 0, "data": {}} if payload is None else payload
        self.text = json.dumps(self.payload) if isinstance(self.payload, dict) else str(self.payload)

    def json(self) -> Any:
        if isinstance(self.payload, Exception):
            raise self.payload
        return self.payload


class RecordingSession:
    def __init__(
        self,
        responses: Iterator[RecordingResponse | requests.RequestException]
        | None = None,
    ) -> None:
        self.calls: list[dict[str, Any]] = []
        self.responses = responses or iter(())

    def request(self, method: str, url: str, **kwargs: Any) -> RecordingResponse:
        call = {"method": method, "url": url, **kwargs}
        if "data" in kwargs:
            call["body"] = kwargs["data"].to_string()
        self.calls.append(call)
        response = next(self.responses, RecordingResponse())
        if isinstance(response, requests.RequestException):
            raise response
        return response


def test_signature_and_headers_are_deterministic() -> None:
    client = ZhiwenClient(
        Credentials("app-1", "secret-1"),
        session=RecordingSession(),
        clock=lambda: 1_725_000_000,
    )

    auth = hashlib.md5(b"app-11725000000").hexdigest()
    expected = base64.b64encode(
        hmac.new(b"secret-1", auth.encode(), hashlib.sha1).digest()
    ).decode()

    assert client.headers() == {
        "appId": "app-1",
        "timestamp": "1725000000",
        "signature": expected,
        "Content-Type": "application/json; charset=utf-8",
    }


def test_methods_send_documented_requests_and_return_documented_results(
    tmp_path: Any,
) -> None:
    complete = {"code": 0, "data": {"pptStatus": "done", "donePages": 3, "pptUrl": "https://ppt"}}
    session = RecordingSession(
        iter(
            [
                RecordingResponse(payload=complete),
                RecordingResponse(payload={"code": 0, "data": {"sid": "task-1"}}),
                RecordingResponse(payload=complete),
                RecordingResponse(payload=complete),
                RecordingResponse(payload=complete),
                RecordingResponse(payload={"code": 0, "data": {"sid": "task-1"}}),
            ]
        )
    )
    client = ZhiwenClient(Credentials("app-1", "secret-1"), session=session, clock=lambda: 1)
    document = tmp_path / "source.docx"
    document.write_bytes(b"document")

    assert client.get_theme_list(style="business", color="blue", industry="finance") == complete
    assert client.create_ppt_task("topic", "template-1") == {"sid": "task-1"}
    assert client.get_task_progress("task-1") == complete
    assert client.create_outline("topic") == complete
    assert client.create_outline_by_doc("source.docx", "topic", file_path=str(document)) == complete
    assert client.create_ppt_by_outline("topic", {"title": "outline"}, "template-1") == {"sid": "task-1"}

    assert [(call["method"], call["url"].removeprefix("https://zwapi.xfyun.cn")) for call in session.calls] == list(EXPECTED_ENDPOINTS.values())
    assert [call["timeout"] for call in session.calls] == [(5.0, 120.0)] * 6
    assert session.calls[0]["params"] == {"payType": "not_free", "pageNum": 2, "pageSize": 10, "style": "business", "color": "blue", "industry": "finance"}
    assert session.calls[2]["params"] == {"sid": "task-1"}
    assert session.calls[1]["headers"]["Content-Type"].startswith("multipart/form-data; boundary=")
    assert session.calls[3]["headers"]["Content-Type"].startswith("multipart/form-data; boundary=")
    assert session.calls[4]["headers"]["Content-Type"].startswith("multipart/form-data; boundary=")
    assert session.calls[5]["json"] == {
        "query": "topic", "outline": {"title": "outline"}, "templateId": "template-1",
        "author": "XXXX", "isCardNote": True, "search": False, "isFigure": True, "aiImage": "normal",
    }
    create_body = session.calls[1]["body"]
    assert b'name="query"\r\n\r\ntopic' in create_body
    assert b'name="templateId"\r\n\r\ntemplate-1' in create_body
    assert b'name="author"\r\n\r\nXXXX' in create_body
    assert b'name="isCardNote"\r\n\r\nTrue' in create_body
    assert b'name="search"\r\n\r\nFalse' in create_body
    assert b'name="isFigure"\r\n\r\nTrue' in create_body
    assert b'name="aiImage"\r\n\r\nnormal' in create_body
    outline_body = session.calls[3]["body"]
    assert b'name="query"\r\n\r\ntopic' in outline_body
    assert b'name="language"\r\n\r\ncn' in outline_body
    assert b'name="search"\r\n\r\nFalse' in outline_body
    document_body = session.calls[4]["body"]
    assert b'name="fileName"\r\n\r\nsource.docx' in document_body
    assert b'name="query"\r\n\r\ntopic' in document_body
    assert b'name="language"\r\n\r\ncn' in document_body
    assert b'name="search"\r\n\r\nFalse' in document_body
    assert b'name="file"; filename="source.docx"' in document_body


def test_create_outline_by_doc_sends_url_source_form() -> None:
    session = RecordingSession(
        iter([RecordingResponse(payload={"code": 0, "data": {"outline": []}})])
    )
    client = ZhiwenClient(Credentials("app-1", "secret-1"), session=session)

    assert client.create_outline_by_doc(
        "source.docx",
        "topic",
        file_url="https://files.example/source.docx",
        language="en",
        search=True,
    ) == {"code": 0, "data": {"outline": []}}

    call = session.calls[0]
    assert call["method"] == "POST"
    assert call["url"] == "https://zwapi.xfyun.cn/api/ppt/v2/createOutlineByDoc"
    assert call["headers"]["Content-Type"].startswith("multipart/form-data; boundary=")
    assert b'name="fileName"\r\n\r\nsource.docx' in call["body"]
    assert b'name="query"\r\n\r\ntopic' in call["body"]
    assert b'name="fileUrl"\r\n\r\nhttps://files.example/source.docx' in call["body"]
    assert b'name="language"\r\n\r\nen' in call["body"]
    assert b'name="search"\r\n\r\nTrue' in call["body"]
    assert b'name="file";' not in call["body"]


@pytest.mark.parametrize(
    ("response", "expects_redaction"),
    [
        (RecordingResponse(status_code=500, payload={"message": "app-1 secret-1 failed"}), True),
        (RecordingResponse(payload=ValueError("invalid json")), False),
        (RecordingResponse(payload={"code": 1, "message": "app-1 secret-1 rejected"}), True),
    ],
)
def test_invalid_upstream_responses_raise_redacted_api_error(
    response: RecordingResponse, expects_redaction: bool
) -> None:
    client = ZhiwenClient(
        Credentials("app-1", "secret-1"),
        session=RecordingSession(iter([response])),
    )

    with pytest.raises(ZhiwenApiError) as error:
        client.get_task_progress("task-1")

    if expects_redaction:
        assert "secret-1" not in str(error.value)
        assert "app-1" not in str(error.value)
        assert "[redacted]" in str(error.value)


def test_create_outline_by_doc_requires_exactly_one_file_source() -> None:
    client = ZhiwenClient(Credentials("app-1", "secret-1"), session=RecordingSession())

    with pytest.raises(ValueError, match="exactly one"):
        client.create_outline_by_doc("source.docx", "topic")


def test_task_creation_rejects_missing_sid() -> None:
    client = ZhiwenClient(
        Credentials("app-1", "secret-1"),
        session=RecordingSession(iter([RecordingResponse(payload={"code": 0, "data": {}})])),
    )

    with pytest.raises(ZhiwenApiError, match="sid"):
        client.create_ppt_task("topic", "template-1")


def test_error_redaction_does_not_leak_an_overlapping_secret() -> None:
    client = ZhiwenClient(
        Credentials("app", "app-secret"),
        session=RecordingSession(
            iter([RecordingResponse(payload={"code": 1, "message": "app-secret app"})])
        ),
    )

    with pytest.raises(ZhiwenApiError) as error:
        client.get_task_progress("task-1")

    assert "app" not in str(error.value)
    assert "secret" not in str(error.value)


def test_custom_connect_and_read_timeouts_are_forwarded() -> None:
    session = RecordingSession()
    client = ZhiwenClient(
        Credentials("app-1", "secret-1"),
        session=session,
        timeouts=ZhiwenTimeouts(connect_seconds=1.5, read_seconds=9.0),
    )

    client.get_task_progress("task-1")

    assert session.calls[0]["timeout"] == (1.5, 9.0)


def test_timeout_settings_are_loaded_from_bounded_environment_values() -> None:
    assert ZhiwenTimeouts.from_environ(
        {
            "PPT_CONNECT_TIMEOUT_SECONDS": "2.5",
            "PPT_READ_TIMEOUT_SECONDS": "45",
        }
    ) == ZhiwenTimeouts(connect_seconds=2.5, read_seconds=45.0)


@pytest.mark.parametrize(
    "environ",
    [
        {"PPT_CONNECT_TIMEOUT_SECONDS": "0"},
        {"PPT_CONNECT_TIMEOUT_SECONDS": "61"},
        {"PPT_READ_TIMEOUT_SECONDS": "nan"},
        {"PPT_READ_TIMEOUT_SECONDS": "301"},
    ],
)
def test_timeout_settings_reject_unbounded_values(environ: dict[str, str]) -> None:
    with pytest.raises(ValueError, match="timeout"):
        ZhiwenTimeouts.from_environ(environ)


def test_timeout_error_is_redacted_and_next_request_recovers() -> None:
    complete = {"code": 0, "data": {"pptStatus": "done"}}
    session = RecordingSession(
        iter(
            [
                requests.ConnectTimeout("connect app-1 secret-1 stalled"),
                RecordingResponse(payload=complete),
            ]
        )
    )
    client = ZhiwenClient(Credentials("app-1", "secret-1"), session=session)

    with pytest.raises(ZhiwenApiError, match="timed out") as error:
        client.get_task_progress("task-1")

    assert "app-1" not in str(error.value)
    assert "secret-1" not in str(error.value)
    assert client.get_task_progress("task-1") == complete


def test_multipart_upload_uses_timeout_and_closes_after_read_timeout(
    tmp_path: Any,
) -> None:
    session = RecordingSession(iter([requests.ReadTimeout("upload timed out")]))
    client = ZhiwenClient(
        Credentials("app-1", "secret-1"),
        session=session,
        timeouts=ZhiwenTimeouts(connect_seconds=3.0, read_seconds=30.0),
    )
    document = tmp_path / "source.docx"
    document.write_bytes(b"document")

    with pytest.raises(ZhiwenApiError, match="timed out"):
        client.create_outline_by_doc(
            "source.docx", "topic", file_path=str(document)
        )

    assert session.calls[0]["timeout"] == (3.0, 30.0)
    assert b'name="file"; filename="source.docx"' in session.calls[0]["body"]
    assert session.calls[0]["data"].fields["file"][1].closed is True
