const express = require('express');
const { Pool } = require('pg');

const app = express();
app.use(express.json());

// PostgreSQL connection pool
const pool = new Pool({
  host: process.env.POSTGRES_HOST || 'iiot_postgres',
  port: process.env.POSTGRES_PORT || 5432,
  database: process.env.POSTGRES_DB || 'iiot_platform',
  user: process.env.POSTGRES_USER || 'admin',
  password: process.env.POSTGRES_PASSWORD || '0039',
  max: 20,
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok', timestamp: new Date().toISOString() });
});

// EMQX Webhook - Telemetry endpoint
app.post('/api/telemetry', async (req, res) => {
  try {
    const { clientid, topic, payload, timestamp } = req.body;
    console.log('Received webhook:', JSON.stringify(req.body, null, 2));

    // Validate required fields
    if (!topic || !payload) {
      return res.status(400).json({ error: 'Missing required fields' });
    }

    // Extract slot from topic: devices/{token}/telemetry/slot/{N}
    const topicParts = topic.split('/');
    const deviceToken = topicParts[1];
    const slot = parseInt(topicParts[4]);

    if (isNaN(slot)) {
      return res.status(400).json({ error: 'Invalid slot number' });
    }

    // Find device by token
    const deviceResult = await pool.query(
      'SELECT id FROM devices WHERE token = $1 AND status = $2',
      [deviceToken, 'active']
    );

    if (deviceResult.rows.length === 0) {
      return res.status(404).json({ error: 'Device not found or inactive' });
    }

    const deviceId = deviceResult.rows[0].id;

    // Insert telemetry
    await pool.query(
      'INSERT INTO telemetry (device_id, slot, value, timestamp) VALUES ($1, $2, $3, $4)',
      [deviceId, slot, payload, timestamp ? new Date(Number(timestamp)).toISOString() : new Date().toISOString()]
    );

    res.status(201).json({ 
      success: true, 
      device_id: deviceId, 
      slot 
    });

  } catch (error) {
    console.error('Telemetry error:', error);
    res.status(500).json({ 
      error: 'Internal server error', 
      details: error.message 
    });
  }
});

const PORT = process.env.PORT || 3000;
app.listen(PORT, '0.0.0.0', () => {
  console.log(`API running on port ${PORT}`);
});