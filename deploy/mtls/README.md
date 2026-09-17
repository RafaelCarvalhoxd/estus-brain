# mTLS — autenticação por certificado de cliente

Este arquivo explica como o Estus Vault é protegido em produção. Não é uma
segunda opinião sobre o assunto — é a única barreira de acesso que existe:
**o app não tem login, senha de usuário nem sessão.** Quem apresenta um
certificado de cliente válido entra; quem não apresenta, nem chega a ver a
tela de carregamento.

## Por que assim

O app é de uso pessoal, single-user (não existe tabela de usuários em lugar
nenhum do banco). Antes, a única checagem que existia era WebAuthn (Touch
ID/Face ID) — e só no módulo Senhas, só para revelar uma senha específica; o
resto do app (financeiro, notas, documentos, agenda...) nunca teve proteção
nenhuma além de "só o servidor Next.js consegue falar com o Go".

Isso deixava de fora todo o resto do app quando exposto na internet. A troca
para mTLS resolve isso de um jeito só: a barreira sobe da aplicação para a
**camada de transporte**. Nenhuma requisição — de nenhuma rota, de nenhum
módulo — chega ao Next.js/Go sem antes provar posse de um certificado
assinado pela CA do projeto. Não há mais "revelar pede confirmação, o resto
não pede nada": ou você está dentro (com o certificado) e usa tudo, ou está
fora e o servidor nem abre a conexão TLS.

## Como funciona

```
                    HTTPS pública (Let's Encrypt)
Browser  ────────────────────────────────────────▶  Caddy (80/443, único exposto)
  │  apresenta client.p12                              │
  │  (certificado assinado pela CA do projeto)          │ 1. verifica o cert de cliente
  ▼                                                     │    contra deploy/mtls/ca/ca.crt
  Safari/Chrome pede a chave                            │ 2. se falhar: TLS nem completa,
  ao Keychain do macOS                                  │    a requisição não existe
                                                         ▼
                                                    reverse_proxy → frontend:3000 → backend:8080
                                                    (backend/frontend só em 127.0.0.1,
                                                     inalcançáveis por fora mesmo sem o Caddy)
```

- **Servidor:** o Caddy (`deploy/Caddyfile`) tem duas responsabilidades TLS
  independentes rodando ao mesmo tempo — emitir um certificado de servidor
  público via ACME/Let's Encrypt para `DOMAIN` (pra não aparecer aviso de
  "site não seguro" no navegador) e, com `client_auth { mode
  require_and_verify }`, recusar qualquer handshake TLS cujo certificado de
  cliente não seja assinado pela CA em `deploy/mtls/ca/ca.crt`.
- **CA própria:** `deploy/mtls/ca/` guarda a autoridade certificadora do
  projeto — uma CA autoassinada, criada uma única vez, que só serve para
  assinar certificados de cliente deste vault. `ca.key` é a peça mais
  sensível de toda essa configuração: quem tiver esse arquivo pode emitir um
  certificado válido para qualquer dispositivo. **Nunca vai pro git** (está
  no `.gitignore`) e idealmente nem deveria sair da máquina onde foi gerada.
- **Certificado de cliente:** cada dispositivo autorizado (seu Mac, seu
  iPhone) recebe um par de chaves assinado por essa CA, embalado num arquivo
  `.p12` protegido por senha. Importado no Keychain do macOS/iOS, o
  Safari/Chrome passam a oferecê-lo sozinhos ao navegador sempre que o
  domínio do vault pede um certificado de cliente — sem digitar nada, sem
  prompt de biometria, só a apresentação do certificado no handshake TLS.
- **Sem revogação automática:** não há CRL/OCSP configurado — é
  intencional, dado o tamanho do projeto (YAGNI). Se um `.p12` vazar, a
  única forma de invalidar o acesso é apagar `deploy/mtls/ca/` inteira,
  gerar uma CA nova e reemitir um certificado para cada dispositivo legítimo
  (a CA antiga para de ser confiável assim que o Caddy sobe com a nova
  `ca.crt`).

## Como rodar

### 1. Gerar a CA e o certificado do seu dispositivo

```bash
deploy/mtls/issue-client-cert.sh mac
```

Na primeira execução, cria a CA (`deploy/mtls/ca/ca.key` + `ca.crt`, válida
10 anos). Sempre, gera um certificado novo assinado por ela em
`deploy/mtls/clients/<nome>/client.p12` — o script pede uma senha de
exportação na hora (não fica salva em lugar nenhum).

Rode de novo com outro nome para outro dispositivo:

```bash
deploy/mtls/issue-client-cert.sh iphone
```

Ele reaproveita a CA existente — não cria uma nova a cada chamada.

### 2. Instalar o certificado no dispositivo

No Mac: duplo clique no `client.p12` (ou Keychain Access → File → Import
Items) e digite a senha definida no passo 1. No iPhone: envie o `.p12` por
AirDrop e abra — o iOS guia a importação em Ajustes.

Depois de importado, não precisa fazer mais nada: o navegador reconhece
sozinho quando o site pede um certificado de cliente e oferece o que foi
importado (pode perguntar qual usar, se houver mais de um).

### 3. Configurar o domínio e subir o Caddy

```bash
# na raiz do projeto
echo "DOMAIN=vault.seudominio.com" >> .env
docker compose --profile mtls up -d --build
```

O serviço `caddy` fica atrás do profile `mtls` de propósito: `docker
compose up --build` sozinho (uso local, sem domínio nem certificado) nunca
tenta subi-lo. Ver a seção "Deploy com mTLS" no README da raiz para o passo
a passo completo (DNS, firewall, etc.).

## Testando sem navegador

Útil para confirmar que o Caddy está mesmo bloqueando sem certificado:

```bash
# sem certificado — deve falhar o handshake TLS
curl -v https://vault.seudominio.com/

# com certificado — deve responder normalmente
curl --cert deploy/mtls/clients/mac/client.crt \
     --key  deploy/mtls/clients/mac/client.key \
     https://vault.seudominio.com/
```

## Problemas comuns

- **Navegador não oferece nenhum certificado / site pede e não aparece
  nada para escolher:** o `.p12` não foi importado nesse
  perfil/dispositivo, ou foi importado no keychain errado (no Mac,
  confirme em Keychain Access → categoria "Meus Certificados" que ele
  aparece com uma chave privada associada).
- **`ERR_BAD_SSL_CLIENT_AUTH_CERT` ou handshake falha mesmo com o
  certificado importado:** o certificado pode ter sido assinado por uma CA
  antiga (se `deploy/mtls/ca/` foi recriada depois que o certificado do
  dispositivo foi emitido) — gere um novo com `issue-client-cert.sh`.
- **Caddy não emite o certificado público (fica em "obtaining
  certificate"):** confirme que o DNS de `DOMAIN` já resolve para o IP da
  VPS e que as portas 80 e 443 estão abertas no firewall — o ACME HTTP-01
  precisa da 80 para o desafio.
- **Perdeu o `.p12`:** sem problema, ele não é a fonte da verdade — a CA
  é. Rode `issue-client-cert.sh` de novo com o mesmo nome (ou um novo) para
  emitir outro.
