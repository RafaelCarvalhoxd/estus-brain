#!/bin/bash
# Sobe o que estiver faltando (Postgres, backend, frontend) e abre a janela.
# Chamado pelo Estus Brain.app; também funciona direto no terminal.
#
#   scripts/estus-brain.sh              # sobe o que falta e abre
#   scripts/estus-brain.sh --rebuild    # força rebuild do backend e do frontend
#   scripts/estus-brain.sh --stop       # derruba backend e frontend
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Portas fora da faixa disputada por outras ferramentas (e abaixo de 49152,
# o início da faixa efêmera do macOS). 37887 é "ESTUS" no teclado do telefone.
WEB_PORT=37887
API_PORT=37888
URL="http://localhost:$WEB_PORT"
LOG_DIR="$HOME/Library/Logs/EstusBrain"
RUN_DIR="$LOG_DIR/run"
mkdir -p "$LOG_DIR" "$RUN_DIR"

# O Finder não dá o PATH do shell: monte o dele aqui.
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
if ! command -v node >/dev/null 2>&1 && [ -d "$HOME/.nvm/versions/node" ]; then
  nvm_default="$(cat "$HOME/.nvm/alias/default" 2>/dev/null)"
  nvm_bin=""
  [ -n "$nvm_default" ] && [ -d "$HOME/.nvm/versions/node/v$nvm_default/bin" ] &&
    nvm_bin="$HOME/.nvm/versions/node/v$nvm_default/bin"
  [ -z "$nvm_bin" ] && nvm_bin="$(ls -d "$HOME"/.nvm/versions/node/*/bin 2>/dev/null | sort -V | tail -1)"
  [ -n "$nvm_bin" ] && export PATH="$nvm_bin:$PATH"
fi

note() { osascript -e "display notification \"$1\" with title \"Estus Brain\"" >/dev/null 2>&1; }

die() {
  echo "ERRO: $1" >&2
  osascript -e "display dialog \"$1\n\nO log fica em ~/Library/Logs/EstusBrain/\" \
    with title \"Estus Brain\" buttons {\"OK\"} default button 1 with icon stop" >/dev/null 2>&1
  exit 1
}

listening() { lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; }
free_port() { ! listening "$1"; }   # wait_for recebe comando, e `!` é keyword

# Espera uma condição por N segundos. wait_for <segundos> <comando...>
wait_for() {
  local deadline=$((SECONDS + $1)); shift
  until "$@" >/dev/null 2>&1; do
    [ $SECONDS -ge $deadline ] && return 1
    sleep 1
  done
}

stop_proc() {
  local name="$1" pid_file="$RUN_DIR/$1.pid" pid
  [ -f "$pid_file" ] || return 0
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" 2>/dev/null; then
    # Filhos primeiro (o next levanta workers), depois o pai.
    pkill -TERM -P "$pid" 2>/dev/null
    kill -TERM "$pid" 2>/dev/null
    echo "parado: $name (pid $pid)"
  fi
  rm -f "$pid_file"
}

stop_all() {
  stop_proc frontend
  stop_proc backend
  exit 0
}

REBUILD=0
for arg in "$@"; do
  case "$arg" in
    --rebuild) REBUILD=1 ;;
    --stop) stop_all ;;
  esac
done

# ---------------------------------------------------------------- Postgres
if ! docker info >/dev/null 2>&1; then
  note "Iniciando o Docker…"
  open -ga Docker || die "Docker Desktop não está instalado em /Applications."
  wait_for 90 docker info || die "O Docker não subiu em 90s."
fi

cd "$ROOT" || die "Projeto não encontrado em $ROOT"
if [ -z "$(docker compose ps -q postgres 2>/dev/null)" ] ||
   [ "$(docker inspect -f '{{.State.Running}}' "$(docker compose ps -q postgres)" 2>/dev/null)" != "true" ]; then
  note "Subindo o banco…"
  docker compose up -d postgres >>"$LOG_DIR/postgres.log" 2>&1 || die "Falha ao subir o Postgres."
fi
wait_for 60 docker compose exec -T postgres pg_isready -U estus -d estus_vault ||
  die "O Postgres não ficou pronto em 60s."

# ---------------------------------------------------------------- Backend
# --rebuild com a API no ar: derruba primeiro, senão o bloco abaixo é pulado
# inteiro e o binário antigo continua servindo. Sem isto, --rebuild recompila
# só o frontend, e uma rota nova do backend fica invisível até alguém reparar
# que a API responde 404 para código que já está no disco.
if [ "$REBUILD" = 1 ] && listening "$API_PORT"; then
  stop_proc backend
  wait_for 15 free_port "$API_PORT"
fi

