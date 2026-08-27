# ⚡ ZENITH SERVER MANAGER — TOTVS PROTHEUS AGENTLESS MONITOR

Solução completa, ultraleve e de alta performance em Go (Golang) para monitoramento **agentless** de servidores Windows rodando TOTVS Protheus, com ingestão de métricas e eventos no TSDB **VictoriaMetrics** e **dashboard web moderna nativa**.

---

## 🚀 Destaques da Arquitetura

- **100% Agentless:** Não instala agentes, DLLs ou executáveis no servidor TOTVS Protheus. Utiliza apenas protocolos nativos do Windows (**WinRM/WMI** e **SMB2**).
- **Backend em Go Estático:** Binário estático puro (`CGO_ENABLED=0`) de alto desempenho e consumo ínfimo de memória (~15MB RAM).
- **TSDB VictoriaMetrics:** Armazenamento analítico de séries temporais de alta densidade e compressão através de HTTP POST (Prometheus Exposition format).
- **Buffer Resiliente Anti-OOM:** Fila em memória com anel finito (*bounded buffer*) que garante proteção estrita contra *memory leak* mesmo em cenários de queda prolongada do banco de dados TSDB.
- **Segurança Reforçada:** Sanitização rigorosa contra *command injection* em consultas WMI/PowerShell, headers de segurança (CSP, X-Frame-Options, HSTS, etc.) e autenticação configurável (Basic Auth / Bearer Token).
- **Dashboard Web Embutida:** Interface responsiva em Dark Mode com acentos em Laranja Protheus (`#FF8C00`), gráficos vetoriais SVG dinâmicos e Fetch API em tempo real. **Zero React, Vue, Angular, Node.js ou tags `<canvas>`.**
- **Modo Mock / Emulador Embutido:** Permite executar, testar e homologar toda a esteira sem dependência imediata de um servidor Windows ativo.

---

## 🛠️ Stack Tecnológica

| Componente | Tecnologia | Detalhes |
|---|---|---|
| **Linguagem Backend** | Go 1.24+ | Binário estático compilado sem CGO (`CGO_ENABLED=0`) |
| **TSDB (Métricas)** | VictoriaMetrics `v1.98.0` | Ingestão Prometheus Exposition via HTTP POST |
| **Frontend** | HTML5, CSS Moderno, Vanilla JS | Gráficos em SVG dinâmico puro, Fetch API assíncrono |
| **Coleta Remota** | WinRM (`masterzen/winrm`), SMB2 (`hirochachacha/go-smb2`) | Telemetria do SO, processos e tail de logs |
| **Containerização** | Docker & Docker Compose | Imagem mínima `alpine:3.21.3` com usuário não-root |
| **CI/CD** | GitHub Actions (`.github/workflows/build-image.yml`) | Gatilho estrito disparado **somente** na branch `main` |

---

## 📂 Estrutura do Projeto

```
├── .github/
│   └── workflows/
│       └── build-image.yml         # CI/CD automatizado estritamente na branch main
├── cmd/
│   └── server/
│       └── main.go                 # Ponto de entrada, graceful shutdown e orquestração
├── config.yaml                     # Arquivo de configuração padrão
├── docker-compose.yml              # Orquestração do VictoriaMetrics e Zenith Monitor
├── Dockerfile                      # Build multi-stage enxuto e seguro
├── go.mod / go.sum                 # Dependências versionadas e travadas
├── internal/
│   ├── api/                        # Servidor HTTP, rotas REST e middlewares de segurança
│   ├── collector/                  # WinRM, SMB2 tailing, TCP healthchecks, Mock e Sanitizer
│   ├── config/                     # Carregamento de YAML e variáveis de ambiente
│   ├── dashboard/                  # Assets estáticos (HTML/CSS/JS) embutidos via embed.FS
│   ├── integration_test.go         # Testes de integração ponta a ponta e resiliência
│   └── tsdb/                       # Ingestor com bounded buffer e cliente VictoriaMetrics
```

