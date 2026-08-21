#!/bin/sh
set -eu

required_vars="
MYSQL_DATABASE
MYSQL_USER
MYSQL_PASSWORD
REDIS_PASSWORD
JWT_SIGNING_KEY
ADMIN_USERNAME
DATA_ENCRYPTION_KEY
"

for name in $required_vars; do
    eval "value=\${$name:-}"
    if [ -z "$value" ]; then
        echo "required environment variable is missing: $name" >&2
        exit 1
    fi
done

safe_vars="MYSQL_DATABASE MYSQL_USER MYSQL_PASSWORD REDIS_PASSWORD JWT_SIGNING_KEY ADMIN_USERNAME"
for name in $safe_vars; do
    eval "value=\${$name}"
    case "$value" in
        *[!A-Za-z0-9_.-]*)
            echo "$name may only contain letters, numbers, dot, underscore, or hyphen in the Docker test environment" >&2
            exit 1
            ;;
    esac
done

decoded_key_length="$(printf '%s' "$DATA_ENCRYPTION_KEY" | base64 -d 2>/dev/null | wc -c | tr -d ' ')"
if [ "$decoded_key_length" != "32" ]; then
    echo "DATA_ENCRYPTION_KEY must be a base64 encoded 32-byte key" >&2
    exit 1
fi

umask 077
admin_password_file=/app/data/admin-password.hash
if [ ! -s "$admin_password_file" ]; then
    if [ -n "${ADMIN_INITIAL_PASSWORD:-}" ]; then
        admin_password="$ADMIN_INITIAL_PASSWORD"
    else
        admin_password="$(head -c 32 /dev/urandom | base64 | tr -d '\n=' | cut -c 1-24)"
        echo "XTrading generated the initial administrator password (shown once): $admin_password" >&2
        echo "Change it after login and protect the Docker logs." >&2
    fi
    printf '%s' "$admin_password" | /usr/local/bin/xtrading password-hash --password-stdin > "$admin_password_file.tmp"
    mv "$admin_password_file.tmp" "$admin_password_file"
    chmod 600 "$admin_password_file"
    unset admin_password
fi

ADMIN_PASSWORD_HASH="$(cat "$admin_password_file")"
case "$ADMIN_PASSWORD_HASH" in
    '$2a$'*|'$2b$'*|'$2y$'*) ;;
    *)
        echo "persisted administrator password is not a valid bcrypt hash: $admin_password_file" >&2
        exit 1
        ;;
esac
export ADMIN_PASSWORD_HASH

envsubst '${MYSQL_DATABASE} ${MYSQL_USER} ${MYSQL_PASSWORD} ${REDIS_PASSWORD} ${JWT_SIGNING_KEY} ${ADMIN_USERNAME} ${ADMIN_PASSWORD_HASH}' \
    < /app/config-templates/server.toml.template \
    > /app/runtime-config/server.toml

envsubst '${REDIS_PASSWORD} ${JWT_SIGNING_KEY}' \
    < /app/config-templates/middleware.toml.template \
    > /app/runtime-config/middleware.toml

cp /app/config-templates/logging.toml /app/runtime-config/logging.toml
cp /app/config-templates/menu_cn.json /app/runtime-config/menu_cn.json
cp /app/config-templates/rbac_model.conf /app/runtime-config/rbac_model.conf
cp /app/config-templates/openapi.json /app/runtime-config/openapi.json

exec /usr/local/bin/xtrading "$@"
