FROM python:3.13-alpine
WORKDIR /app
COPY deploy/compose/sample-backend/server.py /app/server.py
EXPOSE 8080
ENTRYPOINT ["python3", "/app/server.py"]
