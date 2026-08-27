# PROMPT DE COORDENAÇÃO MULTIAGENTE — PROTHEUS MONITOR AGENTLESS
**Repositório Alvo:** `https://github.com/robertocjunior/ZENITH-SERVER-MANAGER/`

**Meta:** Construir uma solução completa, ultraleve e de alta performance para monitoramento *agentless* de servidores Windows rodando TOTVS Protheus, armazenando métricas e eventos no VictoriaMetrics, com dashboard web nativa.

---

## 🌿 Política de Branches & Pipeline CI/CD

1. **Branch de Trabalho (Testes):**
   - Todo o desenvolvimento, refatoração e testes locais devem ser executados **estritamente na branch de testes** (ex: `feature/agentless-monitor` ou `testes`).
   - Durante a fase de testes na branch não-main, a imagem e os containers devem ser **construídos e testados localmente** (`docker compose build` / `docker build`).

2. **Workflow GitHub Actions (`.github/workflows/build-image.yml`):**
   - Criar workflow automatizado de CI/CD para compilar o binário em Go e gerar a imagem Docker.
   - **Gatilho estrito:** A publicação/build automatizado da imagem Docker deve ser acionado **APENAS quando houver push/merge na branch `main`**.

3. **Arquivos de Containerização:**
   - Criar `Dockerfile` multi-stage com Go estático (`CGO_ENABLED=0`) e imagem final enxuta (`alpine` ou `scratch`/`distroless` com `tzdata` e certificados).
   - Criar `docker-compose.yml` orquestrando a aplicação Go e a instância do VictoriaMetrics com volumes locais e redes isoladas.

---

## ⚙️ Diretrizes Operacionais & Restrições Técnicas

1. **Stack Tecnológica Estrita:**
   - **Backend:** 100% Go (Golang) compiled static binary (`CGO_ENABLED=0`).
   - **Banco de Dados (TSDB):** VictoriaMetrics (versão travada via Docker Compose).
   - **Frontend:** HTML5 puro, CSS moderno nativo e JavaScript vanilla com chamadas assíncronas (AJAX / Fetch API / EventSource / WebSocket). **PROIBIDO o uso de React, Vue, Angular, Node.js no build frontend, Tailwind CLI ou tags HTML `<canvas>`.**
   - **Design System do Dashboard:** Interface em Dark Mode profissional, com acentos em tom Laranja (`#FF8C00` / `#FF7A00`), tipografia limpa e layout responsivo focado em visualização técnica de métricas e status de serviços.

2. **Model Router & Eficiência de Tokens:**
   - Use **modelos mais leves/rápidos** (Flash / Haiku / Mini) para tarefas operacionais: templates HTML/CSS, workflows GitHub Actions, parsing de logs, testes unitários e documentação.
   - Use **modelos avançados/raciocínio** (Pro / Sonnet / Opus) **apenas quando estritamente necessário**: design de concorrência com Goroutines/Channels, integração SMB/WinRM complexa, análise de segurança e mitigação de vulnerabilidades.

3. **Arquitetura de Agentes (Até 4 Agentes Paralelos):**
   - **Agente 1 (Tech Lead & Security / CI-CD):** Gerencia o lock de dependências (`go.mod`, `go.sum`, imagens Docker fixas com tag SHA/versão sem `:latest`), cria as GitHub Actions (build restrito à `main`), executa linters (`golangci-lint`), varredura de segurança (`govulncheck`, `gosec`) e garante trabalho isolado na branch de testes.
   - **Agente 2 (Agentless Core Engine):** Implementa os módulos em Go de coleta remota: WinRM (telemetria de CPU, RAM, disco e processos Windows `appserver.exe`, `dbaccess.exe`), SMB2 (tail e parsing de `console.log` / `dbaccess.log`) e TCP healthchecks de portas Protheus.
   - **Agente 3 (TSDB & Pipelines):** Implementa o conector e ingestão de alta performance no VictoriaMetrics (Prometheus/Influx Line Protocol via HTTP POST com buffer/batching em memória) e consultas analíticas de séries temporais.
   - **Agente 4 (Web Dashboard & API):** Implementa o servidor HTTP nativo em Go (`net/http`), endpoints de métricas/alertas e o frontend single-file embutido (`embed.FS`) usando HTML5/CSS e AJAX/Fetch em tempo real.

4. **Travamento de Versões & Reprodutibilidade:**
   - Todas as dependências externas de Go (`go.mod`), imagens no `docker-compose.yml` (`victoriametrics/victoria-metrics:v1.98.0`, `golang:1.22-alpine`, etc.) e actions no GitHub Workflow devem ser **pinadas com versões exatas**.

---

## 📋 Protocolo de Execução Obrigatório

### Fase 1: Coleta de Dados do Servidor (PAUSA OBRIGATÓRIA)
Antes de rodar a bateria final de testes de integração, você **DEVE solicitar ao usuário** as credenciais e parâmetros do servidor de teste:
1. IP / Hostname do Servidor Windows Protheus.
2. Porta WinRM (padrão `5985` HTTP ou `5986` HTTPS).
3. Usuário e Senha de serviço com permissão de leitura remota (WMI/WinRM) e compartilhamento SMB (`C$`/pasta de logs).
4. Caminho UNC ou local dos logs (ex: `C$\totvs\protheus\bin\appserver\console.log`).
5. Lista de portas TCP dos serviços Protheus a monitorar (ex: `1234` AppServer, `7890` DBAccess, `5555` License Server).

*(Caso o usuário ainda não queira fornecer dados reais de imediato, implemente um modo de mock/emulador em Go para validar a suíte de testes locais na branch de testes antes de apontar para a infraestrutura real).*

---

### Fase 2: Implementação, Containers & Testes Rigorosos
1. **Ambiente Local:** Criar `Dockerfile` e `docker-compose.yml` funcionais para rodar os testes localmente na branch de testes.
2. **GitHub Actions:** Criar `.github/workflows/build-image.yml` configurado para disparar a geração da imagem final apenas em commits/merges na branch `main`.
3. **Testes Unitários e de Integração:**
   - Testar o coletor contra timeouts e desconexões de rede no WinRM/SMB com mecanismos de retry e backoff exponencial.
   - Testar o buffer de métricas para garantir que falhas temporárias no VictoriaMetrics não causem memory leak nem queda da aplicação.
4. **Validação de Segurança:**
   - Validar sanitização de inputs nos scripts WMI/PowerShell via WinRM para evitar command injection.
   - Garantir comunicação segura e controle de acesso aos endpoints da dashboard.
