"""setup-gateway usage meter for mitmproxy.

Records one JSON line per model call: time, project, client, host, model,
HTTP status and token counts. Request/response headers and bodies (prompts, answers,
API keys) are never written.

Understands Anthropic Messages, OpenAI Responses and Chat Completions, both
plain JSON and streaming (SSE), plus Responses over WebSocket (Codex).
"""

import base64
import binascii
import json
import logging
import os
import threading
from datetime import datetime, timezone
from urllib.parse import unquote

from mitmproxy import ctx, http

_lock = threading.Lock()


def _metered(path):
    path = path.split("?", 1)[0].rstrip("/")
    return path.endswith("/messages") or path.endswith("/responses") or path.endswith("/chat/completions")


def _client(agent):
    agent = (agent or "").lower()
    for needle, name in (("claude-cli", "claude"), ("claude-code", "claude"), ("codex", "codex"),
                         ("opencode", "opencode"), ("cursor", "cursor")):
        if needle in agent:
            return name
    return agent.split("/", 1)[0][:40] or "unknown"


def _num(value):
    return value if isinstance(value, int) and value >= 0 else 0


def _merge(total, usage):
    # Streaming events repeat or grow counters; the largest value is the final one.
    details_in = usage.get("input_tokens_details") or usage.get("prompt_tokens_details") or {}
    details_out = usage.get("output_tokens_details") or usage.get("completion_tokens_details") or {}
    fields = {
        "input": _num(usage.get("input_tokens", usage.get("prompt_tokens"))),
        "output": _num(usage.get("output_tokens", usage.get("completion_tokens"))),
        "cache_read": _num(usage.get("cache_read_input_tokens", details_in.get("cached_tokens") if isinstance(details_in, dict) else 0)),
        "cache_write": _num(usage.get("cache_creation_input_tokens")),
        "reasoning": _num(details_out.get("reasoning_tokens") if isinstance(details_out, dict) else 0),
    }
    for key, value in fields.items():
        total[key] = max(total.get(key, 0), value)


def _absorb(state, obj):
    if not isinstance(obj, dict):
        return
    sources = [obj]
    for key in ("message", "response"):
        if isinstance(obj.get(key), dict):
            sources.append(obj[key])
    for src in sources:
        if isinstance(src.get("model"), str) and src["model"]:
            state["served"] = src["model"]
        if isinstance(src.get("usage"), dict):
            _merge(state["usage"], src["usage"])


def _absorb_bytes(state, data):
    data = data.strip()
    if not data or data == b"[DONE]":
        return
    try:
        _absorb(state, json.loads(data))
    except (ValueError, UnicodeDecodeError):
        pass


def _project(flow):
    # setup-gateway proxy run puts the project name in the proxy URL user part;
    # clients send it as Proxy-Authorization on CONNECT. The password is unused.
    auth = flow.request.headers.get("proxy-authorization", "")
    if not auth.lower().startswith("basic "):
        return ""
    try:
        user = base64.b64decode(auth[6:].strip()).decode("utf-8", "replace").split(":", 1)[0]
    except (ValueError, binascii.Error):
        return ""
    return unquote(user)[:80]


class UsageMeter:
    def load(self, loader):
        loader.add_option("usage_log", str, "", "JSONL file that receives token usage records")

    def http_connect(self, flow: http.HTTPFlow):
        project = _project(flow)
        if project:
            flow.client_conn.sg_project = project
        # Accept any project tag; mitmproxy itself has no proxy auth configured.
        flow.request.headers.pop("proxy-authorization", None)

    def requestheaders(self, flow: http.HTTPFlow):
        project = _project(flow) or getattr(flow.client_conn, "sg_project", "")
        flow.metadata["sg_project"] = project
        flow.request.headers.pop("proxy-authorization", None)

    def request(self, flow: http.HTTPFlow):
        if not _metered(flow.request.path):
            return
        # Plain bytes let the stream parser read events; usage is all we inspect.
        flow.request.headers["accept-encoding"] = "identity"
        model = None
        try:
            body = json.loads(flow.request.get_text(strict=False) or "{}")
            if isinstance(body, dict) and isinstance(body.get("model"), str):
                model = body["model"]
        except ValueError:
            pass
        flow.metadata["sg_usage"] = {"model": model, "usage": {}, "streamed": False}

    def responseheaders(self, flow: http.HTTPFlow):
        state = flow.metadata.get("sg_usage")
        if state is None or flow.response is None:
            return
        if "text/event-stream" not in flow.response.headers.get("content-type", ""):
            return
        state["streamed"] = True
        pending = bytearray()

        def watch(chunk: bytes) -> bytes:
            pending.extend(chunk)
            while True:
                end = pending.find(b"\n")
                if end < 0:
                    break
                line = bytes(pending[:end]).strip()
                del pending[:end + 1]
                if line.startswith(b"data:"):
                    _absorb_bytes(state, line[5:])
            return chunk

        flow.response.stream = watch

    def response(self, flow: http.HTTPFlow):
        state = flow.metadata.pop("sg_usage", None)
        if state is None:
            return
        if not state["streamed"] and flow.response is not None and flow.response.content:
            _absorb_bytes(state, flow.response.content)
        self._write(flow, state, flow.response.status_code if flow.response else 0)

    def error(self, flow: http.HTTPFlow):
        state = flow.metadata.pop("sg_usage", None)
        if state is not None:
            self._write(flow, state, 0)

    def websocket_message(self, flow: http.HTTPFlow):
        if not _metered(flow.request.path) or flow.websocket is None:
            return
        message = flow.websocket.messages[-1]
        if not message.is_text:
            return
        try:
            obj = json.loads(message.text)
        except ValueError:
            return
        if not isinstance(obj, dict):
            return
        if message.from_client:
            model = obj.get("model") or (obj.get("response") or {}).get("model")
            if isinstance(model, str):
                flow.metadata["sg_ws_model"] = model
            return
        if obj.get("type") not in ("response.completed", "response.incomplete", "response.failed", "response.done"):
            return
        state = {"model": flow.metadata.get("sg_ws_model"), "usage": {}}
        _absorb(state, obj)
        self._write(flow, state, 101)

    def _write(self, flow, state, status):
        usage = state["usage"]
        record = {
            "ts": datetime.now(timezone.utc).isoformat(timespec="seconds"),
            "project": flow.metadata.get("sg_project") or getattr(flow.client_conn, "sg_project", "") or "",
            "client": _client(flow.request.headers.get("user-agent")),
            "host": flow.request.pretty_host,
            "path": flow.request.path.split("?", 1)[0],
            "model": state.get("served") or state.get("model") or "?",
            "status": status,
            "input": usage.get("input", 0),
            "output": usage.get("output", 0),
            "cache_read": usage.get("cache_read", 0),
            "cache_write": usage.get("cache_write", 0),
            "reasoning": usage.get("reasoning", 0),
        }
        logging.info("[usage] %s %s HTTP %s  in=%d out=%d cache_read=%d cache_write=%d",
                     record["client"], record["model"], status, record["input"], record["output"],
                     record["cache_read"], record["cache_write"])
        path = ctx.options.usage_log
        if not path:
            return
        line = json.dumps(record, ensure_ascii=False) + "\n"
        with _lock:
            os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
            with open(path, "a", encoding="utf-8") as handle:
                handle.write(line)


addons = [UsageMeter()]
