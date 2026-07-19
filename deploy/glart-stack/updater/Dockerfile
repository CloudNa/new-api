FROM python:3.13-alpine

RUN apk add --no-cache docker-cli docker-cli-compose git curl

WORKDIR /app
COPY server.py /app/server.py

EXPOSE 8787
CMD ["python", "/app/server.py"]
