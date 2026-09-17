#!/usr/bin/env bash
# Gera (uma vez) a CA privada do vault e, a cada execução, um certificado de
# cliente novo assinado por ela — o par que vai para o Keychain da sua
# máquina e autentica você no Caddy (deploy/Caddyfile, client_auth).
#
# Uso:
#   deploy/mtls/issue-client-cert.sh [nome-do-cliente]
#
# nome-do-cliente é só um rótulo (default: mac) — dá para gerar mais de um
# certificado (ex.: um por dispositivo) rodando de novo com outro nome.
#
# Saída: deploy/mtls/clients/<nome>/client.p12 — importe esse arquivo no
# Keychain Access (macOS) ou nas configurações de certificado do navegador.
# A senha do .p12 é pedida na hora e não fica salva em lugar nenhum.
set -euo pipefail

CLIENT_NAME="${1:-mac}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_DIR="$SCRIPT_DIR/ca"
CLIENT_DIR="$SCRIPT_DIR/clients/$CLIENT_NAME"

mkdir -p "$CA_DIR" "$CLIENT_DIR"

if [ ! -f "$CA_DIR/ca.key" ]; then
	echo "==> Nenhuma CA encontrada em $CA_DIR — criando uma nova (válida 10 anos)."
	openssl genrsa -out "$CA_DIR/ca.key" 4096
	openssl req -x509 -new -nodes -key "$CA_DIR/ca.key" -sha256 -days 3650 \
		-subj "/CN=Estus Vault CA" \
		-out "$CA_DIR/ca.crt"
	echo "==> CA criada: $CA_DIR/ca.crt (aponte deploy/Caddyfile pra ela — já está)."
else
	echo "==> Reaproveitando CA existente em $CA_DIR/ca.crt."
fi

echo "==> Gerando certificado de cliente para '$CLIENT_NAME'."
openssl genrsa -out "$CLIENT_DIR/client.key" 2048
openssl req -new -key "$CLIENT_DIR/client.key" \
	-subj "/CN=$CLIENT_NAME" \
	-out "$CLIENT_DIR/client.csr"
openssl x509 -req -in "$CLIENT_DIR/client.csr" \
	-CA "$CA_DIR/ca.crt" -CAkey "$CA_DIR/ca.key" -CAcreateserial \
	-days 825 -sha256 \
	-out "$CLIENT_DIR/client.crt"
rm -f "$CLIENT_DIR/client.csr"

echo "==> Empacotando em .p12 (defina uma senha de importação quando pedir)."
openssl pkcs12 -export \
	-inkey "$CLIENT_DIR/client.key" \
	-in "$CLIENT_DIR/client.crt" \
	-certfile "$CA_DIR/ca.crt" \
	-name "$CLIENT_NAME" \
	-out "$CLIENT_DIR/client.p12"

echo
echo "Pronto: $CLIENT_DIR/client.p12"
echo "No Mac: abra esse arquivo (ou Keychain Access > File > Import Items) e"
echo "digite a senha que você acabou de definir. Depois disso o Safari/Chrome"
echo "oferece esse certificado sozinho ao acessar o domínio do vault."
