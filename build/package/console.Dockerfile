FROM python:3.13-alpine

WORKDIR /app

COPY deploy/compose/console/server.py /app/server.py
COPY _output/compose/console/dist/ /app/dist/

EXPOSE 8080

ENTRYPOINT ["python3", "/app/server.py"]
