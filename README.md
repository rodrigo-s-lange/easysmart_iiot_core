# IIoT Platform

Plataforma Industrial IoT com MQTT, PostgreSQL e Redis.

## Serviços

- **PostgreSQL 16** (porta 5432): Banco de dados principal
- **Redis 7** (porta 6379): Cache e sessões
- **EMQX 5.5** (portas 1883, 8083, 18083): MQTT Broker

## Configuração

1. Copie `.env.example` para `.env` (se necessário)
2. Ajuste credenciais no `.env`
3. Inicie stack:
```bash
docker-compose up -d
```

## Acessos

- **EMQX Dashboard**: http://192.168.0.99:18083
  - Usuário: admin
  - Senha: (ver `.env`)

- **PostgreSQL**:
```bash
  docker exec -it iiot_postgres psql -U admin -d iiot_platform
```

- **Redis**:
```bash
  docker exec -it iiot_redis redis-cli -a SENHA
```

## Comandos Úteis
```bash
# Ver logs
docker-compose logs -f [serviço]

# Parar tudo
docker-compose down

# Parar e limpar volumes
docker-compose down -v

# Status
docker-compose ps
```

## Estrutura
```
iiot_platform/
├── docker-compose.yml
├── .env
├── .gitignore
└── README.md
```

## TODO

- [ ] Schemas PostgreSQL
- [ ] Next.js frontend
- [ ] Cloudflare Tunnel (produção)