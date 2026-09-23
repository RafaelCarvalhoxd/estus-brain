# Agente externo no lugar dos motores de IA — design

Data: 2026-09-23 · Status: aprovado em conversa, aguardando revisão do spec

## Objetivo

O Estus deixa de rodar a própria IA. O chat do app passa a conversar com um
agente externo — Hermes Agent, OpenClaw ou qualquer outro que fale o formato
de chat da OpenAI — e esse agente usa os dados do Estus pelo MCP que já
existe. Telegram e a ponte Apple saem. Todo código que ficar sem uso depois
disso sai junto.

## Decisões tomadas em conversa

- **Chat do Estus fica**, mas por baixo quem responde é o agente externo.
- **Um motor só: "Agente externo".** Saem Claude Code, Codex, Anthropic,
  OpenAI, Ollama e Apple. Cada agente já escolhe o próprio modelo; manter os
  motores seria duplicar isso.
- **Voz pelo navegador.** Nem Hermes nem OpenClaw aceitam áudio pela API de
  chat (só texto e imagem), então o ditado e a leitura em voz alta passam
  para as APIs do próprio navegador.
- **Avisos ficam com o agente.** O Telegram era o único canal de push
  (resumo da manhã e da noite, alerta de lembrete e de evento). Hermes e
  OpenClaw têm tarefas agendadas e canais próprios (WhatsApp, Telegram…);
  uma tarefa neles consulta o Estus pelo MCP e avisa. O Estus não ganha
  código para isso, só documentação.
- **Código morto sai.** Depois das remoções, uma varredura apaga o que
  ficou sem chamador.

## O que os agentes oferecem (verificado nas docs em 2026-09-23)

| | Hermes Agent | OpenClaw |
|---|---|---|
| Endpoint | `POST /v1/chat/completions` | `POST /v1/chat/completions` (desligado por padrão no Gateway) |
| Porta padrão | `127.0.0.1:8642` | a do Gateway |
| Auth | `Authorization: Bearer <API_SERVER_KEY>` | `Authorization: Bearer <token>` |
| Streaming | SSE | SSE |
| Imagem na mensagem | `image_url` (URL ou `data:image/…`) | `image_url` |
| Outros arquivos | `400 unsupported_content_type` | não aceita |
| Áudio | não | não |
| `model` | nome livre (ex.: `hermes-agent`) | `openclaw/default` ou `openclaw/<agentId>` |

Fontes: <https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server/>,
<https://docs.openclaw.ai/gateway/openai-http-api>.

## 1. O que sai

- **Telegram:** pacote `backend/internal/telegram/`, `httpapi/handlers_telegram.go`,
  `store/postgres/telegram_repo.go`, rotas, `TELEGRAM_BOT_TOKEN`,
  `frontend/components/assistant/TelegramSettings.tsx`, tipos e CSS dele,
  seção do README. `SendRequest.SentAt` e `assistant/sendtime.go` existiam
  só para a entrega atrasada do Telegram e saem também.
- **Migração `0029_drop_telegram`:** apaga as tabelas criadas em 0016/0017.
  O `down` recria o esquema vazio (sem dados).
- **Ponte Apple:** pasta `apple-bridge/`, `assistant/bridge.go`,
  `provider_apple.go`, `voice.go`, os handlers `transcribe`/`speak` e suas
  rotas, `APPLE_BRIDGE_BIN`, `APPLE_BRIDGE_URL`, `ASSISTANT_VOICE`, textos de
  erro da Apple em `explain.go`, seção "Voz" do README.
- **Motores:** `providers_cli.go`, `providers_api.go` (os helpers HTTP que a
  geração de imagem usa mudam para `httpjson.go`), `routing.go` e o laço de
  ferramentas do chat (o agente recebe as ferramentas pelo MCP, não pelo
  Estus). Migração `0030_assistant_agent`: apaga as chaves salvas, as colunas
  `models` e `ollama_url`, e `session_id` das conversas (só os motores de
  linha de comando usavam). `ASSISTANT_MULTI_USER` sai.
- **Frontend de voz:** `useVoiceRecorder.ts` e `wav.ts`, trocados pela versão
  do navegador.
- **Varredura final:** `go vet`, `staticcheck`/`deadcode` no backend e `tsc`
  + `eslint` + busca de exports sem uso no frontend. Tudo sem chamador sai,
  incluindo testes das partes removidas.

## 2. O que fica

- **Registro de ferramentas (`registry.go`, `tools_*.go`)**, servido pelo MCP
  (`/mcp`, HTTP, token) e pelo relay stdio `cmd/estus-mcp`. É por aqui que o
  agente lê e grava no Estus.