---

## ⚙️ Configuração (`config.yaml`)

```yaml
server:
  listen_addr: ":8080"
  auth_username: "admin"
  auth_password: "zenith@2026"
  auth_token: ""

target:
  host: "192.168.1.100"                      # IP do Servidor Windows Protheus
  winrm_port: 5985                           # 5985 (HTTP) ou 5986 (HTTPS)
  winrm_https: false                         # Habilita criptografia TLS no WinRM
  winrm_insecure: true                       # Permite certificados autoassinados
  username: "Administrator"                  # Usuário com permissão WMI/WinRM e SMB
  password: "SuaSenhaSegura"
  domain: ""                                 # Domínio AD (opcional)
  smb_share: "C$"                            # Compartilhamento do disco de logs
  appserver_log: "totvs/protheus/bin/appserver/console.log"
  dbaccess_log: "totvs/dbaccess/dbaccess.log"
  tcp_ports:
    - name: "AppServer - Principal"
      port: 1234
    - name: "DBAccess - TopConnect"
      port: 7890
    - name: "TOTVS License Server"
      port: 5555

tsdb:
  url: "http://victoriametrics:8428"
  batch_size: 250
  flush_interval: 2s
  max_buffer_size: 25000                     # Buffer finito anti-memory leak

collector:
  interval: 5s                               # Frequência de pooling
  timeout: 10s
  mock_mode: false                           # false = Produção | true = Emulação
  monitored_procs:
    - "appserver.exe"
    - "dbaccess.exe"
```

### Variáveis de Ambiente Suportadas

Todas as propriedades podem ser sobrescritas por variáveis de ambiente:
- `ZENITH_LISTEN_ADDR` / `ZENITH_SERVER_PORT`
- `ZENITH_AUTH_USERNAME` / `ZENITH_AUTH_PASSWORD` / `ZENITH_AUTH_TOKEN`
- `ZENITH_TARGET_HOST` / `ZENITH_TARGET_WINRM_PORT` / `ZENITH_TARGET_USERNAME` / `ZENITH_TARGET_PASSWORD`
- `ZENITH_TARGET_SMB_SHARE` / `ZENITH_TARGET_APPSERVER_LOG` / `ZENITH_TARGET_DBACCESS_LOG`
- `ZENITH_TSDB_URL`
- `ZENITH_MOCK_MODE` (`true` ou `false`)

---

## 🏃 Como Executar

### 1. Executando Localmente com Docker Compose

```bash
# Iniciar a stack completa (VictoriaMetrics + Zenith Monitor)
docker compose up -d

# Visualizar logs em tempo real
docker compose logs -f zenith-monitor
```

Acesse o Dashboard em seu navegador:
👉 **[http://localhost:8080](http://localhost:8080)**  
- **Usuário padrão:** `admin`  
- **Senha padrão:** `zenith@2026`

### 2. Executando Diretamente via Go

```bash
# Executar a suíte de testes
go test -v ./...

# Compilar binário estático
CGO_ENABLED=0 go build -ldflags="-s -w" -o zenith-server ./cmd/server

# Rodar a aplicação
./zenith-server -config config.yaml
```

---

## 🔒 Segurança e Resiliência Testadas

1. **Anti-Injeção PowerShell/WMI:** O pacote `internal/collector/sanitizer.go` rejeita metacaracteres perigosos (`;`, `&`, `|`, `` ` ``, `$()`, etc.) antes da montagem de comandos WinRM.
2. **Buffer Finito de Métricas:** Durante indisponibilidades do VictoriaMetrics, o `BoundedBuffer` mantém uso estrito de memória constante e descarta métricas antigas (FIFO) registrando contadores de descarte.
3. **Reconexão com Backoff Exponencial:** Quedas intermitentes de rede no WinRM ou no SMB2 sofrem tentativas automáticas com jitter e backoff progressivo.
