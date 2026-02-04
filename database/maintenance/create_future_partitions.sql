-- Cria partições para os próximos 3 meses
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..2 LOOP
        PERFORM create_telemetry_partition(
            (CURRENT_DATE + ((i + 1) || ' months')::INTERVAL)::TIMESTAMP
        );
    END LOOP;
END;
$$;

-- Lista partições existentes
SELECT tablename, 
       substring(tablename from 'telemetry_(.*)') as periodo
FROM pg_tables 
WHERE schemaname = 'public' 
  AND tablename LIKE 'telemetry_%'
ORDER BY tablename;