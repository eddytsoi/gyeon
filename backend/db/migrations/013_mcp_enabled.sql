-- Add mcp_enabled to site settings
INSERT INTO site_settings (key, value, description)
VALUES ('mcp_enabled', 'false', 'Allow AI agents to connect to this store via MCP')
ON CONFLICT (key) DO NOTHING;
