-- ============================================
-- IIoT Platform - Initial Schema v1.0
-- Multi-tenant ready, Blynk-like architecture
-- ============================================

-- TABELA: users
-- Usuários da plataforma (pessoas físicas)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    plan VARCHAR(20) DEFAULT 'free' CHECK (plan IN ('free', 'premium')),
    max_devices INTEGER DEFAULT 5,
    retention_days INTEGER DEFAULT 30,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Índices para performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status) WHERE status = 'active';

-- Comentários
COMMENT ON TABLE users IS 'Usuários da plataforma IIoT';
COMMENT ON COLUMN users.plan IS 'Plano: free (5 devices, 30d) ou premium (ilimitado, 365d)';
COMMENT ON COLUMN users.max_devices IS 'Limite de dispositivos por plano';
COMMENT ON COLUMN users.retention_days IS 'Dias de retenção de telemetria';


CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token UUID UNIQUE NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    hw_type VARCHAR(50) DEFAULT 'generic',
    firmware_version VARCHAR(50),
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'offline', 'blocked')),
    last_seen TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Índices
CREATE INDEX idx_devices_user_id ON devices(user_id);
CREATE INDEX idx_devices_token ON devices(token);
CREATE INDEX idx_devices_status ON devices(status);
CREATE INDEX idx_devices_last_seen ON devices(last_seen DESC);

-- Comentários
COMMENT ON TABLE devices IS 'Dispositivos IoT vinculados aos usuários';
COMMENT ON COLUMN devices.token IS 'Token único para autenticação MQTT (username)';
COMMENT ON COLUMN devices.hw_type IS 'Tipo de hardware: ESP32-S3, ESP32, generic, etc';
COMMENT ON COLUMN devices.status IS 'active=online, offline=sem comunicação, blocked=rate limit';
COMMENT ON COLUMN devices.metadata IS 'Metadados livres (localização, descrição, etc)';

-- TABELA: device_slots
-- Configuração dos 256 slots por dispositivo (0-255)
CREATE TABLE device_slots (
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    slot_number SMALLINT NOT NULL CHECK (slot_number >= 0 AND slot_number <= 255),
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (device_id, slot_number)
);

-- Índices
CREATE INDEX idx_device_slots_device_id ON device_slots(device_id);
CREATE INDEX idx_device_slots_enabled ON device_slots(device_id) WHERE enabled = true;

-- Comentários
COMMENT ON TABLE device_slots IS 'Configuração dos 256 slots por dispositivo';
COMMENT ON COLUMN device_slots.slot_number IS 'Número do slot (0-255)';
COMMENT ON COLUMN device_slots.config IS 'Metadados JSON: {"type":"temperature","unit":"C","min":-10,"max":50}';
COMMENT ON COLUMN device_slots.enabled IS 'Se false, slot não processa telemetria';

-- TABELA: telemetry
-- Dados de telemetria dos dispositivos (particionada por mês)
CREATE TABLE telemetry (
    id BIGSERIAL,
    device_id UUID NOT NULL,
    slot SMALLINT NOT NULL,
    value JSONB NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Índices
CREATE INDEX idx_telemetry_device_slot ON telemetry(device_id, slot, timestamp DESC);
CREATE INDEX idx_telemetry_timestamp ON telemetry(timestamp DESC);

-- Comentários
COMMENT ON TABLE telemetry IS 'Telemetria dos dispositivos - particionada mensalmente para performance';
COMMENT ON COLUMN telemetry.value IS 'Valor em JSON: {"raw":23.5} ou {"status":"open","current":2.3}';
COMMENT ON COLUMN telemetry.timestamp IS 'Timestamp UTC da leitura';

-- TABELA: commands
-- Comandos enviados aos dispositivos
CREATE TABLE commands (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    slot SMALLINT NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'executed', 'failed', 'timeout')),
    response JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    executed_at TIMESTAMP
);

-- Índices
CREATE INDEX idx_commands_device_id ON commands(device_id, created_at DESC);
CREATE INDEX idx_commands_status ON commands(status) WHERE status = 'pending';

-- Comentários
COMMENT ON TABLE commands IS 'Comandos enviados aos dispositivos';
COMMENT ON COLUMN commands.payload IS 'Comando JSON: {"action":"turn_on","value":255}';
COMMENT ON COLUMN commands.response IS 'Resposta do device após execução';
COMMENT ON COLUMN commands.status IS 'pending→sent→executed ou failed/timeout';

-- TABELA: device_rate_limits
-- Rate limiting para prevenir spam/loops
CREATE TABLE device_rate_limits (
    device_id UUID PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    messages_last_minute INTEGER DEFAULT 0,
    messages_last_hour INTEGER DEFAULT 0,
    last_reset TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    blocked_until TIMESTAMP,
    block_reason TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Índices
CREATE INDEX idx_rate_limits_blocked ON device_rate_limits(device_id) WHERE blocked_until IS NOT NULL;

-- Comentários
COMMENT ON TABLE device_rate_limits IS 'Controle de rate limiting por dispositivo';
COMMENT ON COLUMN device_rate_limits.messages_last_minute IS 'Contador de mensagens no último minuto';
COMMENT ON COLUMN device_rate_limits.blocked_until IS 'NULL se não bloqueado, ou timestamp até quando fica bloqueado';
COMMENT ON COLUMN device_rate_limits.block_reason IS 'Motivo do bloqueio: "Exceeded 100 msg/min"';

-- ============================================
-- FUNÇÕES E TRIGGERS
-- ============================================

-- Função para atualizar updated_at automaticamente
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Aplica trigger em todas as tabelas com updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_devices_updated_at BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_device_slots_updated_at BEFORE UPDATE ON device_slots
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_rate_limits_updated_at BEFORE UPDATE ON device_rate_limits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comentário
COMMENT ON FUNCTION update_updated_at_column() IS 'Atualiza campo updated_at automaticamente em UPDATEs';

-- Função para criar partições de telemetria automaticamente
CREATE OR REPLACE FUNCTION create_telemetry_partition(partition_date TIMESTAMP)
RETURNS void AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    partition_name := 'telemetry_' || to_char(partition_date, 'YYYY_MM');
    start_date := date_trunc('month', partition_date);
    end_date := start_date + INTERVAL '1 month';
    
    -- Cria partição se não existir
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF telemetry
         FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_date, end_date
    );
    
    -- Cria índice específico da partição
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I (device_id, timestamp DESC)',
        partition_name || '_device_ts_idx', partition_name
    );
END;
$$ LANGUAGE plpgsql;

-- Cria partições para os próximos 12 meses
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..11 LOOP
        PERFORM create_telemetry_partition((CURRENT_DATE + (i || ' months')::INTERVAL)::TIMESTAMP);
    END LOOP;
END;
$$;

-- Comentário
COMMENT ON FUNCTION create_telemetry_partition(TIMESTAMP) IS 'Cria partição mensal para telemetry automaticamente';

