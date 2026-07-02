#!/usr/bin/env bash
set -euo pipefail

# This script creates a local .env configuration file populated with secure,
# randomly generated secrets.

ENV_FILE=".env"
EXAMPLE_FILE=".env.example"

if [ ! -f "$EXAMPLE_FILE" ]; then
    echo "Error: $EXAMPLE_FILE not found in the current directory." >&2
    exit 1
fi

if [ -f "$ENV_FILE" ]; then
    echo "Warning: $ENV_FILE already exists."
    read -p "Do you want to overwrite it? (y/N): " -r reply
    if [[ ! "$reply" =~ ^[Yy]$ ]]; then
        echo "Aborting setup. Your existing $ENV_FILE has not been modified."
        exit 0
    fi
fi

echo "Generating secrets..."
# Generate secure keys and passwords
AUTH_SECRET=$(openssl rand -base64 48 | tr -d '\n')
ADMIN_PASSWORD=$(openssl rand -base64 16 | tr -d '\n')
PG_PASSWORD=$(openssl rand -hex 16 | tr -d '\n')

# Copy the template
cp "$EXAMPLE_FILE" "$ENV_FILE"

# Replace variables using portable sed alternatives
# Using a temp file or perl for cleaner replacement across OS environments
if command -v perl >/dev/null 2>&1; then
    # Perl is highly portable and avoids escaping issues with '/' in base64
    export AUTH_SECRET ADMIN_PASSWORD PG_PASSWORD
    perl -pi -e 's/^WISELABZ_AUTH_SECRET=.*/WISELABZ_AUTH_SECRET=$ENV{AUTH_SECRET}/' "$ENV_FILE"
    perl -pi -e 's/^WISELABZ_ADMIN_PASSWORD=.*/WISELABZ_ADMIN_PASSWORD=$ENV{ADMIN_PASSWORD}/' "$ENV_FILE"
    perl -pi -e 's/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$ENV{PG_PASSWORD}/' "$ENV_FILE"
else
    # Fallback to sed (escaping base64 characters like / if needed, using | as delimiter)
    sed -i "s|^WISELABZ_AUTH_SECRET=.*|WISELABZ_AUTH_SECRET=${AUTH_SECRET}|" "$ENV_FILE"
    sed -i "s|^WISELABZ_ADMIN_PASSWORD=.*|WISELABZ_ADMIN_PASSWORD=${ADMIN_PASSWORD}|" "$ENV_FILE"
    sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG_PASSWORD}|" "$ENV_FILE"
fi

echo "========================================================================"
echo " SUCCESS: Created $ENV_FILE with generated credentials."
echo "========================================================================"
echo ""
echo " Please save these credentials in a secure place (like a password manager):"
echo ""
echo "   WISELABZ_ADMIN_PASSWORD :  $ADMIN_PASSWORD"
echo "   POSTGRES_PASSWORD       :  $PG_PASSWORD"
echo ""
echo "========================================================================"
echo " Note: The JWT secret (WISELABZ_AUTH_SECRET) was also generated and saved."
echo "========================================================================"
