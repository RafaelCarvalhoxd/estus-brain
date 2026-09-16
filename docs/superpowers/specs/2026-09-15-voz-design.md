# Voz no Estus Brain — design

Data: 2026-09-15 · Status: aprovado em conversa, aguardando revisão do spec

## Objetivo

Conversar com o Estus por voz:

- **Chat web:** clicar no microfone, falar, e receber a resposta em texto **e**
  lida em voz alta.
- **Telegram:** mandar áudio; o Estus entende, executa as ações e responde
  **só em texto**.

Tudo processado no Mac, sem mandar áudio para fora.

## Decisões

- **Voz por turnos** (clicar para falar), não conversa em tempo real.
- **Motor de voz local, da Apple:** transcrição on-device (`SpeechAnalyzer` /
  `SpeechTranscriber`, pt-BR) e síntese com a voz do sistema (padrão
  `Luciana`). Grátis, privado, offline. Num servidor sem macOS a voz fica
  indisponível e o resto funciona.
- **Abordagem:** a fala mora no `apple-bridge` (que o backend já inicia sob
  demanda); o backend Go expõe rotas de voz para o front e reusa a
  transcrição no bot do Telegram. Descartados: reconhecimento do próprio
  navegador (no Chrome o áudio vai para o Google) e chamar `say`/um CLI Swift a
  cada pedido.
- **Envio automático:** no web a transcrição é enviada assim que a gravação
  termina, sem etapa de edição.
- **Resposta falada só quando a pergunta foi por voz.** Pergunta digitada →
  só texto.
- Voz independe do motor de texto escolhido (Claude, Codex, Ollama, Apple,
  ou "Só atalhos").

## Viabilidade (spike descartável, 2026-09-15, macOS 26.5)

- `SpeechTranscriber` com locale `pt_BR`: suportado e instalado; transcreveu
  "Gastei quarenta e cinco reais no mercado hoje no Pix." corretamente em
  0,16–0,38 s, a partir de WAV e de CAF/Opus, num binário de linha de comando,
  sem pedido de permissão.
- `afconvert -hf` lista Ogg com `opus` entre os formatos lidos (formato das
  mensagens de voz do Telegram). A leitura de um `.ogg` real fica confirmada
  no teste ponta a ponta.
- `say -v Luciana` + AAC: ~0,7 s para uma frase, ~40 KB.
- `MediaRecorder` do Chrome grava WebM/Opus, que o AVFoundation não lê → o
  navegador converte para WAV antes de enviar.

## Componentes

### `apple-bridge` (Swift)

Novo arquivo `Speech.swift` e rotas em `Chat.swift` (`Routes.handle`):

- **`POST /transcribe`** — corpo: bytes do áudio; header `X-Filename` com a
  extensão (`.wav`, `.m4a`, `.caf`, `.ogg`, `.oga`, `.opus`, `.mp3`). Grava num
  arquivo temporário (apagado depois), lê com `AVAudioFile`, transcreve com
  `SpeechTranscriber(locale: pt_BR)`; se os assets não estiverem instalados,
  baixa via `AssetInventory` na primeira vez. Resposta
  `200 {"text": "...", "seconds": <duração do áudio>}`. Áudio sem fala →
  `{"text": ""}`. Duração > 180 s → `422 {"error": "Áudio longo demais — mande até 3 minutos."}`.
  Formato ilegível → `422 {"error": "Formato de áudio não suportado."}`.
- **`POST /speak`** — corpo JSON `{"text": "...", "voice": "Luciana"}`. Sintetiza
  com o `say` do sistema (que conhece pelo nome todas as vozes instaladas,
  inclusive as "Premium"), converte para AAC com `afconvert` e responde
  `200` com `Content-Type: audio/mp4`. Texto vazio → `422 "Nada para ler."`;
  voz não instalada → `422 "A voz <nome> não está instalada neste Mac."` (sem
  trocar de voz em silêncio). Texto recebido já vem limpo de Markdown e com no
  máximo 4000 caracteres (o backend garante).
- **Limite de corpo:** `HTTPLimits.maxBodyBytes` passa a valer por rota: 10 MB
  para `/transcribe`, 1 MB para as demais. `HTTPResponse` ganha um
  `contentType` (padrão JSON) para o áudio.
- `/health` não muda (reflete só o Apple Intelligence); voz não depende do
  Apple Intelligence estar ligado.

### Backend Go (`internal/assistant/voice.go`)

- **`Voice`**: cliente do bridge com `Transcribe(ctx, audio []byte, filename string) (Transcript, error)`
  e `Speak(ctx, text string) ([]byte, error)`, usando a mesma lógica de
  "inicia o bridge se não estiver no ar" do `appleProvider` (extraída para
  uma função compartilhada, não duplicada). `Status(ctx) VoiceStatus{Available bool, Detail string}`:
  disponível quando o binário do bridge existe e ele responde.
- **`SpeakableText(md string) string`**: remove Markdown (negrito, itálico,
  títulos `#`, marcadores de lista, blocos de código viram "código omitido",
  links viram só o texto), junta espaços e corta em 4000 caracteres no fim de
  uma frase.