if ! listening "$API_PORT"; then
  [ -f "$ROOT/backend/.env" ] || die "Falta backend/.env (copie de backend/.env.example)."
  # Lido linha a linha, não com `source`: valores têm espaço (Estus Vault)
  # e o shell tentaria executá-los. Vai para um array em vez do ambiente
  # deste script: o backend lê PORT, e exportá-lo aqui vazaria a porta da
  # API para o `next start` lá embaixo.
  backend_env=()
  while IFS= read -r line || [ -n "$line" ]; do
    [[ $line =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || continue   # pula comentário e linha vazia
    key="${line%%=*}"; val="${line#*=}"
    val="${val%\"}"; val="${val#\"}"; val="${val%\'}"; val="${val#\'}"
    backend_env+=("$key=$val")
  done <"$ROOT/backend/.env"
  # Depois do .env, então vencem: a porta do front manda.
  backend_env+=("PORT=$API_PORT" "FRONTEND_URL=$URL")

  note "Compilando o backend…"
  ( cd "$ROOT/backend" && go build -o bin/estus-api ./cmd/api ) >>"$LOG_DIR/backend.log" 2>&1 ||
    die "Falha ao compilar o backend. Veja backend.log."

  # DOCUMENTS_DIR e ASSISTANT_DIR são relativos ao cwd do binário.
  ( cd "$ROOT/backend" && exec env "${backend_env[@]}" ./bin/estus-api ) >>"$LOG_DIR/backend.log" 2>&1 &
  echo $! >"$RUN_DIR/backend.pid"
  wait_for 30 curl -fsS "http://127.0.0.1:$API_PORT/api/health" ||
    die "O backend não respondeu em 30s. Veja backend.log."
fi

# ---------------------------------------------------------------- Frontend
# --rebuild com o app no ar: derruba primeiro, senão o bloco abaixo é pulado.
if [ "$REBUILD" = 1 ] && listening "$WEB_PORT"; then
  stop_proc frontend
  wait_for 15 free_port "$WEB_PORT"
fi

if ! listening "$WEB_PORT"; then
  cd "$ROOT/frontend" || die "frontend/ não encontrado."
  [ -f .env.local ] || echo "API_URL=http://localhost:$API_PORT" >.env.local
  [ -d node_modules ] || { note "Instalando dependências…"; npm install >>"$LOG_DIR/frontend.log" 2>&1 ||
    die "npm install falhou. Veja frontend.log."; }

  # Rebuild só quando o código mudou depois do último build.
  needs_build=$REBUILD
  if [ ! -f .next/BUILD_ID ]; then
    needs_build=1
  elif [ -n "$(find app components lib public next.config.* package.json -newer .next/BUILD_ID 2>/dev/null | head -1)" ]; then
    needs_build=1
  fi

  if [ "$needs_build" = 1 ]; then
    note "Compilando o app… (só nesta vez)"
    npm run build >>"$LOG_DIR/frontend.log" 2>&1 || die "O build do frontend falhou. Veja frontend.log."
  fi

  # O binário direto, não npx: um wrapper a menos pra sobrar órfão no --stop.
  ./node_modules/.bin/next start -p "$WEB_PORT" >>"$LOG_DIR/frontend.log" 2>&1 &
  echo $! >"$RUN_DIR/frontend.pid"
  wait_for 60 curl -fsS -o /dev/null "$URL/" || die "O frontend não respondeu em 60s. Veja frontend.log."
fi

# ---------------------------------------------------------------- Janela
# Abrir o app com uma janela dele já aberta empilhava outra, porque `open -n`
# quer dizer "nova instância". Então primeiro procuramos a que existe e a
# trazemos para a frente — assim nada do que estava em andamento se perde.
#
# O AppleScript só é consultado se o Chrome já estiver rodando: `tell
# application` num Chrome fechado o abriria, que é justamente o que não
# queremos ao apenas perguntar. Controlar o Chrome assim pede permissão de
# Automação na primeira vez; se ela for negada, ou se qualquer coisa falhar,
# `focused` não vira "found" e caímos no caminho de sempre — abrir o app
# importa mais do que focar a janela certa.
focused="$(osascript 2>/dev/null <<APPLESCRIPT
if application "Google Chrome" is running then
  tell application "Google Chrome"
    repeat with w in windows
      repeat with t in tabs of w
        if URL of t starts with "$URL" then
          set index of w to 1
          activate
          return "found"
        end if
      end repeat
    end repeat
  end tell
end if
return "none"
APPLESCRIPT
)"

if [ "$focused" != "found" ]; then
  # --app abre sem barra de endereço nem abas, no perfil padrão do Chrome.
  open -na "Google Chrome" --args --app="$URL" ||
    die "Não consegui abrir o Google Chrome."
fi
