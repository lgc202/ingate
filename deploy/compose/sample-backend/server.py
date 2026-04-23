import http.server
import os
import socketserver

HOST = os.environ.get("BIND_ADDRESS", "0.0.0.0")
PORT = int(os.environ.get("PORT", "8080"))
ORDERS_BODY = os.environ.get("ORDERS_BODY", "sample-backend-ok\n").encode()


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/orders":
            self._write_response(200, ORDERS_BODY)
            return
        if self.path == "/healthz":
            self._write_response(200, b"ok\n")
            return
        self._write_response(404, b"not found\n")

    def log_message(self, fmt, *args):
        return

    def _write_response(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class ReuseTCPServer(socketserver.TCPServer):
    allow_reuse_address = True


with ReuseTCPServer((HOST, PORT), Handler) as server:
    server.serve_forever()

