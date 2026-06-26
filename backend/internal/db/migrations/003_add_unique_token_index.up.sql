-- 增加服务器 agent_token_hash 的唯一索引（仅对非空的值进行唯一约束）
CREATE UNIQUE INDEX servers_agent_token_hash_idx ON servers (agent_token_hash) WHERE agent_token_hash IS NOT NULL;
