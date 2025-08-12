# 🐔 Infra Template - Rinha de Backend 2025

Este template contém toda a infraestrutura necessária para sua submissão na Rinha de Backend 2025.

## 📋 O que está incluído

- `docker-compose.yml`: 2 instâncias do seu backend + Nginx (porta 9999)
- `nginx.conf`: balanceamento de carga otimizado
- `info.json`: metadados da submissão (preencha com seus dados)
- Este README com instruções completas

## 🚀 Como usar

### 1. Subir os Payment Processors PRIMEIRO
Os Payment Processors criam a rede `payment-processor` que seu backend precisa.

```bash
# Na raiz do repositório da Rinha
cd payment-processor
docker compose up -d
```

Endpoints disponíveis:
- **Default** (taxa 5%): http://localhost:8001
- **Fallback** (taxa 15%): http://localhost:8002

### 2. Configurar sua imagem Docker
Edite `docker-compose.yml` e substitua:

```yaml
image: ghcr.io/SEU_USUARIO/SEU_BACKEND:TAG
```

Por sua imagem pública real.

### 3. Requisitos do seu backend
Seu backend deve:
- Escutar na porta **8080** internamente
- Implementar os endpoints:
  - `POST /payments`
  - `GET /payments-summary`
- Usar as variáveis de ambiente fornecidas:
  - `PAYMENT_PROCESSOR_DEFAULT_URL=http://payment-processor-default:8080`
  - `PAYMENT_PROCESSOR_FALLBACK_URL=http://payment-processor-fallback:8080`

### 4. Subir sua infraestrutura
```bash
# Neste diretório
docker compose up -d
```

Sua API estará em: http://localhost:9999

### 5. Testar localmente com k6
```bash
# Na pasta rinha-test do repositório
cd ../rinha-test

# Opcional: habilitar dashboard
export K6_WEB_DASHBOARD=true
export K6_WEB_DASHBOARD_EXPORT='report.html'

# Rodar os testes
k6 run rinha.js

# Ou com limite de requisições
k6 run -e MAX_REQUESTS=550 rinha.js
```

## 📊 Limites de recursos (já configurados)

| Serviço | CPU | Memória |
|---------|-----|---------|
| nginx   | 0.10 | 20MB   |
| app-1   | 0.60 | 160MB  |
| app-2   | 0.60 | 160MB  |
| **Total** | **1.30** | **340MB** |

✅ Dentro dos limites: 1.5 CPU e 350MB

## 🎯 Endpoints que você deve implementar

### POST /payments
```json
// Request
{
  "correlationId": "4a7901b8-7d26-4d9d-aa19-4dc1c7cf60b3",
  "amount": 19.90
}

// Response: HTTP 2XX (qualquer coisa)
```

### GET /payments-summary
```json
// Request: ?from=2020-07-10T12:34:56.000Z&to=2020-07-10T12:35:56.000Z

// Response
{
  "default": {
    "totalRequests": 43236,
    "totalAmount": 415542345.98
  },
  "fallback": {
    "totalRequests": 423545,
    "totalAmount": 329347.34
  }
}
```

## 🔧 Endpoints dos Payment Processors

### POST /payments (para integração)
```json
{
  "correlationId": "4a7901b8-7d26-4d9d-aa19-4dc1c7cf60b3",
  "amount": 19.90,
  "requestedAt": "2025-07-15T12:34:56.000Z"
}
```

### GET /payments/service-health (estratégia)
```json
{
  "failing": false,
  "minResponseTime": 100
}
```
⚠️ **Rate limit**: 1 chamada a cada 5 segundos!

## 📝 Submissão final

### 1. Preencher info.json
```json
{
  "name": "Seu Nome",
  "social": ["https://github.com/seu-usuario"],
  "source-code-repo": "https://github.com/seu-usuario/seu-repo",
  "langs": ["python", "go", "etc"],
  "storages": ["postgresql", "redis", "etc"],
  "messaging": ["rabbitmq", "etc"],
  "load-balancers": ["nginx"],
  "other-technologies": ["fastapi", "etc"]
}
```

### 2. Criar PR no repositório oficial
1. Fork o repositório da Rinha
2. Copie esta pasta para `participantes/seu-nome/`
3. Abra um PR

⚠️ **IMPORTANTE**:
- Não inclua código-fonte no PR
- Não inclua logs
- Apenas os 4 arquivos: `docker-compose.yml`, `nginx.conf`, `README.md`, `info.json`
- Prazo: **17/08/2025 às 23:59:59**

## 🏆 Dicas para pontuação

### Maximizar lucro
- Use o **default** sempre que possível (taxa 5% vs 15%)
- Monitore health-checks para detectar falhas
- Implemente fallback inteligente

### Performance (bônus)
- p99 < 11ms = bônus de 2% por ms
- Exemplo: p99 de 5ms = 12% de bônus

### Evitar multas
- Garanta consistência no `/payments-summary`
- Inconsistência = multa de 35% do lucro

## 🐛 Troubleshooting

**Erro de rede "payment-processor not found"**
- Certifique-se que os Payment Processors estão rodando primeiro

**HTTP 429 no health-check**
- Respeite o limite de 1 chamada a cada 5 segundos

**Falha no teste k6**
- Verifique se a API está respondendo em http://localhost:9999
- Teste manualmente: `curl http://localhost:9999/payments-summary`

**Limites de CPU/RAM**
- Ajuste os valores em `deploy.resources.limits` se necessário
- Total não pode passar de 1.5 CPU e 350MB

## 📚 Links úteis

- [Instruções completas](../INSTRUCOES.md)
- [Repositório dos Payment Processors](https://github.com/zanfranceschi/rinha-de-backend-2025-payment-processor)
- [Documentação do k6](https://k6.io/docs/)
- [Discord da Rinha](https://discord.gg/Eca6gJba8R)

Boa sorte! 🚀 