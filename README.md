# 🚀 Antigravity Telegram Remote (Go Port)

<p align="center">
  <pre>
    ___     _   _  _____  ___  ____  ___     _  _ 
   / _ \   | \ | ||_   _||_ _||  _ \|_ _|   | \/ |
  / /_\ \  |  \| |  | |   | | | |_) || |    |    |
 / /   \ \ | |\  |  | |   | | |  _ < | |    | || |
/_/     \_\|_| \_|  |_|  |___||_| \_\|___|  |_||_|
  </pre>
  <strong>Controle seu assistente de programação Antigravity IDE diretamente pelo Telegram.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-%3E%3D1.22-blue?style=flat-square&logo=go" alt="Go version" />
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform" />
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License" />
</p>

---

O **Antigravity Telegram Remote** é uma ferramenta de ponte e controle remoto local desenvolvida em **Go**, projetada para conectar o mensageiro **Telegram** à sua IDE **[Antigravity](https://antigravity.dev)** através do **Chrome DevTools Protocol (CDP)**.

Com ele, você pode enviar comandos por texto natural, imagens, fotos ou mensagens de voz, monitorar a execução de tarefas em tempo real, aprovar edições de código e receber notificações de status — mantendo seu código e dados 100% seguros na sua máquina local.

---

## 🌟 Principais Recursos

* **🕹️ Controle Remoto Completo:** Envie prompts, mensagens e instruções pelo Telegram para o Antigravity executar localmente com acesso total ao terminal e arquivos.
* **📂 Gerenciamento de Projetos e Workspaces:** Cada projeto é mapeado dinamicamente para tópicos de fórum no Telegram (**Forum Topics**). Mudar de tópico muda automaticamente o contexto e histórico do workspace correspondente.
* **⏱️ Streaming e Acompanhamento em Tempo Real:** Acompanhe o raciocínio (*thinking*), alterações de arquivos e execução de comandos com atualizações periódicas editadas no mesmo balão de mensagem.
* **🎙️ Mensagens de Voz (Whisper Local):** Envie mensagens de áudio pelo Telegram que são transcritas localmente via `whisper.cpp`, sem envio de dados para APIs na nuvem.
* **🖼️ Envio e Captura de Imagens:** Suporte para envio de fotos/diagramas anexados aos prompts e captura instantânea da tela da IDE com `/screenshot`.
* **✅ Aprovações Remotas:** Botões inline dinâmicos para aprovar (`Allow`), aprovar uma vez (`Allow Once`) ou negar (`Deny`) modificações de arquivos solicitadas pelo agente.
* **⚡ Alta Performance em Go:** Binário nativo compilado sem runtime Node.js, com baixo consumo de memória, concorrência segura com goroutines e banco SQLite local em modo WAL.

---

## 📋 Pré-requisitos

1. **Go 1.22+** instalado ([golang.org](https://golang.org)).
2. **IDE Antigravity** instalada no sistema.
3. **Conta no Telegram** e um **Bot** criado via [@BotFather](https://t.me/BotFather).
4. *(Opcional)* **whisper.cpp** compilado caso deseje transcrição local de áudio.

---

## ⚙️ Configuração Inicial (Telegram & Bot)

### 1. Criar o Bot no Telegram
1. Abra o Telegram e procure por [@BotFather](https://t.me/BotFather).
2. Envie o comando `/newbot` e siga as instruções para definir o nome e username do seu bot.
3. Copie o **HTTP API Token** gerado (exemplo: `123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ`).

### 2. Descobrir seu User ID do Telegram
Para garantir que somente você tenha acesso ao bot, configure seu User ID na whitelist:
1. Abra o bot [@userinfobot](https://t.me/userinfobot) ou [@RawDataBot](https://t.me/RawDataBot).
2. O bot responderá com o seu ID numérico (exemplo: `123456789`).

### 3. Configurar um Supergrupo com Tópicos (Recomendado)
Para aproveitar a separação por projetos em tópicos:
1. Crie um grupo no Telegram e adicione o seu bot como **Administrador** (com permissão de gerenciar tópicos).
2. Acesse as configurações do grupo e ative a opção **Tópicos** (Topics / Forum Mode).

---

## 🚀 Instalação e Compilação

Clone o repositório e compile o executável com Go:

```bash
cd antigravity-telegram-remote-golang

# Baixar dependências
go mod download

# Compilar o binário
go build -o antigravity-telegram-remote ./cmd/antigravity-bot
```

*(Opcional)* Para instalar o binário no seu `$GOPATH/bin` ou `/usr/local/bin`:

```bash
go install ./cmd/antigravity-bot
```

---

## 🛠️ Configuração do Ambiente

Execute o assistente interativo para gerar a configuração inicial:

```bash
./antigravity-telegram-remote setup
```

O assistente solicitará:
* **Telegram Bot Token**: Token fornecido pelo BotFather.
* **Telegram Allowed Users**: Seu Telegram User ID (para proteção por whitelist).
* **Workspace Base Directory**: Diretório raiz onde seus projetos de código residem (ex: `/Users/seu-usuario/Projetos`).
* **Porta CDP**: Porta para depuração (padrão `9222`).

As configurações são salvas em `~/.config/antigravity-telegram-remote/config.json` (ou `.env`).

---

## 🚦 Diagnóstico do Ambiente (Doctor)

Antes de iniciar, execute o comando de diagnóstico para verificar se todos os pré-requisitos estão saudáveis:

```bash
./antigravity-telegram-remote doctor
```

Para validar a integridade dos seletores DOM com a versão atual da IDE:
```bash
./antigravity-telegram-remote doctor --selectors
```

---

## 💻 Inicialização

O funcionamento divide-se em dois passos:

### 1. Abrir o Antigravity com Depuração CDP Habilitada
Abra a IDE com a porta CDP ativa:

```bash
./antigravity-telegram-remote open
```
> O comando detectará automaticamente o executável do Antigravity no seu sistema operacional (macOS, Linux ou Windows) e iniciará a instância com a flag de porta de depuração remota habilitada.

### 2. Iniciar o Bot Telegram
Em um terminal (ou serviço em segundo plano):

```bash
./antigravity-telegram-remote start
```

Use `--verbose` caso queira inspecionar mensagens detalhadas de tráfego CDP e requisições:
```bash
./antigravity-telegram-remote start --verbose
```

---

## 📱 Guia de Uso no Telegram

### Comandos Disponíveis no Chat

| Comando | Descrição |
| :--- | :--- |
| `/help` | Exibe a lista completa de comandos e instruções de uso. |
| `/project` | Lista todos os projetos disponíveis no Workspace com teclado inline para vincular ao tópico/chat atual. |
| `/status` | Exibe o status da conexão CDP, workspace ativo, modelo de IA e modo atual. |
| `/model [nome]` | Altera o modelo de linguagem utilizado (ex: `gemini-2.5-pro`, `claude-3.5-sonnet`) ou abre o menu de seleção interativo. |
| `/mode [fast\|plan]` | Alterna entre o modo de execução rápida (`Fast`) ou modo planejamento detalhado (`Plan`). |
| `/autoaccept [on\|off]` | Ativa ou desativa a auto-aprovação de ferramentas e edições de código. |
| `/screenshot` | Captura uma imagem da tela da janela da IDE Antigravity e envia no Telegram. |
| `/stop` | Interrompe imediatamente a execução do prompt/tarefa atual do assistente. |
| `/template` | Gerencia templates de prompts salvos (`/template list`, `/template add`, `/template del`). |
| `/cleanup` | Limpa registros de sessões e tópicos inativos há mais de 7 dias. |

### Interações Naturais

* **Enviar um Prompt de Texto:** Simplesmente envie a sua dúvida ou tarefa como mensagem normal no Telegram (ou no tópico do projeto desejado).
* **Enviar Imagens:** Envie uma ou mais fotos com a descrição na legenda. O bot converterá e injetará as imagens diretamente no painel do assistente.
* **Mensagens de Áudio:** Envie uma mensagem de voz. O áudio será transcrito e o texto resultante será despachado como prompt automaticamente.
* **Aprovações Interativas:** Se o agente solicitar permissão para alterar arquivos ou executar comandos no terminal, botões inline aparecerão no Telegram:
  * **[✅ Permitir / Allow]**: Concede permissão para a ação pontual.
  * **[🔄 Permitir Esta Conversa]**: Concede permissão contínua durante a sessão.
  * **[❌ Negar / Deny]**: Rejeita a solicitação.

---

## 🧪 Testes e Qualidade de Código

Para executar toda a suíte de testes unitários e de integração com verificação de corrida de dados (*race detector*):

```bash
# Rodar todos os testes com -race
go test -v -race ./...

# Análise estática
go vet ./...

# Verificação de formatação
gofmt -l .
```

---

## 📄 Licença

Distribuído sob a licença [MIT](LICENSE).
