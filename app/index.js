const express = require('express');
const { Pool } = require('pg');

const app = express();
app.use(express.json());

const pool = new Pool({
  host: process.env.POSTGRES_HOST || 'postgres',
  port: process.env.POSTGRES_PORT || 5432,
  database: process.env.POSTGRES_DB || 'iiot_platform',
  user: process.env.POSTGRES_USER || 'admin',
  password: process.env.POSTGRES_PASSWORD || '0039',
  max: 20,
  idleTimeoutMillis: 30000,
  connectionTimeoutMillis: 2000,
});

app.get('/health', (req, res) => {
  res.json({ 
    status: 'ok', 
    timestamp: new Date().toISOString() 
  });
});

app.post('/api/telemetry', async (req, res) => {
  try {
    const { clientid, topic, payload, timestamp } = req.body;
    
    const topicParts = topic.split('/');
    const deviceToken = topicParts[1];
    const slot = parseInt(topicParts[4], 10);
    
    if (!deviceToken || isNaN(slot)) {
      return res.status(400).json({ error: 'Invalid topic format' });
    }
    
    const deviceResult = await pool.query(
      'SELECT id FROM devices WHERE token::text = $1 AND status = $2',
      [deviceToken, 'active']
    );
    
    if (deviceResult.rows.length === 0) {
      return res.status(404).json({ error: 'Device not found or inactive' });
    }
    
    const deviceId = deviceResult.rows[0].id;
    
    const timestampDate = timestamp 
      ? new Date(Number(timestamp)).toISOString() 
      : new Date().toISOString();
    
    await pool.query(
      'INSERT INTO telemetry (device_id, slot, value, timestamp) VALUES ($1, $2, $3, $4)',
      [deviceId, slot, payload, timestampDate]
    );
    
    res.json({ 
      success: true, 
      device_id: deviceId, 
      slot 
    });
    
  } catch (error) {
    console.error('Error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`API running on port ${PORT}`);
});