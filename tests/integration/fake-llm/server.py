"""Deterministic OpenAI-compatible LLM used by integration and controlled runs.

FAKE_LLM_LATENCY_MS adds a fixed delay to every completion and
FAKE_LLM_JITTER_MS adds a uniform random delay in [0, jitter] on top, so the
same simulated inference cost can be applied to both runtimes.
"""

import json
import os
import random
import re
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LATENCY_SECONDS = int(os.getenv("FAKE_LLM_LATENCY_MS", "0")) / 1000
JITTER_SECONDS = int(os.getenv("FAKE_LLM_JITTER_MS", "0")) / 1000


# Benchmark seed references (bench/seed.sql); integration fixtures fall back
# to the customer names below.
BENCH_REFERENCE = re.compile(r"\bBK\d{6}\b")


def booking_reference(prompt: str) -> str:
    match = BENCH_REFERENCE.search(prompt)
    if match:
        return match.group(0)
    lowered = prompt.lower()
    if "alice" in lowered or "abc123" in lowered:
        return "ABC123"
    if "carla" in lowered:
        return "GHI789"
    return "DEF456"


def simulated_latency() -> float:
    return LATENCY_SECONDS + random.uniform(0, JITTER_SECONDS)


class Handler(BaseHTTPRequestHandler):
    # Keep-alive: clients reuse connections instead of opening one per call,
    # which would exhaust ephemeral ports at high request rates.
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.send_header("content-length", "0")
            self.end_headers()
            return
        self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        request = json.loads(self.rfile.read(length))
        system = request["messages"][0]["content"]
        prompt = request["messages"][-1]["content"]
        if "understand" in system.lower() or "booking" in system.lower() and "departure" in system.lower():
            result = {
                "booking_reference": booking_reference(prompt),
                "departure_from": "2026-09-21T00:00:00Z",
                "departure_to": "2026-09-23T23:59:59Z",
            }
        else:
            data = json.loads(prompt)
            options = data.get("options") or data.get("Options", [])
            first = options[0] if options else {}
            flight = first.get("flight") or first.get("Flight") or {}
            result = {
                "ranked_option_ids": [flight.get("id") or flight.get("ID")] if options else [],
                "summary": "deterministic integration option",
                "has_matching_option": bool(options),
            }
        delay = simulated_latency()
        if delay > 0:
            time.sleep(delay)
        body = json.dumps({"model": "fake-integration", "choices": [{"message": {"content": json.dumps(result)}, "finish_reason": "stop"}]}).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


# A threaded server so concurrent agent runs wait in parallel instead of
# queueing behind each other's simulated latency.
ThreadingHTTPServer.daemon_threads = True
ThreadingHTTPServer.request_queue_size = 1024
ThreadingHTTPServer(("0.0.0.0", 8081), Handler).serve_forever()
