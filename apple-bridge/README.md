# estus-apple-bridge

Helper macOS que expõe o Apple Intelligence (framework on-device `FoundationModels`) via HTTP local para o Estus Brain.
Escuta só em `127.0.0.1`. Endpoints: `GET /health`, `POST /chat` (com ferramentas dinâmicas chamadas no backend),
`POST /transcribe` (áudio → texto em pt-BR, on-device; header `X-Filename` com a extensão) e `POST /speak`
(`{"text", "voice"}` → áudio AAC com a voz do sistema). A voz não depende do Apple Intelligence estar ligado.

Requisitos: macOS 26+, Apple Silicon e **Apple Intelligence ativado** em Ajustes do Sistema > Apple Intelligence e Siri.

```sh
cd apple-bridge
swift build -c release
APPLE_BRIDGE_PORT=8765 .build/release/estus-apple-bridge   # porta padrão: 8765
curl http://127.0.0.1:8765/health
```

Variável de ambiente: `APPLE_BRIDGE_PORT` (padrão `8765`). Logs vão para stderr.
Limites: corpo de até 1 MB (10 MB em `/transcribe`); áudio de até 3 minutos; janela de contexto do modelo ~4k tokens (histórico é truncado para ~3000 caracteres).
Testes: `swift test` (usa `say` e o modelo de transcrição pt-BR, baixado na primeira vez).
Se só tiver as Command Line Tools instaladas (sem Xcode.app), `swift test` sozinho compila mas roda 0 testes em
silêncio — o binário de teste não acha `Testing.framework` nem no `import` nem em tempo de execução. Nesse caso rode:

```sh
F=/Library/Developer/CommandLineTools/Library/Developer/Frameworks
swift test -Xswiftc -F -Xswiftc "$F" -Xlinker -F -Xlinker "$F" \
  -Xlinker -rpath -Xlinker "$F" -Xlinker -rpath -Xlinker "$F/../usr/lib"
```
