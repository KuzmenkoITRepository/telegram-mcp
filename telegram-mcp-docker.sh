#!/bin/bash
# Wrapper script for running telegram-mcp in Docker
# This script allows MCP clients to run telegram-mcp in a Docker container

docker run -i --rm \
  -e TG_APP_ID="${TG_APP_ID}" \
  -e TG_API_HASH="${TG_API_HASH}" \
  -e TG_SESSION_PATH=/app/data/session.json \
  -e TG_DEBUG_LOG="${TG_DEBUG_LOG:-}" \
  -v telegram-mcp-data:/app/data \
  telegram-mcp "$@"


