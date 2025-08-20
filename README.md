# Rinha de Backend

Backend de alta performance desenvolvido em Go para a **Rinha de Backend**, implementando um sistema de pagamentos com arquitetura distribuída e otimizações para alta concorrência.

## 🛠️ Tecnologias

<div align="left" >

<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/go/go-original-wordmark.svg" width="60" height="60" />
<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/postgresql/postgresql-original-wordmark.svg" width="60" height="60" />
<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/redis/redis-original-wordmark.svg" width="60" height="60" />
<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/docker/docker-original-wordmark.svg" width="60" height="60" />
<img src="https://cdn.jsdelivr.net/gh/devicons/devicon/icons/nginx/nginx-original.svg" width="60" height="60" />

</div>

## 🏗️ Arquitetura

- **🔧 Go 1.22** - Linguagem principal com otimizações de memória e GC
- **🗄️ PostgreSQL 16** - Banco de dados principal com pool de conexões
- **⚡ Redis 7** - Cache em memória para alta performance
- **🐳 Docker Compose** - Orquestração de serviços
- **🌐 Nginx** - Load balancer e proxy reverso
- **📊 Health Checks** - Monitoramento de saúde dos serviços

## 🎯 Estratégia de Performance

**Sistema de Buckets Circulares** com processamento assíncrono em lotes:

- **🔄 Sliding Window** - Janela deslizante de 600s com buckets circulares
- **⚡ Async Processing** - Queue de 200k itens + batch de 2k pagamentos
- **🚀 Zero Blocking** - Fluxo principal nunca espera I/O
- **📊 Batch Persistence** - COPY FROM PostgreSQL + Redis pipeline
- **🐳 Horizontal Scaling** - 2 instâncias + Nginx load balancer
- **🔧 Memory Optimization** - GOGC=200 + GOMEMLIMIT=512MiB

## 📁 Estrutura do Projeto

```
myRinhaGo/
├── 🐳 docker-compose.yml
├── 🐳 Dockerfile
├── 🐙 main.go
├── 📊 database/
├── 🎯 handlers/
├── 🏗️ models/
├── 📚 repositories/
├── ⚙️ services/
├── 🧪 tests/
├── 📜 sql/
└── 🛠️ scripts/
```

## 🌟 Destaques

- **Queue System** - Sistema de filas para processamento assíncrono
- **Batch Operations** - Operações em lote para otimizar I/O
- **Redis TTL** - Cache com expiração automática
- **Health Monitoring** - Verificação contínua de saúde dos serviços
- **Environment Configuration** - Configuração flexível via variáveis de ambiente

---

<div align="center">

**Desenvolvido com ❤️ em Go para a Rinha de Backend**

</div> 