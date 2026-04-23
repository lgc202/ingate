from __future__ import annotations

import os
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


WEB_ROOT = Path("/app/dist")
PORT = int(os.environ.get("CONSOLE_BIND_PORT", "8080"))


class ConsoleHandler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(WEB_ROOT), **kwargs)

    def do_GET(self):
        if self.path == "/healthz":
            body = b"ok"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        super().do_GET()

    def send_head(self):
        path = self.translate_path(self.path)
        if os.path.isdir(path):
            return super().send_head()
        if os.path.exists(path):
            return super().send_head()

        self.path = "/index.html"
        return super().send_head()


if __name__ == "__main__":
    server = ThreadingHTTPServer(("0.0.0.0", PORT), ConsoleHandler)
    server.serve_forever()