- **Geração de imagem (`tools_images.go`)** continua: OpenAI se houver
  `OPENAI_API_KEY` no ambiente, senão Draw Things local. A chave deixa de
  poder ser salva pela tela, já que a tela de chaves sai.
- **Conversas salvas no Estus** (tabelas de 0015), com histórico e título.

## 3. O motor novo

Um `Provider` chamado `agent` em `assistant/provider_agent.go`.

- **Configuração** (salva nas configurações do assistente, com fallback em
  env `AGENT_URL`, `AGENT_TOKEN`, `AGENT_MODEL`):
  - endereço base, ex. `http://localhost:8642`;
  - token, guardado cifrado com a mesma chave do cofre;
  - modelo/agente, ex. `hermes-agent` ou `openclaw/default`. Vazio usa o
    primeiro que o agente lista em `/v1/models`.
- **Envio:** `POST {endereço}/v1/chat/completions` com `stream: true`,
  `model`, e `messages` = mensagem de sistema do Estus (módulo atual e data)
  + histórico salvo + mensagem nova. Sem sessão do lado do agente: o Estus
  manda a conversa inteira a cada vez, então nada duplica e trocar de
  agente no meio não quebra.
- **Imagem anexada** vai como parte `image_url` com `data:` URL.
- **Outro tipo de anexo** segue como hoje: PDF, planilha e texto viram texto
  dentro da mensagem (`attachments.go`), então o agente recebe o conteúdo
  sem precisar aceitar arquivos.
- **Resposta:** cada `delta.content` do SSE vira um evento `text`, como hoje.
  Eventos que não são do formato OpenAI (ex. `hermes.tool.progress`) são
  ignorados.
- **Erros:**
  - sem endereço configurado: "Configure o agente em Ajustes do chat.";
  - conexão recusada ou timeout: "Agente não respondeu em <endereço>.";
  - 401/403: "O agente recusou o token.";
  - outro status: "O agente respondeu <status>." com o corpo em "detalhes
    técnicos".
  - erro dentro de uma resposta 200 (um chunk com `error`, ou o corpo JSON
    único de um agente que ignora `stream: true`): "O agente devolveu um
    erro: <mensagem>.".
- **Capacidades:** visão sim, geração de imagem pelo lado do agente, voz
  pelo navegador.

## 4. Voz no navegador

- **Ditado:** `SpeechRecognition` (`webkitSpeechRecognition`) em `pt-BR`. O
  texto reconhecido é enviado na hora, e a resposta é lida em voz alta (como
  no fluxo de voz anterior). Sem suporte no navegador, o botão do microfone
  não aparece.
- **Ouvir resposta:** `speechSynthesis` com a primeira voz `pt-BR`
  disponível.
- Nota na tela de ajustes: no Chrome o ditado passa pelo servidor do Google.

## 5. Tela de ajustes

`EngineSettings.tsx` passa a ter duas abas:

- **Agente:** endereço, token, modelo, botão "Testar conexão". Os ajustes
  já consultam `GET {endereço}/v1/models` com o token; o botão recarrega e
  mostra "Conectado" ou o erro. Quando o agente lista modelos, o campo vira
  uma lista para escolher.
- **MCP:** endereço e token do MCP do Estus para copiar, com exemplos de
  configuração para Hermes e OpenClaw no lugar dos exemplos de Claude Code,
  Codex e Desktop.

## 6. Documentação

README ganha "Conectando um agente", com:

1. ligar a API do agente (Hermes: `API_SERVER_KEY`; OpenClaw: habilitar
   `chatCompletions` no Gateway);
2. apontar o MCP do agente para `http://localhost:<porta>/mcp` com o token;
3. preencher a aba Agente no Estus e testar;
4. criar no agente uma tarefa agendada que chama as ferramentas de
   lembretes e agenda e manda os avisos no canal dele.

## Testes

- `provider_agent_test.go` contra um servidor falso (`httptest`) no formato
  OpenAI: streaming de texto, imagem anexada virando `image_url`, 401,
  conexão recusada, evento desconhecido ignorado, modelo vazio usando o
  primeiro de `/v1/models`.
- `Status` do motor contra o mesmo servidor falso (o botão "Testar conexão"
  só recarrega os ajustes, que chamam `Status`).
- `go test ./...`, `npm run lint`, `npx tsc --noEmit` passando depois da
  varredura.
- Não há teste contra Hermes ou OpenClaw reais: depende de o usuário ter um
  deles rodando.

## Fora do escopo

- Push de avisos pelo próprio Estus (web push, e-mail).
- Mostrar no chat as ferramentas que o agente chamou.
- Sessão/memória do lado do agente (`X-Hermes-Session-Id`, `user`).
