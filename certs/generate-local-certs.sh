#!/usr/bin/env bash
set -euo pipefail

# Скрипт генерирует локальный CA и серверный TLS-сертификат.

# Директория certs, независимо от того, откуда был запущен скрипт.
CERT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Приватный ключ и самоподписанный сертификат локального CA.
# ca.key нужен только для выпуска сертификатов; ca.crt клиент использует как корень доверия.
CA_KEY="${CERT_DIR}/ca.key"
CA_CERT="${CERT_DIR}/ca.crt"

# Приватный ключ сервера, заявка на сертификат и итоговый сертификат сервера.
# server.key хранится только на сервере; server.crt сервер показывает клиенту при TLS-handshake.
SERVER_KEY="${CERT_DIR}/server.key"
SERVER_CSR="${CERT_DIR}/server.csr"
SERVER_CERT="${CERT_DIR}/server.crt"
SERVER_EXT="${CERT_DIR}/server.ext"

# Срок действия сертификатов: CA живет дольше, серверный сертификат короче.
DAYS_CA=3650
DAYS_SERVER=365

# Создаем приватный ключ локального CA.
openssl genrsa -out "${CA_KEY}" 4096

# Создаем самоподписанный сертификат CA.
# Именно этому сертификату будет доверять клиент при проверке server.crt.
openssl req -x509 -new -nodes \
  -key "${CA_KEY}" \
  -sha256 \
  -days "${DAYS_CA}" \
  -out "${CA_CERT}" \
  -subj "/CN=gophkeeper-local-ca"

# Создаем приватный ключ gRPC/HTTP-сервера.
openssl genrsa -out "${SERVER_KEY}" 2048

# Создаем CSR: заявку на выпуск серверного сертификата.
# CSR содержит публичную часть server.key и имя сервера, но не содержит приватный ключ.
openssl req -new \
  -key "${SERVER_KEY}" \
  -out "${SERVER_CSR}" \
  -subj "/CN=localhost"

# SAN обязателен для современных TLS-клиентов.
# Клиент сможет подключаться к localhost или 127.0.0.1.
cat > "${SERVER_EXT}" <<'EOF_EXT'
subjectAltName = DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF_EXT

# Подписываем серверный сертификат локальным CA.
openssl x509 -req \
  -in "${SERVER_CSR}" \
  -CA "${CA_CERT}" \
  -CAkey "${CA_KEY}" \
  -CAcreateserial \
  -out "${SERVER_CERT}" \
  -days "${DAYS_SERVER}" \
  -sha256 \
  -extfile "${SERVER_EXT}"

# Для запуска приложения нужны только:
# - серверу: server.crt + server.key
# - клиенту: ca.crt
echo "Generated local TLS certificates:"
echo "  CA cert:     ${CA_CERT}"
echo "  Server cert: ${SERVER_CERT}"
echo "  Server key:  ${SERVER_KEY}"
