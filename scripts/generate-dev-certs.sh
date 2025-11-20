#!/bin/bash
# Generate development certificates for mTLS testing
# This script creates a CA, server certificate, and client certificate for local development

set -e

CERTS_DIR="deployments/certs"
mkdir -p "$CERTS_DIR"

echo "🔐 Generating development certificates for mTLS..."
echo ""

# 1. Generate CA (Certificate Authority)
echo "📜 Step 1/3: Generating Certificate Authority (CA)..."
openssl genrsa -out "$CERTS_DIR/ca.key" 4096 2>/dev/null
openssl req -new -x509 -key "$CERTS_DIR/ca.key" -sha256 \
    -subj "/C=US/ST=CA/O=Lumo Dev/CN=Lumo Dev CA" \
    -days 365 -out "$CERTS_DIR/ca.crt"
echo "  ✓ CA certificate created: $CERTS_DIR/ca.crt"
echo ""

# 2. Generate Server Certificate
echo "🖥️  Step 2/3: Generating Server Certificate..."
openssl genrsa -out "$CERTS_DIR/server.key" 4096 2>/dev/null

# Create server CSR config
cat > "$CERTS_DIR/server.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = CA
O = Lumo Dev
CN = localhost

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = lumo-api
DNS.3 = lumo-api.default.svc.cluster.local
DNS.4 = lumo-api.lumo.svc.cluster.local
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

openssl req -new -key "$CERTS_DIR/server.key" \
    -out "$CERTS_DIR/server.csr" \
    -config "$CERTS_DIR/server.cnf"

openssl x509 -req -in "$CERTS_DIR/server.csr" \
    -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial \
    -out "$CERTS_DIR/server.crt" -days 365 -sha256 \
    -extfile "$CERTS_DIR/server.cnf" -extensions v3_req 2>/dev/null

rm "$CERTS_DIR/server.csr" "$CERTS_DIR/server.cnf"
echo "  ✓ Server certificate created: $CERTS_DIR/server.crt"
echo ""

# 3. Generate Client Certificate (for agents)
echo "🤖 Step 3/3: Generating Client Certificate (for agents)..."
openssl genrsa -out "$CERTS_DIR/client.key" 4096 2>/dev/null
openssl req -new -key "$CERTS_DIR/client.key" \
    -out "$CERTS_DIR/client.csr" \
    -subj "/C=US/ST=CA/O=Lumo Agent/CN=lumo-agent"
openssl x509 -req -in "$CERTS_DIR/client.csr" \
    -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial \
    -out "$CERTS_DIR/client.crt" -days 365 -sha256 2>/dev/null

rm "$CERTS_DIR/client.csr"
echo "  ✓ Client certificate created: $CERTS_DIR/client.crt"
echo ""

# Set proper permissions
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.crt

echo "✅ Certificate generation complete!"
echo ""
echo "📁 Generated files in $CERTS_DIR/:"
echo "   ca.crt, ca.key       - Certificate Authority"
echo "   server.crt, server.key - Server certificate (for gRPC API)"
echo "   client.crt, client.key - Client certificate (for agents)"
echo ""
echo "🚀 Usage:"
echo "   Server: lumo serve-grpc --tls --cert=$CERTS_DIR/server.crt --key=$CERTS_DIR/server.key"
echo "   Client: Use client.crt and client.key for mTLS connections"
echo ""
echo "⚠️  Note: These are DEVELOPMENT certificates only. Do not use in production!"
echo "   For production, use cert-manager or your organization's PKI."