- Voz configurável por `ASSISTANT_VOICE` (padrão `Luciana`).
- **Rotas** (em `handlers_assistant.go`, cobertas pelo proxy do Next):
  - `POST /api/assistant/voice/transcribe` — corpo bruto (limite 10 MB),
    header `X-Filename`. `200 {"text", "seconds"}`; erros do bridge → `422
    {"error"}` com a frase; bridge indisponível → `503 {"error": "A voz não
    está disponível nesta máquina."}`.
  - `POST /api/assistant/voice/speak` — `{"text"}`. `200 audio/mp4`;
    mesmos erros.
  - `GET /api/assistant/settings` ganha `voice: {available, detail}`.
  - O proxy do Next (`app/api/assistant/[...path]/route.ts`) passa a repassar o
    header `X-Filename`.
- Logs: só duração e tempo de processamento; nunca o áudio nem o texto.

### Frontend (`components/assistant/`)

- **`useVoiceRecorder`** (hook novo, arquivo próprio): `getUserMedia` +
  `MediaRecorder`; `AnalyserNode` mede o volume para parar após 2 s de
  silêncio depois que houve fala; limite de 2 minutos; ao parar, decodifica
  com `AudioContext.decodeAudioData`, reamostra para 16 kHz mono com
  `OfflineAudioContext` e gera WAV PCM 16-bit (função pura `encodeWav`).
- **`ChatApp.tsx`**:
  - Botão 🎙️ no compositor, só quando `settings.voice.available`. Estados:
    parado → gravando (vermelho pulsando + cronômetro; clique para parar) →
    "Transcrevendo…".
  - Transcrição vazia → aviso "Não ouvi nada — tente de novo." (não envia).
  - Transcrição → mesmo caminho do texto digitado (`submit`), marcando o item
    do usuário com `voice: true` (mostra 🎙️ pequeno).
  - Quando a resposta a uma mensagem de voz termina (texto ou erro), chama
    `/voice/speak` com o texto (ou a explicação do erro) e toca; botão
    "■ Parar voz" enquanto toca. Nova gravação ou novo envio para a fala.
  - Permissão negada → "Permita o microfone para este site nas configurações
    do navegador."; falha de transcrição → caixa "Não deu certo" com a frase;
    falha ao sintetizar → nota "Não consegui ler em voz alta." abaixo da
    resposta.
- `EngineSettings.tsx`: linha "Voz" com `voice.detail` (ex.: "Voz do Mac
  (Luciana), offline" ou o motivo de estar indisponível).

### Telegram (`internal/telegram`)

- `Message` ganha `Voice *Voice` e `Audio *Audio` (`file_id`, `duration`,
  `mime_type`, `file_name`).
- `API` ganha `GetFile(ctx, fileID) (File, error)` e
  `DownloadFile(ctx, filePath string) ([]byte, error)` (URL
  `.../file/bot<token>/<path>`; erros sem a URL, como no resto do cliente;
  limite de 20 MB).
- `Config` ganha `Voice` (interface `Transcriber` com `Available(ctx) bool` e
  `Transcribe(ctx, audio, filename)`); `nil` = sem voz.
- Em `handle`: mensagem com `Voice`/`Audio` do dono →
  - sem voz disponível → "Por enquanto só entendo texto aqui.";
  - `duration > 180` → "Áudio longo demais — mande até 3 minutos.";
  - senão vai para a fila do worker de IA como um item de áudio. No worker:
    baixa, transcreve; vazio → "Não entendi o áudio — pode repetir?"; senão
    envia "🎙️ Entendi: <texto>" e segue exatamente como uma mensagem de texto
    (`ask`). Falha ao baixar/transcrever → "Não consegui ouvir o áudio agora.
    Tente de novo ou mande em texto."
- Fotos, documentos e outros continuam com "Por enquanto eu só entendo texto."
- A resposta no Telegram é sempre texto.

## Erros

| situação | web | Telegram |
|---|---|---|
| bridge não compilado / sem macOS | microfone some; motivo em Motor de IA e conexões | "Por enquanto só entendo texto aqui." |
| permissão de microfone negada | aviso para liberar no navegador | — |
| nada foi dito | "Não ouvi nada — tente de novo." | "Não entendi o áudio — pode repetir?" |
| áudio > limite | web corta em 2 min | "Áudio longo demais — mande até 3 minutos." |
| falha na transcrição | caixa "Não deu certo" | "Não consegui ouvir o áudio agora…" |
| falha ao sintetizar | texto fica + "Não consegui ler em voz alta." | — |

## Testes

- **Go:** `Voice` contra um bridge falso (`httptest`): transcrição, fala,
  erro 422 repassado, bridge antigo (404) e bridge fora → indisponível;
  `SpeakableText` (Markdown, corte em 4000 no fim de frase); as rotas HTTP
  são conferidas com `curl` pelo app rodando (transcrição, fala, formato
  inválido);
  Telegram com fakes: áudio transcrito vai para a IA e manda "🎙️ Entendi",
  áudio vazio, longo demais, voz indisponível, falha ao baixar.
- **Swift:** alvo de teste no pacote: gera áudio com `say -v Luciana`, chama a
  transcrição e confere que contém "mercado"; áudio vazio e formato inválido;
  `/speak` devolve bytes que o `AVAudioFile` abre, e recusa texto vazio, voz
  inexistente e corpo que não é JSON; limite de corpo por rota.
- **Frontend:** `tsc` e `eslint`; checagem no navegador: gravar, ver a
  mensagem com 🎙️, ouvir a resposta, parar a voz.
- **Ponta a ponta com o dono:** áudio real no Telegram ("gastei 30 reais de
  Uber") → "🎙️ Entendi" + lançamento feito.

## Fora do escopo

Conversa em tempo real, resposta em áudio no Telegram, escolher a voz pela
tela, voz na nuvem, transcrição com edição antes de enviar, outros idiomas.
