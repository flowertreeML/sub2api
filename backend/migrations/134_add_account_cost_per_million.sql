ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS cost_per_million_input  DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS cost_per_million_output DECIMAL(10,4);

COMMENT ON COLUMN accounts.cost_per_million_input  IS 'api_key 模式：每 1M input token 的厂商批发成本';
COMMENT ON COLUMN accounts.cost_per_million_output IS 'api_key 模式：每 1M output token 的厂商批发成本';
