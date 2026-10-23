-- QuickCart Phase 5: Revert Refresh Tokens Migration
-- Migration: 000002_auth_refresh_tokens.down.sql

DROP TABLE IF EXISTS refresh_tokens;
