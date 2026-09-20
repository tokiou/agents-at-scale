import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
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
                "booking_reference": "ABC123" if "alice" in prompt.lower() or "abc123" in prompt.lower() else ("GHI789" if "carla" in prompt.lower() else "DEF456"),
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
        body = json.dumps({"model": "fake-integration", "choices": [{"message": {"content": json.dumps(result)}, "finish_reason": "stop"}]}).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


HTTPServer(("0.0.0.0", 8081), Handler).serve_forever()
