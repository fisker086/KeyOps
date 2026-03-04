-- KeyOps Database Initialization Script (PostgreSQL)
-- Converted from MySQL init.sql
--
-- Usage:
--   psql -U postgres -d keyops -f init_postgres.sql
--
-- Or create database first:
--   CREATE DATABASE keyops;
--   \c keyops
--   \i init_postgres.sql

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Set connection character set to prevent Chinese character encoding issues
-- PostgreSQL uses UTF-8 by default, but explicitly setting it ensures compatibility
SET client_encoding = 'UTF8';

-- KeyOps Database Initialization Script
-- Complete schema with all required tables

-- Create database

-- ============================================================================
-- User Management Tables
-- ============================================================================

-- Users table (platform users)
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY ,
    username VARCHAR(50) UNIQUE NOT NULL ,
    password VARCHAR(255) NOT NULL ,
    ssh_public_key TEXT ,
    ssh_private_key_encrypted TEXT ,
    auth_method VARCHAR(20) DEFAULT 'password' ,
    ssh_key_generated_at TIMESTAMP ,
    ssh_key_fingerprint VARCHAR(255) ,
    email VARCHAR(100) UNIQUE ,
    full_name VARCHAR(100) ,
    role VARCHAR(20) DEFAULT 'user' ,
    status VARCHAR(20) DEFAULT 'active' ,
    expires_at TIMESTAMP ,
    expiration_warning_sent BOOLEAN DEFAULT FALSE ,
    auto_disable_on_expiry BOOLEAN DEFAULT TRUE ,
    last_login_time TIMESTAMP ,
    last_login_ip VARCHAR(45) ,
    organization_id VARCHAR(36) ,

    -- 2FA related fields
    two_factor_enabled BOOLEAN DEFAULT FALSE ,
    two_factor_secret VARCHAR(255) ,
    two_factor_backup_codes TEXT ,
    two_factor_verified_at TIMESTAMP ,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;
CREATE INDEX idx_users_organization_id ON users(organization_id);

-- ============================================================================
-- Host Management Tables
-- ============================================================================

-- Hosts table (asset hosts)
-- Note: Authentication credentials (username, password, private_key) and protocol have been moved to system_users table
-- Hosts are now linked to system_users via permission_rules for flexible permission management
CREATE TABLE IF NOT EXISTS hosts (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(255) NOT NULL ,
    ip VARCHAR(45) NOT NULL ,
    port INTEGER DEFAULT 22 ,
    status VARCHAR(20) DEFAULT 'unknown' ,
    os VARCHAR(100) ,
    cpu VARCHAR(100) ,
    memory VARCHAR(50) ,
    device_type VARCHAR(20) DEFAULT 'linux' ,
    connection_mode VARCHAR(20) DEFAULT 'auto' ,
    proxy_id VARCHAR(128) ,
    network_zone VARCHAR(50) ,
    tags TEXT ,
    description TEXT ,
    last_login_time TIMESTAMP ,
    login_count INTEGER DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Host Groups Tables
-- ============================================================================

-- Host groups table (user-defined groups)
CREATE TABLE IF NOT EXISTS host_groups (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    description TEXT ,
    color VARCHAR(20) ,
    icon VARCHAR(50) ,
    sort_order INTEGER DEFAULT 0 ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Host-Group relationship table (many-to-many)
CREATE TABLE IF NOT EXISTS host_group_members (
    id SERIAL PRIMARY KEY,
    group_id VARCHAR(36) NOT NULL ,
    host_id VARCHAR(36) NOT NULL ,
    added_by VARCHAR(36) ,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (group_id) REFERENCES host_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE,
    UNIQUE (group_id, host_id)

)
;

-- ==================================================================================
-- DEPRECATED: 以下两个表已废弃，新权限架构使用：
-- User → Role → PermissionRule → (SystemUser + HostGroup)
-- 保留这些表是为了向后兼容，但建议在新系统中不再使用
-- ==================================================================================

-- User-Group permissions table (DEPRECATED - 使用新的 roles + permission_rules)
CREATE TABLE IF NOT EXISTS user_group_permissions (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL ,
    group_id VARCHAR(36) NOT NULL ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, group_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES host_groups(id) ON DELETE CASCADE
)
;

-- User-Host permissions table (DEPRECATED - 使用新的 roles + permission_rules)
CREATE TABLE IF NOT EXISTS user_host_permissions (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL ,
    host_id VARCHAR(36) NOT NULL ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, host_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
)
;

-- ============================================================================
-- Session & Connection Tables
-- ============================================================================

-- VM Login records table (只记录虚拟机登录记录，不包括平台登录)
CREATE TABLE IF NOT EXISTS login_records (
    id VARCHAR(100) PRIMARY KEY ,
    user_id VARCHAR(36) NOT NULL ,
    host_id VARCHAR(36) NOT NULL ,
    host_name VARCHAR(255) ,
    host_ip VARCHAR(45) ,
    username VARCHAR(100) ,
    login_ip VARCHAR(45) ,
    user_agent VARCHAR(255) ,
    login_time TIMESTAMP NOT NULL ,
    logout_time TIMESTAMP ,
    duration INTEGER ,
    status VARCHAR(20) DEFAULT 'active' ,
    session_id VARCHAR(100) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Platform Login records table (只记录用户登录平台的记录，不包括虚拟机登录)
CREATE TABLE IF NOT EXISTS platform_login_records (
    id VARCHAR(36) PRIMARY KEY ,
    user_id VARCHAR(36) NOT NULL ,
    username VARCHAR(50) NOT NULL ,
    login_ip VARCHAR(45) ,
    user_agent VARCHAR(255) ,
    login_time TIMESTAMP NOT NULL ,
    status VARCHAR(20) DEFAULT 'active' ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Audit Tables
-- ============================================================================

-- Session history table (complete audit trail)
CREATE TABLE IF NOT EXISTS session_histories (
    id SERIAL PRIMARY KEY,
    proxy_id VARCHAR(100) NOT NULL ,
    session_id VARCHAR(100) UNIQUE NOT NULL ,
    host_id VARCHAR(36) ,
    user_id VARCHAR(100) ,
    username VARCHAR(100) ,
    host_ip VARCHAR(45) ,
    start_time TIMESTAMP NOT NULL ,
    end_time TIMESTAMP ,
    status VARCHAR(20) DEFAULT 'active' ,
    recording TEXT ,
    terminal_cols INTEGER DEFAULT 120 ,
    terminal_rows INTEGER DEFAULT 30 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Command history table (command audit trail)
CREATE TABLE IF NOT EXISTS command_histories (
    id SERIAL PRIMARY KEY,
    proxy_id VARCHAR(100) NOT NULL ,
    session_id VARCHAR(100) NOT NULL ,
    host_id VARCHAR(36) ,
    user_id VARCHAR(100) ,
    username VARCHAR(100) ,
    host_ip VARCHAR(45) ,
    command TEXT NOT NULL ,
    output TEXT ,
    exit_code INTEGER ,
    executed_at TIMESTAMP NOT NULL ,
    duration_ms BIGINT ,
    is_dangerous BOOLEAN DEFAULT FALSE ,
    blocked BOOLEAN DEFAULT FALSE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Pod command history table (Pod terminal command audit trail)
CREATE TABLE IF NOT EXISTS pod_command_histories (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(100) NOT NULL ,
    cluster_name VARCHAR(255) ,
    namespace VARCHAR(255) NOT NULL ,
    pod_name VARCHAR(255) NOT NULL ,
    container VARCHAR(255) ,
    user_id VARCHAR(100) ,
    username VARCHAR(100) ,
    command TEXT NOT NULL ,
    executed_at TIMESTAMP NOT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Session recordings table (unified recordings from webshell and direct SSH)
CREATE TABLE IF NOT EXISTS session_recordings (
    id VARCHAR(36) PRIMARY KEY ,
    session_id VARCHAR(100) UNIQUE NOT NULL ,
    connection_type VARCHAR(20) DEFAULT 'webshell' ,
    proxy_id VARCHAR(100) ,
    user_id VARCHAR(36) ,
    host_id VARCHAR(36) NOT NULL ,
    host_name VARCHAR(255) ,
    host_ip VARCHAR(45) ,
    username VARCHAR(100) ,
    start_time TIMESTAMP NOT NULL ,
    end_time TIMESTAMP ,
    duration VARCHAR(50) ,
    command_count INTEGER DEFAULT 0 ,
    status VARCHAR(20) DEFAULT 'active' ,
    recording TEXT ,
    terminal_cols INTEGER DEFAULT 80 ,
    terminal_rows INTEGER DEFAULT 24 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Operation logs table (API operation audit trail)
CREATE TABLE IF NOT EXISTS operation_logs (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL ,
    ip VARCHAR(50) NOT NULL ,
    method VARCHAR(10) NOT NULL ,
    path VARCHAR(255) NOT NULL ,
    "desc" VARCHAR(255) ,
    status INTEGER NOT NULL ,
    start_time TIMESTAMP NOT NULL ,
    time_cost BIGINT ,
    user_agent VARCHAR(500) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- SSH Host Key Management Tables
-- ============================================================================

-- SSH Host Keys table (shared host key for multi-instance deployment)
CREATE TABLE IF NOT EXISTS ssh_host_keys (
    id SERIAL PRIMARY KEY,
    key_type VARCHAR(20) DEFAULT 'rsa' ,
    key_name VARCHAR(50) DEFAULT 'default' ,
    private_key TEXT NOT NULL ,
    public_key TEXT NOT NULL ,
    fingerprint VARCHAR(255) NOT NULL ,
    key_size INTEGER DEFAULT 2048 ,
    comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (key_type, key_name)

)
;

-- Note: SSH host keys will be automatically generated on first startup
-- The system will check for existence of default RSA key and generate if not found

-- ============================================================================
-- Proxy Management Tables
-- ============================================================================

-- Proxies table (proxy agent status and registration - unified table)
CREATE TABLE IF NOT EXISTS proxies (
    id VARCHAR(36) PRIMARY KEY,
    proxy_id VARCHAR(100) UNIQUE NOT NULL ,
    host_name VARCHAR(255) ,
    ip VARCHAR(45) ,
    port INTEGER ,
    type VARCHAR(32) ,
    status VARCHAR(20) DEFAULT 'offline' ,
    version VARCHAR(50) ,
    network_zone VARCHAR(50) ,
    start_time TIMESTAMP ,
    last_heartbeat TIMESTAMP ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Security Tables
-- ============================================================================

-- Blacklist rules table (dangerous command blocking)
CREATE TABLE IF NOT EXISTS blacklist_rules (
    id VARCHAR(64) PRIMARY KEY ,
    command VARCHAR(255) NOT NULL ,
    pattern VARCHAR(512) NOT NULL ,
    description TEXT ,
    scope VARCHAR(20) DEFAULT 'global' ,
    users JSONB ,
    enabled BOOLEAN DEFAULT TRUE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- System Configuration Tables
-- ============================================================================

-- Settings table (system configuration)
CREATE TABLE IF NOT EXISTS settings (
    id SERIAL PRIMARY KEY,
    "key" VARCHAR(100) UNIQUE NOT NULL ,
    value TEXT ,
    category VARCHAR(50) ,
    type VARCHAR(20) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Initial Data
-- ============================================================================

-- Admin user will be inserted later with a specific UUID for consistency

-- Insert default host groups (no default hosts, users should add their own)
INSERT INTO host_groups (id, name, description, color, icon, sort_order, created_by) VALUES
    ('default-group', 'Default', 'Default host group', '#1890ff', '', 0, NULL)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- Insert default blacklist rules
INSERT INTO blacklist_rules (id, command, pattern, description, scope, enabled)
VALUES
    (gen_random_uuid(), 'rm', '^rm\\s+.*(-rf?|--recursive).*', 'Block dangerous file deletion', 'global', TRUE),
    (gen_random_uuid(), 'dd', '^dd\\s+.*of=/dev/', 'Block disk overwrite', 'global', TRUE),
    (gen_random_uuid(), 'mkfs', '^mkfs\\.', 'Block filesystem formatting', 'global', TRUE),
    (gen_random_uuid(), 'reboot', '^(reboot|shutdown|halt|poweroff)', 'Block system restart', 'global', TRUE),
    (gen_random_uuid(), 'fdisk', '^fdisk\\s+/dev/', 'Block disk partitioning', 'global', TRUE)
ON CONFLICT (id) DO UPDATE SET command = EXCLUDED.command;

-- Insert default system settings
INSERT INTO settings ("key", value, category, type)
VALUES
    ('system.title', 'KeyOps', 'system', 'string'),
    ('system.siteName', 'KeyOps', 'system', 'string'),
    ('system.showWatermark', 'false', 'system', 'boolean'),
    ('system.session_timeout', '3600', 'system', 'number'),
    ('security.max_login_attempts', '5', 'security', 'number'),
    ('security.lockout_duration', '1800', 'security', 'number'),
    ('audit.enable_command_recording', 'true', 'audit', 'boolean'),
    ('audit.enable_session_recording', 'true', 'audit', 'boolean'),
    ('terminal.default_cols', '120', 'terminal', 'number'),
    ('terminal.default_rows', '30', 'terminal', 'number'),
    -- Windows / RDP defaults
    ('windows.enable_windows_access', 'false', 'windows', 'boolean'),
    -- guacd_host: Docker Compose 服务名，容器间通信使用服务名
    ('windows.guacd_host', 'guacd', 'windows', 'string'),
    ('windows.guacd_port', '4822', 'windows', 'number'),
    ('windows.recording_enabled', 'true', 'windows', 'boolean'),
    -- recording_path: 宿主机路径（挂载路径）
    -- Docker 部署时，docker-compose.yml 中挂载为: ./recordings:/replay
    -- 宿主机路径是 ./recordings（相对于 docker-compose.yml 所在目录）
    -- 实际使用时，建议使用绝对路径，例如: /data/keyops/recordings
    -- 注意：不要填容器内路径 /replay，要填宿主机路径
    ('windows.recording_path', './recordings', 'windows', 'string'),
    ('windows.recording_format', 'guac', 'windows', 'string'),
    ('windows.allow_clipboard', 'false', 'windows', 'boolean'),
    ('windows.enable_file_transfer', 'false', 'windows', 'boolean'),
    ('windows.drive_path', '/replay-drive', 'windows', 'string')
ON CONFLICT ("key") DO UPDATE SET "key" = EXCLUDED."key";

-- ============================================================================
-- Low-Code Ticket Platform Tables (低代码工单平台)
-- ============================================================================

-- Form categories table (表单模板分类表)
CREATE TABLE IF NOT EXISTS form_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE ,
    description VARCHAR(255) DEFAULT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- Form templates table (表单模板表)
CREATE TABLE IF NOT EXISTS form_templates (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL ,
    category VARCHAR(50) DEFAULT NULL ,
    description TEXT ,
    "schema" JSONB NOT NULL ,
    approval_config JSONB DEFAULT NULL ,
    status VARCHAR(20) DEFAULT 'active' ,
    version VARCHAR(20) DEFAULT '1.0.0' ,
    created_by VARCHAR(50) DEFAULT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Tickets table (工单表)
CREATE TABLE IF NOT EXISTS tickets (
    id SERIAL PRIMARY KEY,
    ticket_number VARCHAR(50) NOT NULL ,
    template_id BIGINT NULL ,
    type VARCHAR(20) DEFAULT 'daily' ,
    title VARCHAR(200) NOT NULL ,
    form_data JSONB NOT NULL ,
    status VARCHAR(20) DEFAULT 'draft' ,
    priority VARCHAR(20) DEFAULT 'normal' ,
    applicant_id VARCHAR(50) NOT NULL ,
    applicant_name VARCHAR(100) NOT NULL ,
    applicant_email VARCHAR(100) DEFAULT NULL ,
    approval_platform VARCHAR(20) DEFAULT NULL ,
    approval_instance_id VARCHAR(100) DEFAULT NULL ,
    approval_url VARCHAR(500) DEFAULT NULL ,
    current_approver VARCHAR(100) DEFAULT NULL ,
    approvers JSONB DEFAULT NULL ,
    approval_steps JSONB DEFAULT NULL ,
    approval_result VARCHAR(20) DEFAULT NULL ,
    approval_comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (ticket_number),
    FOREIGN KEY (template_id) REFERENCES form_templates(id) ON DELETE SET NULL
)
;

-- Approval configs table (审批配置表 - 用于工单模板的审批配置)
CREATE TABLE IF NOT EXISTS ticket_approval_configs (
    id SERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL ,
    platform VARCHAR(20) NOT NULL ,
    approval_code VARCHAR(100) DEFAULT NULL ,
    approval_flow JSONB NOT NULL ,
    auto_approve BOOLEAN DEFAULT FALSE ,
    timeout_hours INTEGER DEFAULT 24 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (template_id, platform),
    FOREIGN KEY (template_id) REFERENCES form_templates(id) ON DELETE CASCADE
)
;

-- ============================================================================
-- Approval Management Tables (工单审批系统)
-- ============================================================================

-- Approvals table (审批工单)
CREATE TABLE IF NOT EXISTS approvals (
    id VARCHAR(36) PRIMARY KEY ,
    title VARCHAR(255) NOT NULL ,
    description TEXT ,
    type VARCHAR(50) NOT NULL ,
    status VARCHAR(50) DEFAULT 'pending' ,
    platform VARCHAR(50) DEFAULT 'internal' ,

    -- Applicant information
    applicant_id VARCHAR(36) NOT NULL ,
    applicant_name VARCHAR(100) ,
    applicant_email VARCHAR(100) ,

    -- Approver information
    approver_ids TEXT ,
    approver_names TEXT ,
    current_approver VARCHAR(100) ,

    -- Resource information
    resource_type VARCHAR(50) ,
    resource_ids TEXT ,
    resource_names TEXT ,

    -- Permission information
    permissions TEXT ,
    duration INTEGER ,
    expires_at TIMESTAMP ,

    -- Approval details
    reason TEXT ,
    approval_note TEXT ,
    reject_reason TEXT ,
    priority VARCHAR(20) DEFAULT 'normal' ,

    -- External platform information
    external_id VARCHAR(255) ,
    external_url TEXT ,
    external_data TEXT ,

    -- Deployment related fields (when type is deployment)
    deploy_config TEXT ,
    deployment_id VARCHAR(36) ,
    deployed BOOLEAN DEFAULT FALSE ,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    approved_at TIMESTAMP ,
    rejected_at TIMESTAMP,
    FOREIGN KEY (applicant_id) REFERENCES users(id) ON DELETE CASCADE
)
;

-- Approval comments table (审批评论/历史记录)
CREATE TABLE IF NOT EXISTS approval_comments (
    id VARCHAR(36) PRIMARY KEY ,
    approval_id VARCHAR(36) NOT NULL ,
    user_id VARCHAR(36) NOT NULL ,
    user_name VARCHAR(100) ,
    action VARCHAR(50) ,
    comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (approval_id) REFERENCES approvals(id) ON DELETE CASCADE
)
;

-- 文件传输审计表
CREATE TABLE IF NOT EXISTS file_transfers (
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    username VARCHAR(100),
    host_id VARCHAR(36) NOT NULL,
    host_ip VARCHAR(50),
    host_name VARCHAR(200),
    direction VARCHAR(20) NOT NULL,  -- upload, download
    local_path VARCHAR(500),
    remote_path VARCHAR(500),
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'uploading',  -- uploading, completed, failed
    progress INTEGER DEFAULT 0,
    error_message TEXT,
    transferred_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    duration INTEGER DEFAULT 0
    -- 传输耗时（秒）
)
;

-- 资产同步配置表
CREATE TABLE IF NOT EXISTS asset_sync_configs (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- prometheus, zabbix, cmdb, custom
    enabled BOOLEAN DEFAULT true,
    url VARCHAR(500) NOT NULL,
    auth_type VARCHAR(20),  -- none, basic, token, oauth
    username VARCHAR(100),
    password VARCHAR(200),
    token TEXT,
    sync_interval INTEGER DEFAULT 60,  -- 同步间隔（分钟）
    last_sync_time TIMESTAMP,
    last_sync_status VARCHAR(20),  -- success, failed
    synced_count INTEGER DEFAULT 0,
    error_message TEXT,
    config TEXT,  -- JSON配置
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 资产同步日志表
CREATE TABLE IF NOT EXISTS asset_sync_logs (
    id VARCHAR(36) PRIMARY KEY,
    config_id VARCHAR(36) NOT NULL,
    status VARCHAR(20),  -- success, failed
    synced_count INTEGER DEFAULT 0,
    error_message TEXT,
    duration INTEGER DEFAULT 0,  -- 耗时（秒）
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Approval Platform Configuration Tables
-- ============================================================================

-- 工单平台配置表
CREATE TABLE IF NOT EXISTS approval_configs (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    type VARCHAR(20) NOT NULL ,
    enabled BOOLEAN DEFAULT false ,

    -- 应用凭证
    app_id VARCHAR(100) NOT NULL ,
    app_secret VARCHAR(200) NOT NULL ,

    -- 平台特定字段
    approval_code VARCHAR(100) ,
    process_code VARCHAR(100) ,
    template_id VARCHAR(100) ,

    -- 表单字段映射
    form_fields TEXT ,

    -- 审批人配置
    approver_user_ids TEXT ,

    -- API配置
    api_base_url VARCHAR(500) DEFAULT '' ,
    api_path VARCHAR(200) DEFAULT '' ,
    api_path_get VARCHAR(200) DEFAULT '' ,
    api_path_cancel VARCHAR(200) DEFAULT '' ,

    -- 回调配置
    callback_url VARCHAR(500) DEFAULT '' ,

    -- 时间戳
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- System Users and Roles Tables (多系统用户权限管理)
-- ============================================================================

-- 系统用户表（目标主机上的操作系统用户）
CREATE TABLE IF NOT EXISTS system_users (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    username VARCHAR(100) NOT NULL ,

    -- 认证信息 (明确认证方式，不支持 auto)
    auth_type VARCHAR(20) DEFAULT 'password' ,
    password TEXT ,
    private_key TEXT ,
    passphrase TEXT ,

    -- 协议和设置
    protocol VARCHAR(20) DEFAULT 'ssh' ,

    -- 其他设置
    priority INTEGER DEFAULT 0 ,
    description TEXT ,
    status VARCHAR(20) DEFAULT 'active' ,

    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 角色表（平台角色，用于批量权限管理）
CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL UNIQUE ,
    description TEXT ,

    -- 显示相关
    color VARCHAR(20) ,
    icon VARCHAR(50) ,
    priority INTEGER DEFAULT 0 ,

    status VARCHAR(20) DEFAULT 'active' ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 角色成员表（用户加入角色）
CREATE TABLE IF NOT EXISTS role_members (
    id SERIAL PRIMARY KEY,
    role_id VARCHAR(36) NOT NULL ,
    user_id VARCHAR(36) NOT NULL ,
    added_by VARCHAR(36) ,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (role_id, user_id)

)
;

-- 授权规则表（角色 + 系统用户 + 主机组 = 权限）
CREATE TABLE IF NOT EXISTS permission_rules (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(200) NOT NULL ,

    -- 授权对象
    role_id VARCHAR(36) NOT NULL ,

    -- 资产范围
    host_group_id VARCHAR(36) ,
    host_ids TEXT ,

    -- 系统用户
    system_user_id VARCHAR(36) ,

    -- 时间限制
    valid_from TIMESTAMP ,
    valid_to TIMESTAMP ,

    -- 状态
    enabled BOOLEAN DEFAULT true ,
    priority INTEGER DEFAULT 0 ,

    description TEXT ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (host_group_id) REFERENCES host_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (system_user_id) REFERENCES system_users(id) ON DELETE CASCADE

)
;

-- Permission rule to system user mapping (many-to-many)
CREATE TABLE IF NOT EXISTS permission_rule_system_users (
    id SERIAL PRIMARY KEY,
    permission_rule_id VARCHAR(36) NOT NULL ,
    system_user_id VARCHAR(36) NOT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (permission_rule_id, system_user_id),
    FOREIGN KEY (permission_rule_id) REFERENCES permission_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (system_user_id) REFERENCES system_users(id) ON DELETE CASCADE
)
;

-- Permission rule to host group mapping (many-to-many)
CREATE TABLE IF NOT EXISTS permission_rule_host_groups (
    id SERIAL PRIMARY KEY,
    permission_rule_id VARCHAR(36) NOT NULL ,
    host_group_id VARCHAR(36) NOT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (permission_rule_id, host_group_id),
    FOREIGN KEY (permission_rule_id) REFERENCES permission_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (host_group_id) REFERENCES host_groups(id) ON DELETE CASCADE
)
;

-- COMMIT; -- PostgreSQL doesn't need explicit COMMIT in scripts

-- =====================================================
-- Initialize Default Admin User
-- =====================================================

-- Insert default admin user (password: admin123, should be changed after first login)
-- Password hash is bcrypt hash of 'admin123'
-- Note: organization_id will be set after organizations table is created
INSERT INTO users (id, username, password, full_name, email, role, status, organization_id, created_at, updated_at)
VALUES
    ('00000000-0000-0000-0000-000000000001',
     'admin',
     '$2a$10$j/lQBaOvW9dMo/O13g65qeCwYnxuaZerNcB/eA3IZZXxRp4MbePhG',
     'System Admin',
     'admin@keyops.local',
     'admin',
     'active',
     NULL, -- Will be updated after organizations are created
     NOW(),
     NOW());

-- =====================================================
-- Initialize System Settings
-- =====================================================

-- Host Monitor Settings
INSERT INTO settings ("key", "value", "category", "type", "created_at", "updated_at") VALUES
('host_monitor_enabled', 'false', 'host_monitor', 'boolean', NOW(), NOW()),
('host_monitor_interval', '5', 'host_monitor', 'number', NOW(), NOW()),
('host_monitor_method', 'tcp', 'host_monitor', 'string', NOW(), NOW()),
('host_monitor_timeout', '3', 'host_monitor', 'number', NOW(), NOW()),
('host_monitor_concurrent', '20', 'host_monitor', 'number', NOW(), NOW());

-- =====================================================
-- Initialize System Users and Roles
-- =====================================================

-- 插入系统角色到 roles 表（统一角色管理）
-- 系统角色和自定义角色都统一在 roles 表中管理，通过 role_members 表关联用户
INSERT INTO roles (id, name, description, color, priority, status, created_at, updated_at) VALUES
('role:admin', '管理员', '系统管理员角色，拥有所有权限', '#f5222d', 999, 'active', NOW(), NOW()),
('role:user', '普通用户', '普通用户角色，拥有基础权限', '#52c41a', 0, 'active', NOW(), NOW());

-- 为 admin 用户分配系统角色 role:admin
INSERT INTO role_members (role_id, user_id, added_by, added_at)
SELECT 'role:admin', id, id, NOW()
FROM users
WHERE username = 'admin' AND NOT EXISTS (
    SELECT 1 FROM role_members WHERE role_id = 'role:admin' AND user_id = users.id
);

-- =====================================================
-- Expiration Management Tables
-- =====================================================

-- User expiration logs
CREATE TABLE IF NOT EXISTS user_expiration_logs (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL ,
    username VARCHAR(50) NOT NULL ,
    action VARCHAR(50) NOT NULL ,
    expires_at TIMESTAMP ,
    new_expires_at TIMESTAMP ,
    reason TEXT ,
    performed_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Permission expiration logs
CREATE TABLE IF NOT EXISTS permission_expiration_logs (
    id SERIAL PRIMARY KEY,
    rule_id VARCHAR(36) NOT NULL ,
    rule_name VARCHAR(200) NOT NULL ,
    role_id VARCHAR(36) NOT NULL ,
    role_name VARCHAR(100) ,
    action VARCHAR(50) NOT NULL ,
    valid_to TIMESTAMP ,
    new_valid_to TIMESTAMP ,
    reason TEXT ,
    performed_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Expiration notification config
CREATE TABLE IF NOT EXISTS expiration_notification_config (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL ,
    warning_days INTEGER NOT NULL DEFAULT 7 ,
    enabled BOOLEAN DEFAULT TRUE ,
    notification_channels TEXT ,
    message_template TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE (type)
)
;

-- Insert default notification configs
INSERT INTO expiration_notification_config (type, warning_days, enabled, notification_channels)
VALUES
    ('user', 7, TRUE, '["system", "email"]'),
    ('permission', 3, TRUE, '["system", "email"]');

-- Expiration system settings
INSERT INTO settings ("key", "value", "category", "type", "created_at", "updated_at") VALUES
('expiration_check_enabled', 'true', 'expiration', 'boolean', NOW(), NOW()),
('expiration_check_interval', '3600', 'expiration', 'number', NOW(), NOW()),
('user_expiration_auto_disable', 'true', 'expiration', 'boolean', NOW(), NOW()),
('permission_expiration_auto_disable', 'true', 'expiration', 'boolean', NOW(), NOW()),
('expiration_warning_days_user', '7', 'expiration', 'number', NOW(), NOW()),
('expiration_warning_days_permission', '3', 'expiration', 'number', NOW(), NOW());

-- ============================================================================
-- Two-Factor Authentication Tables
-- ============================================================================

-- 2FA global configuration table
CREATE TABLE IF NOT EXISTS two_factor_config (
    id SERIAL PRIMARY KEY,
    enabled BOOLEAN DEFAULT FALSE ,
    issuer VARCHAR(100) DEFAULT 'KeyOps' ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- Insert default 2FA configuration
INSERT INTO two_factor_config (enabled, issuer) VALUES (FALSE, 'KeyOps')
ON CONFLICT (id) DO UPDATE SET issuer = EXCLUDED.issuer;

-- ============================================================================
-- Permission Management Tables (菜单和API权限管理)
-- ============================================================================

-- 菜单表
CREATE TABLE IF NOT EXISTS menus (
    id VARCHAR(36) PRIMARY KEY ,
    parent_id VARCHAR(36) DEFAULT '' ,
    path VARCHAR(255) NOT NULL ,
    name VARCHAR(100) NOT NULL ,
    component VARCHAR(255) ,
    hidden BOOLEAN DEFAULT FALSE ,
    sort INTEGER DEFAULT 0 ,

    -- 菜单元数据
    title VARCHAR(100) NOT NULL ,
    icon VARCHAR(50) ,
    keep_alive BOOLEAN DEFAULT FALSE ,
    active_name VARCHAR(100) ,
    close_tab BOOLEAN DEFAULT FALSE ,
    default_menu BOOLEAN DEFAULT FALSE ,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 菜单权限关联表（角色和菜单的关联）
-- 注意：role_id可以是role:admin、role:user或角色ID，所以不设置外键约束
CREATE TABLE IF NOT EXISTS menu_permissions (
    id SERIAL PRIMARY KEY,
    role_id VARCHAR(100) NOT NULL ,
    menu_id VARCHAR(36) NOT NULL ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE CASCADE,
    UNIQUE (role_id, menu_id)

)
;

-- AI运维助手：目标环境（用户自定义）
CREATE TABLE IF NOT EXISTS ai_assistant_environments (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    prom_url VARCHAR(500) DEFAULT '' ,
    graf_url VARCHAR(500) DEFAULT '' ,
    graf_token VARCHAR(500) DEFAULT '' ,
    cluster VARCHAR(100) DEFAULT '' ,
    k8s_cluster_id VARCHAR(36) DEFAULT '' ,
    extra_config TEXT ,
    allowed_role_ids TEXT ,
    sort INTEGER DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_env_sort ON ai_assistant_environments(sort);
CREATE INDEX IF NOT EXISTS idx_ai_env_k8s_cluster ON ai_assistant_environments(k8s_cluster_id);
COMMENT ON COLUMN ai_assistant_environments.extra_config IS '扩展配置(JSON对象)，非Prometheus/Grafana时使用';
COMMENT ON COLUMN ai_assistant_environments.allowed_role_ids IS '允许使用该环境的平台角色ID列表(JSON数组)，空或NULL表示所有角色可用';
COMMENT ON COLUMN ai_assistant_environments.k8s_cluster_id IS '关联K8s集群ID（来自k8s管理）';

-- 默认环境（占位测试地址，请在「目标环境」页修改为实际地址）
INSERT INTO ai_assistant_environments (id, name, prom_url, graf_url, graf_token, cluster, k8s_cluster_id, sort) VALUES
('default', '默认环境', 'http://prometheus.example.com', '', '', '', '', 0),
('k8s-install', 'K8s 安装', '', '', '', '', '', 1)
ON CONFLICT (id) DO NOTHING;

-- AI运维助手：专家角色（模板+用户自定义）
CREATE TABLE IF NOT EXISTS ai_assistant_experts (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    description VARCHAR(500) DEFAULT '' ,
    system_prompt TEXT NOT NULL ,
    skill_id VARCHAR(64) DEFAULT NULL ,
    is_custom BOOLEAN DEFAULT FALSE ,
    allowed_role_ids TEXT ,
    sort INTEGER DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_expert_sort ON ai_assistant_experts(sort);
CREATE INDEX IF NOT EXISTS idx_ai_expert_custom ON ai_assistant_experts(is_custom);
COMMENT ON COLUMN ai_assistant_experts.allowed_role_ids IS '允许使用该专家的平台角色ID列表(JSON数组)，空或NULL表示所有角色可用';

-- Seed default experts (prompts editable in UI; code uses same only when DB empty or no-DB)
INSERT INTO ai_assistant_experts (id, name, description, system_prompt, is_custom, sort) VALUES
('sre', '故障排查专家', '专注单点异常的深度下钻和根因定位，适合处理已知报警或故障。', '你是一名资深系统故障排查专家（SRE Analysis Expert），拥有大规模系统运维经验，擅长从异常信号中通过深度下钻（Drill-down）精确定位根因。

## 核心逻辑：
1. **现象先行**：先通过工具获取真实指标（如 execute_promql_query、list_all_dashboards），仅根据 Observation 中的真实数据描述现象（如 CPU 突增、错误率飙升），禁止在无 Observation 时猜测具体数值。
2. **深度下钻**：发现异常后，针对该维度做关联查询（如 get_metric_dimension 查标签、多指标 check_correlation），逐步缩小范围。
3. **因果推导**：结合多步 Observation 推导根因，Final Answer 中每一句结论都应对应到某一步的 Observation；若多轮后仍无法确定根因，如实写出「已查内容」与「未确定项」。
4. **修复闭环**：根据目标环境与已观测指标，给出可直接执行的修复命令（按 OS/集群环境区分）。若用户明确授权“请直接执行/帮我处理”，且当前环境存在可执行类工具，则在变更前先说明动作与风险，再执行并回报结果；若无可执行工具，明确说明并给出人工执行命令。


## 核心约束（严禁违反）：
1. **身份/能力类问题**：若用户仅询问你的身份、角色或能力（如：你是谁、有什么功能、你能做什么），请**直接**以 "Final Answer:" 开头回答，不要使用任何 Action 或工具。
2. **严禁幻觉**：你只能看到系统通过 "Observation:" 返回的真实数据。**严禁自行猜测、臆造工具的执行结果或业务指标数据**；结论中出现的任何数字、指标、状态必须来自某一步的 Observation。
3. **单步执行**：每一轮输出必须且只能包含**一个** Action。格式为：Thought: ... Action: 工具名 Action Input: 纯 JSON。输出完 Action Input 后**立即停止**，不要在同一轮中写多个 Action、不要继续写 Thought 或其它内容，等待系统返回 Observation。
4. **格式严求**：必须严格按顺序输出 Thought → Action → Action Input 三段；Action Input 必须是**纯 JSON 对象**（键名与工具箱参数一致），不要使用 Markdown 代码块包裹，不要加任何说明文字。
5. **真实结论**：Final Answer 必须完全基于前面步骤中获得的真实 Observation 数据。若某步 Observation 返回错误或超时，可下一步尝试修正查询（如调整时间范围、指标名）再执行；若无法恢复，在 Final Answer 中如实说明「某步查询失败及可能原因」，不要编造数据。
6. **语言一致**：回答使用与用户相同的语言（用户用中文则用中文，用英文则用英文）。

## 工具调用规范：
- 系统会根据目标环境配置，在下方自动追加「当前环境可用技能」及工具箱列表。**Action** 必须是其中列出的工具名称，未列出的工具表示当前环境未配置，请勿调用。
- **Action Input**：仅写一个合法 JSON 对象，键名和值类型严格符合工具 Schema，例如 execute_promql_query 的 {"query":"up","duration":"1h"}。


## 最终结论要求 (Final Answer 格式)：
必须以 "Final Answer:" 开头，包含：1. 现象描述与影响（引用 Observation 数据） 2. 定位与分析过程 3. 根因结论 (RCA) 4. 修复与建议措施（含可执行命令） 5. 若已执行变更，给出执行记录与验证结果。禁止编造未在 Observation 中出现的数据。
', false, 1),
('inspector', '全局巡检专家', '广度优先，全面扫描系统各组件状态，提供整体"体检报告"。', '你是一名系统全局巡检专家（Global Inspection Expert），负责对系统做「大面积扫射」式巡检，覆盖多维度指标，发现潜在隐患并输出专业巡检报告。

## 巡检逻辑：
1. **多维扫描**：依次覆盖资源利用率（CPU/内存/磁盘）、流量与 QPS、错误率、延迟（如 P99）、依赖健康等；可先用 list_all_dashboards、find_metrics_by_keyword 了解可用指标。
2. **全局覆盖**：优先查看所有服务/实例的汇总或抽样数据，避免只盯单点；对异常维度使用 execute_promql_query、detect_anomaly 做进一步确认。
3. **隐患识别**：记录趋势恶化、容量接近上限、错误率抬升等，所有结论必须对应 Observation 中的真实数据。
4. **全面汇总**：在 30 步内优先覆盖核心组件与关键指标，再视步数补充；若某步查询失败，在 Final Answer 中说明并给出可能原因。
5. **处置建议可执行**：对每个高优先级问题给出对应修复命令（按目标环境区分）。若用户明确授权执行且存在可执行类工具，可执行低风险修复动作并反馈结果；高风险动作需先说明风险与回滚建议。


## 核心约束（严禁违反）：
1. **身份/能力类问题**：若用户仅询问你的身份、角色或能力（如：你是谁、有什么功能、你能做什么），请**直接**以 "Final Answer:" 开头回答，不要使用任何 Action 或工具。
2. **严禁幻觉**：你只能看到系统通过 "Observation:" 返回的真实数据。**严禁自行猜测、臆造工具的执行结果或业务指标数据**；结论中出现的任何数字、指标、状态必须来自某一步的 Observation。
3. **单步执行**：每一轮输出必须且只能包含**一个** Action。格式为：Thought: ... Action: 工具名 Action Input: 纯 JSON。输出完 Action Input 后**立即停止**，不要在同一轮中写多个 Action、不要继续写 Thought 或其它内容，等待系统返回 Observation。
4. **格式严求**：必须严格按顺序输出 Thought → Action → Action Input 三段；Action Input 必须是**纯 JSON 对象**（键名与工具箱参数一致），不要使用 Markdown 代码块包裹，不要加任何说明文字。
5. **真实结论**：Final Answer 必须完全基于前面步骤中获得的真实 Observation 数据。若某步 Observation 返回错误或超时，可下一步尝试修正查询（如调整时间范围、指标名）再执行；若无法恢复，在 Final Answer 中如实说明「某步查询失败及可能原因」，不要编造数据。
6. **语言一致**：回答使用与用户相同的语言（用户用中文则用中文，用英文则用英文）。

## 工具调用规范：
- 系统会根据目标环境配置，在下方自动追加「当前环境可用技能」及工具箱列表。**Action** 必须是其中列出的工具名称，未列出的工具表示当前环境未配置，请勿调用。
- **Action Input**：仅写一个合法 JSON 对象，键名和值类型严格符合工具 Schema，例如 execute_promql_query 的 {"query":"up","duration":"1h"}。


## 最终结论输出要求 (Final Answer 格式)：
必须以 "Final Answer:" 开头，包含：1. 巡检概况（整体评分） 2. 指标分类与分析（引用 Observation） 3. 问题识别与根因分析 4. 关联、趋势与容量建议 5. 行动建议（含可执行命令） 6. 总结与改进（若已执行，附执行结果）。不得编造未在 Observation 中出现的数据。
', false, 2),
('k8s-expert', 'K8s 容器专家', '专注 K8s 集群巡检与容器排障，覆盖节点、Pod、工作负载与资源使用分析。', '你是一名 K8s 容器专家（K8s Container Expert），负责对 Kubernetes 集群进行巡检，从节点、Pod、工作负载、资源与事件等维度发现潜在问题，并输出专业巡检报告。

## 巡检逻辑：
1. **集群概览**：优先用 K8s 工具箱（k8s_cluster_summary、k8s_list_nodes、k8s_list_namespaces、k8s_list_pods）获取真实状态；可结合 Prometheus/Grafana 工具（execute_promql_query 等）查看资源指标。
2. **事件优先**：尽早使用 k8s_list_events 查看 Critical/Warning 事件，便于发现近期错误与调度失败；再结合 k8s_list_pods（按 namespace 或状态筛选）定位异常 Pod。
3. **资源与容量**：关注节点就绪、Pending/Failed Pod、副本数、资源 request/limit；用 k8s_list_deployments、k8s_list_daemonsets、k8s_list_statefulsets 检查工作负载健康，必要时用 execute_promql_query 做 CPU/内存使用分析。
4. **全面汇总**：在 30 步内覆盖集群关键资源与组件，先 K8s 工具再指标工具；结论必须基于 Observation，若某步失败则在 Final Answer 中说明。
5. **修复闭环**：针对异常工作负载/节点给出可执行修复命令（如 kubectl rollout restart、驱逐异常 Pod、修复配置示例）。若用户明确授权执行且当前环境存在可执行类工具，可执行低风险修复动作并回报变更结果与验证结果。


## 核心约束（严禁违反）：
1. **身份/能力类问题**：若用户仅询问你的身份、角色或能力（如：你是谁、有什么功能、你能做什么），请**直接**以 "Final Answer:" 开头回答，不要使用任何 Action 或工具。
2. **严禁幻觉**：你只能看到系统通过 "Observation:" 返回的真实数据。**严禁自行猜测、臆造工具的执行结果或业务指标数据**；结论中出现的任何数字、指标、状态必须来自某一步的 Observation。
3. **单步执行**：每一轮输出必须且只能包含**一个** Action。格式为：Thought: ... Action: 工具名 Action Input: 纯 JSON。输出完 Action Input 后**立即停止**，不要在同一轮中写多个 Action、不要继续写 Thought 或其它内容，等待系统返回 Observation。
4. **格式严求**：必须严格按顺序输出 Thought → Action → Action Input 三段；Action Input 必须是**纯 JSON 对象**（键名与工具箱参数一致），不要使用 Markdown 代码块包裹，不要加任何说明文字。
5. **真实结论**：Final Answer 必须完全基于前面步骤中获得的真实 Observation 数据。若某步 Observation 返回错误或超时，可下一步尝试修正查询（如调整时间范围、指标名）再执行；若无法恢复，在 Final Answer 中如实说明「某步查询失败及可能原因」，不要编造数据。
6. **语言一致**：回答使用与用户相同的语言（用户用中文则用中文，用英文则用英文）。

## 工具调用规范：
- 系统会根据目标环境配置，在下方自动追加「当前环境可用技能」及工具箱列表。**Action** 必须是其中列出的工具名称，未列出的工具表示当前环境未配置，请勿调用。
- **Action Input**：仅写一个合法 JSON 对象，键名和值类型严格符合工具 Schema，例如 execute_promql_query 的 {"query":"up","duration":"1h"}。


## 最终结论输出要求 (Final Answer 格式)：
必须以 "Final Answer:" 开头，包含：1. K8s 集群巡检概况（整体评分） 2. 节点与 Pod 健康分析 3. 工作负载与资源使用问题 4. 问题识别与根因分析 5. 容量与优化建议 6. 行动建议与总结（含可执行命令；若已执行附结果）。不得编造未在 Observation 中出现的数据。
', false, 3),
('k8s-installer', 'K8s 安装专家', '指导 K8s 集群安装，含 containerd、CNI、kubeadm/二进制，分步骤输出命令，每步等待用户确认。', '你是 K8s 集群安装专家，负责指导用户安装 Kubernetes 集群（含 containerd、CNI、kubeadm 或二进制方式）。

## 核心约束（必须遵守）：
1. **不使用任何工具**：你**不要**使用任何 Action 或工具，每次**直接**以 "Final Answer:" 开头输出，不要输出 Thought、Action、Action Input。
2. **分步骤输出**：每步只输出**一个阶段**的安装命令，步骤末尾必须加：「请确认以上命令执行无误后，回复「继续」以获取下一步。」
3. **首次回复**：若用户未提供完整信息，先以 Final Answer 形式询问：安装方式（kubeadm/二进制）、master/worker/etcd 节点、K8s 版本、CNI 类型（Calico/Flannel）、操作系统。
4. **引用知识库**：所有下载链接、命令必须严格使用下方「安装知识库」中的 URL 格式；**版本号根据用户指定或首次确认的值动态替换**，禁止写死版本。
5. **语言一致**：使用与用户相同的语言。

## 下方为安装知识库（由系统注入，包含官方下载地址与安装流程）：
', false, 4)
ON CONFLICT (id) DO NOTHING;

-- AI运维助手：模型配置（可新建多个，智能对话中可选）
CREATE TABLE IF NOT EXISTS ai_assistant_models (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    model VARCHAR(100) NOT NULL ,
    api_key VARCHAR(500) NOT NULL ,
    base_url VARCHAR(500) DEFAULT '' ,
    proxy_url VARCHAR(500) DEFAULT '' ,
    max_steps INTEGER DEFAULT 30 ,
    sort INTEGER DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_model_sort ON ai_assistant_models(sort);
COMMENT ON TABLE ai_assistant_models IS 'AI运维助手模型配置';

-- API表（用于API权限管理）
CREATE TABLE IF NOT EXISTS apis (
    id SERIAL PRIMARY KEY,
    path VARCHAR(255) NOT NULL ,
    method VARCHAR(20) NOT NULL ,
    "group" VARCHAR(100) NOT NULL ,
    description VARCHAR(255) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (path, method)
)
;

-- ============================================================================
-- K8s Cluster Management Tables
-- ============================================================================

-- K8s集群表
CREATE TABLE IF NOT EXISTS k8s_clusters (
    id VARCHAR(36) PRIMARY KEY ,
    name VARCHAR(100) NOT NULL UNIQUE ,
    display_name VARCHAR(100) ,
    description TEXT ,

    -- 连接配置
    api_server VARCHAR(255) NOT NULL ,
    token TEXT ,
    kubeconfig TEXT ,
    auth_type VARCHAR(20) DEFAULT 'token' ,

    -- 集群信息
    version VARCHAR(50) ,
    region VARCHAR(100) ,
    environment VARCHAR(50) ,

    -- 状态和设置
    status VARCHAR(20) DEFAULT 'active' ,
    default_namespace VARCHAR(100) ,

    -- 审计和元数据
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_checked_at TIMESTAMP,
    UNIQUE (api_server)
)
;

-- K8s集群权限表（用户/角色对集群的访问权限）
CREATE TABLE IF NOT EXISTS cluster_permissions (
    id VARCHAR(36) PRIMARY KEY ,
    cluster_id VARCHAR(36) NOT NULL ,
    user_id VARCHAR(36) NULL ,
    role_id VARCHAR(36) NULL ,

    -- 权限类型
    permission_type VARCHAR(20) DEFAULT 'read' ,

    -- 命名空间限制（可选，NULL表示所有命名空间）
    allowed_namespaces TEXT ,

    -- 元数据
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (cluster_id, user_id),
    UNIQUE (cluster_id, role_id),
    FOREIGN KEY (cluster_id) REFERENCES k8s_clusters(id) ON DELETE CASCADE
)
;

-- Casbin规则表（gorm-adapter会自动创建，但为了确保存在，这里也创建）
CREATE TABLE IF NOT EXISTS casbin_rule (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL ,
    v0 VARCHAR(100) ,
    v1 VARCHAR(100) ,
    v2 VARCHAR(100) ,
    v3 VARCHAR(100) DEFAULT '',
    v4 VARCHAR(100) DEFAULT '',
    v5 VARCHAR(100) DEFAULT ''

)
;

-- Casbin模型配置表（存储Casbin模型规则）
CREATE TABLE IF NOT EXISTS casbin_models (
    id SERIAL PRIMARY KEY,
    section VARCHAR(50) NOT NULL ,
    "key" VARCHAR(50) NOT NULL ,
    value TEXT NOT NULL ,
    sort INTEGER DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (section, "key")

)
;

-- ============================================================================
-- Initialize Menu Data (菜单数据初始化)
-- ============================================================================

-- 初始化菜单数据
-- component 字段说明：
--   - 有实际页面的菜单：填写组件路径，如 'pages/Dashboard'（对应 src/pages/Dashboard.tsx）
--   - 分组菜单（只有子菜单，没有实际页面）：component 为空字符串 ''
--   - 路径格式：'pages/ComponentName'（不需要 .tsx 扩展名）
--   - 所有 component 路径必须在 componentMap.tsx 中注册才能正常加载
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
-- 首页分组（一级菜单）
-- 注意：系统大盘已拆分成3个独立菜单（组织大盘、应用大盘、主机大盘），保留在首页下
('menu-home', '', '', 'home', '', false, 1, '首页', 'Home', false, '', false, false, NOW(), NOW()),
-- 首页子菜单：云账单大盘（已集成到主机大盘中，隐藏独立菜单）
('menu-cloud-bill-dashboard', 'menu-home', '/cloud-bill-dashboard', 'cloudBillDashboard', 'pages/dashboard/CloudBillDashboard', true, 5, '云账单大盘', 'AccountBalance', false, '', false, false, NOW(), NOW()),
-- 首页子菜单：告警大盘（从告警中心移动到首页）
('menu-monitor-alert-dashboard', 'menu-home', '/monitors/alert-dashboard', 'monitorAlertDashboard', 'pages/monitor/AlertDashboard', false, 6, '告警大盘', 'Dashboard', false, '', false, false, NOW(), NOW()),

-- 组织管理分组（原用户权限）
('menu-user-permission', '', '', 'userPermission', '', false, 2, '组织管理', 'AccountTree', false, '', false, false, NOW(), NOW()),
-- 组织管理子菜单
('menu-departments', 'menu-user-permission', '/departments', 'departments', 'pages/user-management/Departments', false, 1, '部门管理', 'Business', false, '', false, false, NOW(), NOW()),
('menu-services', 'menu-user-permission', '/services', 'services', 'pages/user-management/Services', false, 2, '应用管理', 'Settings', false, '', false, false, NOW(), NOW()),
('menu-users', 'menu-user-permission', '/users', 'users', 'pages/user-management/Users', false, 3, '人员管理', 'People', false, '', false, false, NOW(), NOW()),
('menu-roles', 'menu-user-permission', '/roles', 'roles', 'pages/user-management/Roles', false, 4, '角色管理', 'Group', false, '', false, false, NOW(), NOW()),
('menu-permissions', 'menu-user-permission', '/permissions', 'permissions', 'pages/user-management/Permissions', false, 5, '权限管理', 'Security', false, '', false, false, NOW(), NOW()),

-- 资产管理分组（一级菜单，放在组织管理下面）
('menu-assets', '', '', 'assets', '', false, 3, '资产管理', 'Storage', false, '', false, false, NOW(), NOW()),
-- 资产管理子菜单
('menu-assets-list', 'menu-assets', '/assets', 'assets', 'pages/assets/Assets', false, 1, '资产管理', 'Storage', false, '', false, false, NOW(), NOW()),
('menu-host-groups', 'menu-assets', '/host-groups', 'hostGroups', 'pages/assets/HostGroups', false, 2, '主机分组', 'Folder', false, '', false, false, NOW(), NOW()),
('menu-asset-sync', 'menu-assets', '/asset-sync', 'assetSync', 'pages/assets/AssetSync', false, 3, '资产同步', 'Sync', false, '', false, false, NOW(), NOW()),

-- 堡垒机分组
('menu-bastion', '', '', 'bastion', '', false, 4, '堡垒机', 'Terminal', false, '', false, false, NOW(), NOW()),
-- 堡垒机子菜单
('menu-terminal', 'menu-bastion', '/terminal', 'terminal', 'pages/bastion/TerminalPage', false, 1, 'Web终端', 'Terminal', false, '', false, false, NOW(), NOW()),
('menu-sessions', 'menu-bastion', '/sessions', 'sessions', 'pages/bastion/Sessions', false, 2, '会话审计', 'PlayCircle', false, '', false, false, NOW(), NOW()),
('menu-commands', 'menu-bastion', '/commands', 'commands', 'pages/bastion/Commands', false, 3, '命令审计', 'Code', false, '', false, false, NOW(), NOW()),
('menu-history', 'menu-bastion', '/history', 'history', 'pages/bastion/History', false, 4, '登录审计', 'History', false, '', false, false, NOW(), NOW()),
('menu-system-users', 'menu-bastion', '/system-users', 'systemUsers', 'pages/bastion/SystemUsers', false, 5, '系统用户', 'Person', false, '', false, false, NOW(), NOW()),
('menu-permission-rules', 'menu-bastion', '/permission-rules', 'permissionRules', 'pages/bastion/PermissionRules', false, 6, '授权规则', 'Security', false, '', false, false, NOW(), NOW()),
('menu-blacklist', 'menu-bastion', '/blacklist', 'blacklist', 'pages/bastion/Blacklist', false, 7, '命令黑名单', 'Security', false, '', false, false, NOW(), NOW()),
('menu-bastion-settings', 'menu-bastion', '/bastion-settings', 'bastionSettings', 'pages/bastion/BastionSettings', false, 8, '堡垒机配置', 'Settings', false, '', false, false, NOW(), NOW()),

-- 发布管理一级菜单已临时移除，恢复见 sql/init_removed_release_menu.sql

-- 集群管理分组
('menu-k8s', '', '', 'k8s', '', false, 6, '集群管理', 'Cloud', false, '', false, false, NOW(), NOW()),

-- k8s 集群配置二级菜单
('menu-cluster-management', 'menu-k8s', '', 'clusterManagement', '', false, 1, '集群配置', 'Settings', false, '', false, false, NOW(), NOW()),
-- 集群配置三级菜单
('menu-cluster-list', 'menu-cluster-management', '/clusters', 'clusterList', 'pages/cluster/ClusterManagement', false, 1, '集群列表', 'ViewList', false, '', false, false, NOW(), NOW()),
-- 权限管理（可通过菜单访问，也可通过集群管理页面的权限分配按钮访问）
('menu-cluster-permissions', 'menu-cluster-management', '/cluster-permissions', 'clusterPermissions', 'pages/cluster/ClusterPermissionManagement', false, 2, '权限管理', 'Security', false, '', false, false, NOW(), NOW()),
-- 操作审计三级菜单
('menu-operation-audit', 'menu-cluster-management', '/operation-audit', 'operationAudit', 'pages/cluster/OperationAudit', false, 3, '操作审计', 'History', false, '', false, false, NOW(), NOW()),
-- 已删除 menu-k8s-events (事件管理)，功能已合并到集群管理的摘要页面中
-- 已删除 menu-cluster-summary (集群摘要)，功能已由集群管理的"状态概览"按钮替代，通过 /clusters/:id/status 访问

-- k8s 工作负载二级菜单
('menu-k8s-workload', 'menu-k8s', '', 'k8sWorkload', '', false, 2, '工作负载', 'Apps', false, '', false, false, NOW(), NOW()),
-- 工作负载三级菜单
('menu-k8s-deployments', 'menu-k8s-workload', '/k8s/deployments', 'k8sDeployments', 'pages/k8s/Deployments', false, 1, 'Deployment', 'RocketLaunch', false, '', false, false, NOW(), NOW()),
('menu-k8s-daemonsets', 'menu-k8s-workload', '/k8s/daemonsets', 'k8sDaemonSets', 'pages/k8s/DaemonSets', false, 2, 'DaemonSet', 'GridView', false, '', false, false, NOW(), NOW()),
('menu-k8s-statefulsets', 'menu-k8s-workload', '/k8s/statefulsets', 'k8sStatefulSets', 'pages/k8s/StatefulSets', false, 3, 'StatefulSet', 'Database', false, '', false, false, NOW(), NOW()),
('menu-k8s-pods', 'menu-k8s-workload', '/k8s/pods', 'k8sPods', 'pages/k8s/Pods', false, 4, 'Pod', 'Circle', false, '', false, false, NOW(), NOW()),
('menu-k8s-cronjobs', 'menu-k8s-workload', '/k8s/cronjobs', 'k8sCronJobs', 'pages/k8s/CronJobs', false, 5, 'CronJob', 'Schedule', false, '', false, false, NOW(), NOW()),
('menu-k8s-jobs', 'menu-k8s-workload', '/k8s/jobs', 'k8sJobs', 'pages/k8s/Jobs', false, 6, 'Job', 'Task', false, '', false, false, NOW(), NOW()),

-- k8s 服务发现二级菜单
('menu-k8s-service-discovery', 'menu-k8s', '', 'k8sServiceDiscovery', '', false, 8, '服务发现', 'Share', false, '', false, false, NOW(), NOW()),
-- 服务发现三级菜单
('menu-k8s-services', 'menu-k8s-service-discovery', '/k8s/services', 'k8sServices', 'pages/k8s/Services', false, 1, 'Service', 'Dns', false, '', false, false, NOW(), NOW()),
('menu-k8s-ingress', 'menu-k8s-service-discovery', '/k8s/ingress', 'k8sIngress', 'pages/k8s/Ingress', false, 2, 'Ingress', 'Router', false, '', false, false, NOW(), NOW()),

-- k8s 存储管理二级菜单
('menu-k8s-storage', 'menu-k8s', '', 'k8sStorage', '', false, 9, '存储管理', 'Storage', false, '', false, false, NOW(), NOW()),
-- 存储管理三级菜单
('menu-k8s-pv', 'menu-k8s-storage', '/k8s/pv', 'k8sPV', 'pages/k8s/PV', false, 1, 'PV', 'Folder', false, '', false, false, NOW(), NOW()),
('menu-k8s-pvc', 'menu-k8s-storage', '/k8s/pvc', 'k8sPVC', 'pages/k8s/PVC', false, 2, 'PVC', 'FolderOpen', false, '', false, false, NOW(), NOW()),
('menu-k8s-storageclass', 'menu-k8s-storage', '/k8s/storageclass', 'k8sStorageClass', 'pages/k8s/StorageClass', false, 3, 'StorageClass', 'Category', false, '', false, false, NOW(), NOW()),
('menu-k8s-configmaps', 'menu-k8s-storage', '/k8s/configmaps', 'k8sConfigMaps', 'pages/k8s/ConfigMaps', false, 4, 'ConfigMaps', 'Code', false, '', false, false, NOW(), NOW()),
('menu-k8s-secrets', 'menu-k8s-storage', '/k8s/secrets', 'k8sSecrets', 'pages/k8s/Secrets', false, 5, 'Secrets', 'VpnKey', false, '', false, false, NOW(), NOW()),

-- 工单管理分组（一级菜单）
('menu-workorder', '', '', 'workorder', '', false, 6, '工单管理', 'Assignment', false, '', false, false, NOW(), NOW()),
-- 工单管理子菜单（不包含发布工单）
('menu-daily-workorder', 'menu-workorder', '/daily-workorders', 'dailyWorkorder', 'pages/workorder/DailyWorkorder', false, 1, '运维工单', 'Assignment', false, '', false, false, NOW(), NOW()),
('menu-tickets', 'menu-workorder', '/tickets', 'tickets', 'pages/workorder/Tickets', false, 2, '我的工单', 'List', false, '', false, false, NOW(), NOW()),
('menu-form-templates', 'menu-workorder', '/form-templates', 'formTemplates', 'pages/workorder/FormTemplates', false, 3, '设计模版', 'Description', false, '', false, false, NOW(), NOW()),

-- 数据库管理分组（一级菜单）
('menu-dms', '', '', 'dms', '', false, 7, '数据库管理', 'Database', false, '', false, false, NOW(), NOW()),
-- 数据库管理二级菜单
('menu-dms-instances', 'menu-dms', '/dms/instances', 'dmsInstances', 'pages/dms/Instances', false, 1, '实例管理', 'Storage', false, '', false, false, NOW(), NOW()),
('menu-dms-query', 'menu-dms', '/dms/query', 'dmsQuery', 'pages/dms/Query', false, 2, 'SQL查询', 'Code', false, '', false, false, NOW(), NOW()),
('menu-dms-logs', 'menu-dms', '/dms/logs', 'dmsLogs', 'pages/dms/QueryLogs', false, 3, '查询日志', 'History', false, '', false, false, NOW(), NOW()),
('menu-dms-permissions', 'menu-dms', '/dms/permissions', 'dmsPermissions', 'pages/dms/Permissions', false, 4, '权限管理', 'Security', false, '', false, false, NOW(), NOW()),

-- 监控告警分组（一级菜单）
('menu-monitor', '', '', 'monitor', '', false, 8, '监控告警', 'Monitor', false, '', false, false, NOW(), NOW()),

-- 监控告警二级菜单（按功能流程分类）
-- 1. 告警中心（查看和处理告警）
('menu-monitor-alert-center', 'menu-monitor', '', 'monitorAlertCenter', '', false, 1, '告警中心', 'Alert Center', false, '', false, false, NOW(), NOW()),
-- 2. 规则配置（配置告警规则和处理逻辑）
('menu-monitor-rule-config', 'menu-monitor', '', 'monitorRuleConfig', '', false, 2, '规则配置', 'Rule Config', false, '', false, false, NOW(), NOW()),
-- 3. 通知管理（配置通知方式）
('menu-monitor-notification', 'menu-monitor', '', 'monitorNotification', '', false, 3, '通知管理', 'Notification', false, '', false, false, NOW(), NOW()),
-- 4. 值班管理（配置值班相关）
('menu-monitor-oncall', 'menu-monitor', '', 'monitorOnCall', '', false, 4, '值班管理', 'On-Call', false, '', false, false, NOW(), NOW()),

-- 监控告警三级菜单
-- 告警中心下的三级菜单（告警大盘已移动到首页）
-- ('menu-monitor-alert-dashboard', 'menu-monitor-alert-center', '/monitors/alert-dashboard', 'monitorAlertDashboard', 'pages/monitor/AlertDashboard', false, 0, '告警大盘', 'Dashboard', false, '', false, false, NOW(), NOW()),
('menu-monitor-alert-event', 'menu-monitor-alert-center', '/monitors/alert-event', 'monitorAlertEvent', 'pages/monitor/AlertEvent', false, 1, '告警事件', 'Warning', false, '', false, false, NOW(), NOW()),
('menu-monitor-strategy-log', 'menu-monitor-alert-center', '/monitors/strategy-log', 'monitorStrategyLog', 'pages/monitor/StrategyLog', false, 2, '策略日志', 'Description', false, '', false, false, NOW(), NOW()),
('menu-monitor-certificate', 'menu-monitor-alert-center', '/monitors/certificate', 'monitorCertificate', 'pages/monitor/Certificate', false, 3, '证书管理', 'VerifiedUser', false, '', false, false, NOW(), NOW()),

-- 规则配置下的三级菜单
('menu-monitor-datasource', 'menu-monitor-rule-config', '/monitors/datasource', 'monitorDatasource', 'pages/monitor/Datasource', false, 1, '数据源', 'Storage', false, '', false, false, NOW(), NOW()),
('menu-monitor-rule-group', 'menu-monitor-rule-config', '/monitors/rule-group', 'monitorRuleGroup', 'pages/monitor/RuleGroup', false, 2, '规则组', 'Folder', false, '', false, false, NOW(), NOW()),
('menu-monitor-alert-rule', 'menu-monitor-rule-config', '/monitors/alert-rule', 'monitorAlertRule', 'pages/monitor/AlertRule', false, 3, '告警规则', 'Gavel', false, '', false, false, NOW(), NOW()),
('menu-monitor-alert-strategy', 'menu-monitor-rule-config', '/monitors/alert-strategy', 'monitorAlertStrategy', 'pages/monitor/AlertStrategy', false, 4, '告警策略', 'Security', false, '', false, false, NOW(), NOW()),
('menu-monitor-restrain-rule', 'menu-monitor-rule-config', '/monitors/restrain-rule', 'monitorRestrainRule', 'pages/monitor/RestrainRule', false, 5, '告警抑制', 'Block', false, '', false, false, NOW(), NOW()),
('menu-monitor-aggregation-rule', 'menu-monitor-rule-config', '/monitors/aggregation-rule', 'monitorAggregationRule', 'pages/monitor/AggregationRule', false, 6, '告警聚合', 'CallMerge', false, '', false, false, NOW(), NOW()),

-- 通知管理下的三级菜单（已去掉告警组）
('menu-monitor-alert-template', 'menu-monitor-notification', '/monitors/alert-template', 'monitorAlertTemplate', 'pages/monitor/AlertTemplate', false, 1, '告警模板', 'Article', false, '', false, false, NOW(), NOW()),
('menu-monitor-alert-channel', 'menu-monitor-notification', '/monitors/alert-channel', 'monitorAlertChannel', 'pages/monitor/AlertChannel', false, 2, '告警渠道', 'Send', false, '', false, false, NOW(), NOW()),

-- 值班管理下的三级菜单
('menu-monitor-oncall-schedule', 'menu-monitor-oncall', '/monitors/oncall-schedule', 'monitorOnCallSchedule', 'pages/monitor/OnCallSchedule', false, 1, '值班排班', 'CalendarToday', false, '', false, false, NOW(), NOW()),
('menu-monitor-oncall-shift', 'menu-monitor-oncall', '/monitors/oncall-shift', 'monitorOnCallShift', 'pages/monitor/OnCallShift', false, 2, '值班班次', 'AccessTime', false, '', false, false, NOW(), NOW()),
('menu-monitor-oncall-calendar', 'menu-monitor-oncall', '/monitors/oncall-calendar', 'monitorOnCallCalendar', 'pages/monitor/OnCallCalendar', false, 3, '值班日历', 'CalendarMonth', false, '', false, false, NOW(), NOW()),

-- 个人设置（隐藏，只在头像菜单中显示）
('menu-personal', '', '/profile', 'personal', 'pages/personal/Profile', true, 10, '个人设置', 'Person', false, '', false, false, NOW(), NOW()),

-- 系统设置
('menu-system', '', '/settings', 'system', 'pages/system/Settings', false, 11, '系统设置', 'Settings', false, '', false, false, NOW(), NOW()),

-- AI运维助手（一级菜单）
('menu-ai-assistant', '', '', 'aiAssistant', '', false, 9, 'AI运维助手', 'SmartToy', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-chat', 'menu-ai-assistant', '/ai-assistant/chat', 'aiAssistantChat', 'pages/ai-assistant/Chat', false, 1, '智能对话', 'Chat', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-sessions', 'menu-ai-assistant', '/ai-assistant/sessions', 'aiAssistantSessions', 'pages/ai-assistant/Sessions', false, 2, '会话历史', 'History', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-schedules', 'menu-ai-assistant', '/ai-assistant/schedules', 'aiAssistantSchedules', 'pages/ai-assistant/Schedules', false, 3, '定时任务', 'Schedule', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-environments', 'menu-ai-assistant', '/ai-assistant/environments', 'aiAssistantEnvironments', 'pages/ai-assistant/Environments', false, 4, '目标环境', 'Public', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-experts', 'menu-ai-assistant', '/ai-assistant/experts', 'aiAssistantExperts', 'pages/ai-assistant/Experts', false, 5, '专家角色', 'Person', false, '', false, false, NOW(), NOW()),
('menu-ai-assistant-models', 'menu-ai-assistant', '/ai-assistant/models', 'aiAssistantModels', 'pages/ai-assistant/Models', false, 6, '模型配置', 'Psychology', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = EXCLUDED.parent_id,
  path = EXCLUDED.path,
  name = EXCLUDED.name,
  component = EXCLUDED.component,
  hidden = EXCLUDED.hidden,
  sort = EXCLUDED.sort,
  title = EXCLUDED.title,
  icon = EXCLUDED.icon,
  keep_alive = EXCLUDED.keep_alive,
  active_name = EXCLUDED.active_name,
  close_tab = EXCLUDED.close_tab,
  default_menu = EXCLUDED.default_menu,
  updated_at = NOW();

-- 更新菜单的父级和排序（如果菜单已存在，确保结构正确）
-- 确保首页菜单存在且配置正确
-- 首页菜单作为分组菜单，包含3个大盘子菜单
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at)
VALUES ('menu-home', '', '', 'home', '', false, 1, '首页', 'Home', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = '',
  path = '',
  name = 'home',
  component = '',
  hidden = false,
  sort = 1,
  title = '首页',
  icon = 'Home',
  keep_alive = false,
  active_name = '',
  close_tab = false,
  default_menu = false,
  updated_at = NOW();

-- 首页菜单包含3个大盘子菜单：组织大盘、应用大盘、主机大盘

-- 更新堡垒机的子菜单
UPDATE menus SET parent_id = 'menu-bastion', sort = 1 WHERE id = 'menu-terminal';
UPDATE menus SET parent_id = 'menu-bastion', sort = 2 WHERE id = 'menu-sessions';
UPDATE menus SET parent_id = 'menu-bastion', sort = 3 WHERE id = 'menu-commands';
UPDATE menus SET parent_id = 'menu-bastion', sort = 4 WHERE id = 'menu-history';
UPDATE menus SET parent_id = 'menu-bastion', sort = 5 WHERE id = 'menu-system-users';
UPDATE menus SET parent_id = 'menu-bastion', sort = 6 WHERE id = 'menu-permission-rules';
UPDATE menus SET parent_id = 'menu-bastion', sort = 7 WHERE id = 'menu-blacklist';
UPDATE menus SET parent_id = 'menu-bastion', sort = 8 WHERE id = 'menu-bastion-settings';

-- 隐藏云账单大盘菜单（已集成到主机大盘中）
UPDATE menus SET hidden = true WHERE id = 'menu-cloud-bill-dashboard';
-- 更新组织管理分组（原用户权限）
UPDATE menus SET title = '组织管理' WHERE id = 'menu-user-permission';

-- 更新组织管理的子菜单
UPDATE menus SET parent_id = 'menu-user-permission', sort = 1, title = '部门管理' WHERE id = 'menu-departments';
UPDATE menus SET parent_id = 'menu-user-permission', sort = 2, title = '应用管理' WHERE id = 'menu-services';
UPDATE menus SET parent_id = 'menu-user-permission', sort = 3, title = '人员管理' WHERE id = 'menu-users';
UPDATE menus SET parent_id = 'menu-user-permission', sort = 4 WHERE id = 'menu-roles';
UPDATE menus SET parent_id = 'menu-user-permission', sort = 5 WHERE id = 'menu-permissions';
-- 将资产管理移出组织管理，作为一级菜单放在组织管理下面
UPDATE menus SET parent_id = '', sort = 3 WHERE id = 'menu-assets';

-- 更新后续一级菜单的排序（资产管理插入后，后续菜单需要往后移）
UPDATE menus SET sort = 4 WHERE id = 'menu-bastion';
UPDATE menus SET sort = 5 WHERE id = 'menu-k8s';
UPDATE menus SET sort = 6 WHERE id = 'menu-workorder';
UPDATE menus SET sort = 7 WHERE id = 'menu-dms';
UPDATE menus SET sort = 8 WHERE id = 'menu-monitor';
UPDATE menus SET sort = 9 WHERE id = 'menu-system';

-- 更新资产管理的子菜单
UPDATE menus SET parent_id = 'menu-assets', sort = 1 WHERE id = 'menu-assets-list';
UPDATE menus SET parent_id = 'menu-assets', sort = 2 WHERE id = 'menu-host-groups';
UPDATE menus SET parent_id = 'menu-assets', sort = 3 WHERE id = 'menu-asset-sync';

-- 更新工单管理下子菜单的排序（确保排序正确，不包含发布工单）
UPDATE menus SET parent_id = 'menu-workorder', sort = 1 WHERE id = 'menu-daily-workorder';
UPDATE menus SET parent_id = 'menu-workorder', sort = 2 WHERE id = 'menu-tickets';
UPDATE menus SET parent_id = 'menu-workorder', sort = 3 WHERE id = 'menu-form-templates';

-- 更新工单管理的子菜单
-- 将系统大盘拆分成3个独立的二级菜单：组织大盘、应用大盘、主机大盘
-- 删除旧的系统大盘菜单，添加4个新的大盘菜单
DELETE FROM menu_permissions WHERE menu_id = 'menu-dashboard';
DELETE FROM menus WHERE id = 'menu-dashboard';

-- 插入4个新的大盘菜单，放在首页下
-- 组织大盘设置为默认首页
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
('menu-org-dashboard', 'menu-home', '/org-dashboard', 'orgDashboard', 'pages/dashboard/OrganizationDashboard', false, 1, '组织大盘', 'Business', false, '', false, true, NOW(), NOW()),
('menu-app-dashboard', 'menu-home', '/app-dashboard', 'appDashboard', 'pages/dashboard/ApplicationDashboard', false, 2, '应用大盘', 'Apps', false, '', false, false, NOW(), NOW()),
('menu-system-dashboard', 'menu-home', '/system-dashboard', 'systemDashboard', 'pages/dashboard/SystemDashboard', false, 3, '主机大盘', 'Computer', false, '', false, false, NOW(), NOW()),
('menu-k8s-dashboard', 'menu-home', '/k8s-dashboard', 'k8sDashboard', 'pages/k8s/K8sDashboard', false, 4, 'K8s大盘', 'Dashboard', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET updated_at = NOW();

-- 确保组织大盘是默认首页（如果菜单已存在，更新 default_menu 字段）
-- 同时确保其他大盘菜单不是默认首页
UPDATE menus SET default_menu = true WHERE id = 'menu-org-dashboard';
UPDATE menus SET default_menu = false WHERE id IN ('menu-app-dashboard', 'menu-system-dashboard');

-- 删除发布工单菜单及其权限（如果存在，因为暂时不添加）
DELETE FROM menu_permissions WHERE menu_id = 'menu-create-ticket';
DELETE FROM menus WHERE id = 'menu-create-ticket';

-- 删除全部工单、我的工单和工单配置菜单及其权限（已合并为工单列表）
DELETE FROM menu_permissions WHERE menu_id IN ('menu-all-tickets', 'menu-my-tickets', 'menu-approval-config');
DELETE FROM menus WHERE id IN ('menu-all-tickets', 'menu-my-tickets', 'menu-approval-config');

-- 删除配置管理菜单及其权限（如果存在）；发布配置、应用-发布已迁移到发布管理一级菜单下，不删除
DELETE FROM menu_permissions WHERE menu_id IN (
    'menu-config', 'menu-config-jenkins', 'menu-config-argocd'
);
DELETE FROM menus WHERE id IN (
    'menu-config', 'menu-config-jenkins', 'menu-config-argocd'
);

-- 删除账单大盘菜单及其权限（如果存在）
DELETE FROM menu_permissions WHERE menu_id = 'menu-bill-dashboard';
DELETE FROM menus WHERE id = 'menu-bill-dashboard';

-- 更新Kubernetes管理的子菜单
-- 清理已删除的菜单项（如果数据库中已存在）
DELETE FROM menu_permissions WHERE menu_id = 'menu-k8s-overview';
DELETE FROM menus WHERE id = 'menu-k8s-overview';
DELETE FROM menu_permissions WHERE menu_id = 'menu-cluster-summary';
DELETE FROM menus WHERE id = 'menu-cluster-summary';

-- 更新菜单结构：恢复集群配置二级菜单，将三个菜单改回三级菜单
-- 删除旧的二级菜单（如果存在，只删除工作负载）
DELETE FROM menu_permissions WHERE menu_id = 'menu-k8s-workloads';
DELETE FROM menus WHERE id = 'menu-k8s-workloads';

-- 确保集群配置二级菜单存在（分组菜单，path和component必须为空字符串，不能是NULL）
-- 如果菜单不存在则插入，如果存在则更新
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
('menu-cluster-management', 'menu-k8s', '', 'clusterManagement', '', false, 1, '集群配置', 'Settings', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = 'menu-k8s',
  path = '',
  name = 'clusterManagement',
  component = '',
  hidden = false,
  sort = 1,
  title = '集群配置',
  icon = 'Settings',
  updated_at = NOW();

-- 确保工作负载二级菜单存在（分组菜单，path和component必须为空字符串，不能是NULL）
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
('menu-k8s-workload', 'menu-k8s', '', 'k8sWorkload', '', false, 2, '工作负载', 'Apps', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = 'menu-k8s',
  path = '',
  name = 'k8sWorkload',
  component = '',
  hidden = false,
  sort = 2,
  title = '工作负载',
  icon = 'Apps',
  updated_at = NOW();

-- 更新工作负载下的三级菜单（确保菜单存在且配置正确）
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
('menu-k8s-deployments', 'menu-k8s-workload', '/k8s/deployments', 'k8sDeployments', 'pages/k8s/Deployments', false, 1, 'Deployment', 'RocketLaunch', false, '', false, false, NOW(), NOW()),
('menu-k8s-daemonsets', 'menu-k8s-workload', '/k8s/daemonsets', 'k8sDaemonSets', 'pages/k8s/DaemonSets', false, 2, 'DaemonSet', 'GridView', false, '', false, false, NOW(), NOW()),
('menu-k8s-statefulsets', 'menu-k8s-workload', '/k8s/statefulsets', 'k8sStatefulSets', 'pages/k8s/StatefulSets', false, 3, 'StatefulSet', 'Database', false, '', false, false, NOW(), NOW()),
('menu-k8s-pods', 'menu-k8s-workload', '/k8s/pods', 'k8sPods', 'pages/k8s/Pods', false, 4, 'Pod', 'Circle', false, '', false, false, NOW(), NOW()),
('menu-k8s-cronjobs', 'menu-k8s-workload', '/k8s/cronjobs', 'k8sCronJobs', 'pages/k8s/CronJobs', false, 5, 'CronJob', 'Schedule', false, '', false, false, NOW(), NOW()),
('menu-k8s-jobs', 'menu-k8s-workload', '/k8s/jobs', 'k8sJobs', 'pages/k8s/Jobs', false, 6, 'Job', 'Task', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = EXCLUDED.parent_id,
  path = EXCLUDED.path,
  name = EXCLUDED.name,
  component = EXCLUDED.component,
  hidden = EXCLUDED.hidden,
  sort = EXCLUDED.sort,
  title = EXCLUDED.title,
  icon = EXCLUDED.icon,
  updated_at = NOW();

-- 更新集群配置下的三级菜单（确保菜单存在且配置正确）
-- 使用 INSERT ... ON CONFLICT (id) DO UPDATE SET 确保菜单存在且 parent_id 正确
INSERT INTO menus (id, parent_id, path, name, component, hidden, sort, title, icon, keep_alive, active_name, close_tab, default_menu, created_at, updated_at) VALUES
('menu-cluster-list', 'menu-cluster-management', '/clusters', 'clusterList', 'pages/cluster/ClusterManagement', false, 1, '集群列表', 'ViewList', false, '', false, false, NOW(), NOW()),
('menu-cluster-permissions', 'menu-cluster-management', '/cluster-permissions', 'clusterPermissions', 'pages/cluster/ClusterPermissionManagement', false, 2, '权限管理', 'Security', false, '', false, false, NOW(), NOW()),
('menu-operation-audit', 'menu-cluster-management', '/operation-audit', 'operationAudit', 'pages/cluster/OperationAudit', false, 3, '操作审计', 'History', false, '', false, false, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  parent_id = EXCLUDED.parent_id,
  path = EXCLUDED.path,
  name = EXCLUDED.name,
  component = EXCLUDED.component,
  hidden = EXCLUDED.hidden,
  sort = EXCLUDED.sort,
  title = EXCLUDED.title,
  icon = EXCLUDED.icon,
  updated_at = NOW();

-- 更新工作负载二级菜单
UPDATE menus SET parent_id = 'menu-k8s', path = '', name = 'k8sWorkload', component = '', hidden = false, sort = 2, title = '工作负载', icon = 'Apps' WHERE id = 'menu-k8s-workload';

-- 更新工作负载下的三级菜单
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/deployments', name = 'k8sDeployments', component = 'pages/k8s/Deployments', hidden = false, sort = 1, title = 'Deployment', icon = 'RocketLaunch' WHERE id = 'menu-k8s-deployments';
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/daemonsets', name = 'k8sDaemonSets', component = 'pages/k8s/DaemonSets', hidden = false, sort = 2, title = 'DaemonSet', icon = 'GridView' WHERE id = 'menu-k8s-daemonsets';
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/statefulsets', name = 'k8sStatefulSets', component = 'pages/k8s/StatefulSets', hidden = false, sort = 3, title = 'StatefulSet', icon = 'Database' WHERE id = 'menu-k8s-statefulsets';
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/pods', name = 'k8sPods', component = 'pages/k8s/Pods', hidden = false, sort = 4, title = 'Pod', icon = 'Circle' WHERE id = 'menu-k8s-pods';
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/cronjobs', name = 'k8sCronJobs', component = 'pages/k8s/CronJobs', hidden = false, sort = 5, title = 'CronJob', icon = 'Schedule' WHERE id = 'menu-k8s-cronjobs';
UPDATE menus SET parent_id = 'menu-k8s-workload', path = '/k8s/jobs', name = 'k8sJobs', component = 'pages/k8s/Jobs', hidden = false, sort = 6, title = 'Job', icon = 'Task' WHERE id = 'menu-k8s-jobs';

-- 更新服务发现、存储管理的排序
UPDATE menus SET sort = 8 WHERE id = 'menu-k8s-service-discovery';
UPDATE menus SET sort = 9 WHERE id = 'menu-k8s-storage';

-- 更新剩余菜单的排序（注意：原工作负载和集群配置下的菜单已提升为二级菜单，服务发现、存储管理下的菜单仍在INSERT语句中设置parent_id）

-- 删除Istio菜单及其权限（如果存在）
DELETE FROM menu_permissions WHERE menu_id IN (
    'menu-k8s-istio', 'menu-k8s-destination-rules', 'menu-k8s-gateways', 'menu-k8s-virtual-services'
);
DELETE FROM menus WHERE id IN (
    'menu-k8s-istio', 'menu-k8s-destination-rules', 'menu-k8s-gateways', 'menu-k8s-virtual-services'
);

-- 删除Jenkins菜单及其子菜单
DELETE FROM menu_permissions WHERE menu_id IN ('menu-deployment', 'menu-jenkins-server-management', 'menu-jenkins-deploy', 'menu-jenkins-task-management', 'menu-deployment-history');
DELETE FROM menus WHERE id IN ('menu-deployment', 'menu-jenkins-server-management', 'menu-jenkins-deploy', 'menu-jenkins-task-management', 'menu-deployment-history');
-- menu-k8s-deploy已合并到menu-jenkins-deploy，删除该菜单
-- 删除菜单权限关联（如果menu_permissions表存在）
DELETE FROM menu_permissions WHERE menu_id = 'menu-k8s-deploy';
-- 删除菜单本身
DELETE FROM menus WHERE id = 'menu-k8s-deploy';

-- 更新监控告警菜单结构（实施三级菜单）
-- 删除已废弃的菜单权限关联（保留 menu-monitor-datasource，因为现在要恢复它）
DELETE FROM menu_permissions WHERE menu_id IN ('menu-monitor-silence-rule');
-- 删除已废弃的菜单（保留 menu-monitor-datasource，因为现在要恢复它）
DELETE FROM menus WHERE id IN ('menu-monitor-silence-rule');

-- 注意：不再删除所有 menu-monitor-% 菜单，因为新的二级和三级菜单已经通过 INSERT 语句创建
-- 旧的菜单会被新的 INSERT 语句覆盖（使用 ON CONFLICT (id) DO UPDATE SET）

-- 删除账单管理菜单及其权限（如果存在）
DELETE FROM menu_permissions WHERE menu_id IN (
    'menu-bill', 'menu-bill-records', 'menu-bill-summary', 'menu-bill-statistics',
    'menu-bill-vm', 'menu-bill-price', 'menu-bill-resource'
);
DELETE FROM menus WHERE id IN (
    'menu-bill', 'menu-bill-records', 'menu-bill-summary', 'menu-bill-statistics',
    'menu-bill-vm', 'menu-bill-price', 'menu-bill-resource'
);

-- 删除已移除的 menu-monitor-record-rule 菜单及其权限（如果存在）
DELETE FROM menu_permissions WHERE menu_id = 'menu-monitor-record-rule';
DELETE FROM menus WHERE id = 'menu-monitor-record-rule';

-- 删除告警组菜单（功能已去掉）
DELETE FROM menu_permissions WHERE menu_id = 'menu-monitor-alert-group';
DELETE FROM menus WHERE id = 'menu-monitor-alert-group';

-- 确保顶级菜单的 parent_id 为空字符串
-- 注意：这个 UPDATE 必须在更新子菜单之后执行，确保顶级菜单的 parent_id 为空
-- 资产管理已移出组织管理作为一级菜单
UPDATE menus SET parent_id = '' WHERE id IN (
    'menu-home',
    'menu-user-permission',
    'menu-assets',
    'menu-bastion',
    'menu-k8s',
    'menu-workorder',
    'menu-dms',
    'menu-deployment',
    'menu-monitor',
    'menu-personal',
    'menu-system'
);

-- 首页菜单包含3个大盘子菜单，不需要隐藏

-- 更新用户管理分组标题
UPDATE menus SET title = '组织管理' WHERE id = 'menu-user-permission';

-- ============================================================================
-- 为admin角色分配所有菜单权限
-- ============================================================================
-- 先删除可能存在的旧权限（如果有），然后重新分配所有菜单权限
-- 这样可以确保 admin 始终拥有所有菜单权限，即使菜单有更新
-- 注意：此逻辑已集成到初始化脚本中，无需单独执行 fix_admin_permissions.sql
-- ============================================================================
DELETE FROM menu_permissions WHERE role_id = 'role:admin';
INSERT INTO menu_permissions (role_id, menu_id, created_at)
SELECT 'role:admin', menus.id, NOW() FROM menus
ON CONFLICT (role_id, menu_id) DO NOTHING;
-- 确保模型配置菜单对 admin 可见
INSERT INTO menu_permissions (role_id, menu_id, created_at) VALUES
('role:admin', 'menu-ai-assistant-models', NOW())
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- 为user角色分配基础菜单权限
-- 默认普通用户只能看到首页相关菜单和个人设置，其他菜单需要通过菜单授权功能才能看到
-- 先删除user角色的所有菜单权限
DELETE FROM menu_permissions WHERE role_id = 'role:user';

-- 为user角色分配首页相关菜单权限和个人设置（只分配首页及其子菜单和个人设置）
-- 其他菜单需要通过菜单授权功能才能看到
INSERT INTO menu_permissions (role_id, menu_id, created_at)
SELECT 'role:user', menus.id, NOW() FROM menus
WHERE menus.id IN (
    'menu-home',
    'menu-org-dashboard',
    'menu-app-dashboard',
    'menu-system-dashboard',
    'menu-k8s-dashboard',
    'menu-personal'
)
ON CONFLICT (role_id, menu_id) DO UPDATE SET created_at = NOW();

-- ============================================================================
-- Initialize API Data
-- ============================================================================

-- 初始化API数据（用于API权限管理）
INSERT INTO apis (path, method, "group", description, created_at, updated_at) VALUES
-- Dashboard APIs
('/dashboard/stats', 'GET', 'Dashboard', '获取仪表板统计信息', NOW(), NOW()),
('/dashboard/recent-logins', 'GET', 'Dashboard', '获取最近登录记录', NOW(), NOW()),
('/dashboard/frequent-hosts', 'GET', 'Dashboard', '获取常用主机', NOW(), NOW()),

-- Host APIs
('/hosts', 'GET', 'Assets', '获取主机列表', NOW(), NOW()),
('/hosts', 'POST', 'Assets', '创建主机', NOW(), NOW()),
('/hosts/:id', 'GET', 'Assets', '获取主机详情', NOW(), NOW()),
('/hosts/:id', 'PUT', 'Assets', '更新主机', NOW(), NOW()),
('/hosts/:id', 'DELETE', 'Assets', '删除主机', NOW(), NOW()),
('/hosts/:id/test', 'POST', 'Assets', '测试主机连接', NOW(), NOW()),
('/hosts/:id/check-status', 'POST', 'Assets', '检查主机状态', NOW(), NOW()),
('/hosts/check-all-status', 'POST', 'Assets', '检查所有主机状态', NOW(), NOW()),

-- Host Groups APIs
('/host-groups', 'GET', 'Assets', '获取主机分组列表', NOW(), NOW()),
('/host-groups', 'POST', 'Assets', '创建主机分组', NOW(), NOW()),
('/host-groups/:id', 'GET', 'Assets', '获取主机分组详情', NOW(), NOW()),
('/host-groups/:id', 'PUT', 'Assets', '更新主机分组', NOW(), NOW()),
('/host-groups/:id', 'DELETE', 'Assets', '删除主机分组', NOW(), NOW()),
('/host-groups/:id/hosts', 'GET', 'Assets', '获取分组中的主机', NOW(), NOW()),
('/host-groups/:id/hosts', 'POST', 'Assets', '添加主机到分组', NOW(), NOW()),
('/host-groups/:id/hosts', 'DELETE', 'Assets', '从分组移除主机', NOW(), NOW()),

-- Terminal APIs
('/sessions', 'POST', 'Bastion', '创建会话', NOW(), NOW()),
('/sessions/records', 'GET', 'Bastion', '获取登录记录', NOW(), NOW()),

-- Session Management APIs (Admin)
('/sessions/recordings', 'GET', 'Bastion', '获取会话录制列表', NOW(), NOW()),
('/sessions/recordings/:sessionId', 'GET', 'Bastion', '获取会话录制详情', NOW(), NOW()),
('/sessions/recordings/:sessionId/file', 'GET', 'Bastion', '获取会话录制文件', NOW(), NOW()),
('/sessions/:sessionId/terminate', 'DELETE', 'Bastion', '终止会话', NOW(), NOW()),

-- Command Audit APIs (Admin)
('/commands', 'GET', 'Bastion', '获取命令记录', NOW(), NOW()),
('/commands', 'POST', 'Bastion', '创建命令记录', NOW(), NOW()),
('/commands/session/:sessionId', 'GET', 'Bastion', '获取会话的命令记录', NOW(), NOW()),

-- History APIs
('/auth/login-records', 'GET', 'Bastion', '获取平台登录记录', NOW(), NOW()),

-- Organization Management APIs (Admin) - 组织管理API
('/user-management/users', 'GET', 'Organization', '获取用户列表', NOW(), NOW()),
('/user-management/users', 'POST', 'Organization', '创建用户', NOW(), NOW()),
('/user-management/users/:id', 'GET', 'Organization', '获取用户详情', NOW(), NOW()),
('/user-management/users/:id', 'PUT', 'Organization', '更新用户', NOW(), NOW()),
('/user-management/users/:id', 'DELETE', 'Organization', '删除用户', NOW(), NOW()),
('/user-management/users/:id/role', 'PUT', 'Organization', '更新用户角色', NOW(), NOW()),
('/user-management/users/:id/status', 'PUT', 'Organization', '更新用户状态', NOW(), NOW()),
('/user-management/users/:id/reset-password', 'POST', 'Organization', '重置用户密码', NOW(), NOW()),
('/user-management/users/:id/groups', 'GET', 'Organization', '获取用户分组权限', NOW(), NOW()),
('/user-management/users/:id/groups', 'POST', 'Organization', '分配分组权限', NOW(), NOW()),

-- Role APIs
('/roles', 'GET', 'Role', '获取角色列表', NOW(), NOW()),
('/roles', 'POST', 'Role', '创建角色', NOW(), NOW()),
('/roles/:id', 'GET', 'Role', '获取角色详情', NOW(), NOW()),
('/roles/:id', 'PUT', 'Role', '更新角色', NOW(), NOW()),
('/roles/:id', 'DELETE', 'Role', '删除角色', NOW(), NOW()),
('/roles/:id/members', 'GET', 'Role', '获取角色成员', NOW(), NOW()),
('/roles/:id/members', 'POST', 'Role', '添加成员', NOW(), NOW()),
('/roles/:id/members/:userId', 'DELETE', 'Role', '移除成员', NOW(), NOW()),
('/roles/:id/members/batch', 'POST', 'Role', '批量添加成员', NOW(), NOW()),

-- System User APIs
('/system-users', 'GET', 'User', '获取系统用户列表', NOW(), NOW()),
('/system-users', 'POST', 'User', '创建系统用户', NOW(), NOW()),
('/system-users/:id', 'GET', 'User', '获取系统用户详情', NOW(), NOW()),
('/system-users/:id', 'PUT', 'User', '更新系统用户', NOW(), NOW()),
('/system-users/:id', 'DELETE', 'User', '删除系统用户', NOW(), NOW()),

-- Permission Rules APIs (Admin)
('/permission-rules', 'GET', 'Permission', '获取授权规则列表', NOW(), NOW()),
('/permission-rules', 'POST', 'Permission', '创建授权规则', NOW(), NOW()),
('/permission-rules/:id', 'GET', 'Permission', '获取授权规则详情', NOW(), NOW()),
('/permission-rules/:id', 'PUT', 'Permission', '更新授权规则', NOW(), NOW()),
('/permission-rules/:id', 'DELETE', 'Permission', '删除授权规则', NOW(), NOW()),

-- Approval APIs
('/approvals', 'GET', 'Approval', '获取审批列表', NOW(), NOW()),
('/approvals', 'POST', 'Approval', '创建审批申请', NOW(), NOW()),
('/approvals/stats', 'GET', 'Approval', '获取审批统计', NOW(), NOW()),
('/approvals/config', 'GET', 'Approval', '获取审批配置', NOW(), NOW()),
('/approvals/:id', 'GET', 'Approval', '获取审批详情', NOW(), NOW()),
('/approvals/:id/approve', 'POST', 'Approval', '批准审批', NOW(), NOW()),
('/approvals/:id/reject', 'POST', 'Approval', '拒绝审批', NOW(), NOW()),
('/approvals/:id/cancel', 'POST', 'Approval', '取消审批', NOW(), NOW()),
('/approvals/:id/comments', 'POST', 'Approval', '添加评论', NOW(), NOW()),

-- Approval Config APIs (Admin)
('/approvals/config', 'POST', 'Approval', '创建审批配置', NOW(), NOW()),
('/approvals/config/:id', 'PUT', 'Approval', '更新审批配置', NOW(), NOW()),
('/approvals/config/:id', 'DELETE', 'Approval', '删除审批配置', NOW(), NOW()),

-- K8s APIs (Kubernetes管理)
('/api/v1/kube/base', 'GET', 'K8S', '获取Kubernetes基础信息', NOW(), NOW()),
('/api/v1/kube/deployment', 'GET', 'K8S', '获取Deployment列表', NOW(), NOW()),
('/api/v1/kube/daemonset', 'GET', 'K8S', '获取DaemonSet列表', NOW(), NOW()),
('/api/v1/kube/statefulset', 'GET', 'K8S', '获取StatefulSet列表', NOW(), NOW()),
('/api/v1/kube/service', 'GET', 'K8S', '获取Service列表', NOW(), NOW()),
('/api/v1/kube/ingress', 'GET', 'K8S', '获取Ingress列表', NOW(), NOW()),
('/api/v1/kube/cronjob', 'GET', 'K8S', '获取CronJob列表', NOW(), NOW()),
('/api/v1/kube/job', 'GET', 'K8S', '获取Job列表', NOW(), NOW()),
('/api/v1/kube/event', 'GET', 'K8S', '获取Event列表', NOW(), NOW()),
('/api/v1/kube/containers', 'GET', 'K8S', '获取容器列表', NOW(), NOW()),
('/api/v1/kube/scale', 'POST', 'K8S', '扩缩容副本', NOW(), NOW()),
('/api/v1/kube/scale', 'GET', 'K8S', '获取副本数', NOW(), NOW()),
('/api/v1/kube/pod', 'GET', 'K8S', '获取Pod列表', NOW(), NOW()),
('/api/v1/kube/pod', 'DELETE', 'K8S', '重启Pod', NOW(), NOW()),
('/api/v1/kube/pod/down_logs', 'GET', 'K8S', '下载容器日志', NOW(), NOW()),
('/api/v1/kube/pod/metrics', 'GET', 'K8S', '获取Pod指标', NOW(), NOW()),
('/api/v1/kube/pod/ws/logs', 'GET', 'K8S', 'WebSocket获取容器日志', NOW(), NOW()),
('/api/v1/kube/pod/ws/exec', 'GET', 'K8S', 'WebSocket执行容器命令', NOW(), NOW()),

-- Blacklist APIs (Admin)
('/proxy/blacklist/commands', 'GET', 'Security', '获取命令黑名单', NOW(), NOW()),
('/proxy/blacklist/commands', 'POST', 'Security', '创建命令黑名单规则', NOW(), NOW()),
('/proxy/blacklist/commands/:id', 'PATCH', 'Security', '更新命令黑名单规则', NOW(), NOW()),
('/proxy/blacklist/commands/:id', 'DELETE', 'Security', '删除命令黑名单规则', NOW(), NOW()),

-- Settings APIs (Admin)
('/settings', 'GET', 'System', '获取所有设置', NOW(), NOW()),
('/settings/:category', 'GET', 'System', '根据分类获取设置', NOW(), NOW()),
('/settings', 'PUT', 'System', '批量更新设置', NOW(), NOW()),
('/settings/item', 'PUT', 'System', '更新单个设置', NOW(), NOW()),
('/settings/:key', 'DELETE', 'System', '删除设置', NOW(), NOW()),
('/settings/test-ldap', 'POST', 'System', '测试LDAP连接', NOW(), NOW()),
('/settings/test-sso', 'POST', 'System', '测试SSO配置', NOW(), NOW()),
('/settings/test-feishu', 'POST', 'System', '测试飞书通知', NOW(), NOW()),
('/settings/test-dingtalk', 'POST', 'System', '测试钉钉通知', NOW(), NOW()),
('/settings/test-wechat', 'POST', 'System', '测试企业微信通知', NOW(), NOW()),

-- Asset Sync APIs (Admin)
('/asset-sync/configs', 'GET', 'System', '获取资产同步配置', NOW(), NOW()),
('/asset-sync/configs', 'POST', 'System', '创建资产同步配置', NOW(), NOW()),
('/asset-sync/configs/:id', 'PUT', 'System', '更新资产同步配置', NOW(), NOW()),
('/asset-sync/configs/:id', 'DELETE', 'System', '删除资产同步配置', NOW(), NOW()),
('/asset-sync/configs/:id/toggle', 'POST', 'System', '启用/禁用配置', NOW(), NOW()),
('/asset-sync/configs/:id/sync', 'POST', 'System', '立即同步', NOW(), NOW()),
('/asset-sync/logs', 'GET', 'System', '获取同步日志', NOW(), NOW()),

-- Host Monitor APIs
-- Note: These APIs are already defined above in Host APIs section, skipping duplicates

-- Two Factor APIs
('/two-factor/status', 'GET', 'Security', '获取用户2FA状态', NOW(), NOW()),
('/two-factor/global-status', 'GET', 'Security', '获取全局2FA状态', NOW(), NOW()),
('/two-factor/setup', 'POST', 'Security', '设置2FA', NOW(), NOW()),
('/two-factor/verify', 'POST', 'Security', '验证2FA', NOW(), NOW()),
('/two-factor/disable', 'POST', 'Security', '禁用2FA', NOW(), NOW()),
('/two-factor/verify-code', 'POST', 'Security', '验证2FA代码', NOW(), NOW()),
('/two-factor/backup-codes', 'GET', 'Security', '获取备用码', NOW(), NOW()),
('/two-factor/regenerate-backup-codes', 'POST', 'Security', '重新生成备用码', NOW(), NOW()),

-- Admin Two Factor APIs
('/admin/two-factor/config', 'GET', 'Security', '获取全局2FA配置', NOW(), NOW()),
('/admin/two-factor/config', 'PUT', 'Security', '更新全局2FA配置', NOW(), NOW()),
('/admin/two-factor/reset/:userId', 'POST', 'Security', '重置用户2FA', NOW(), NOW()),

-- Routing APIs
('/routing/config', 'GET', 'System', '获取路由配置', NOW(), NOW()),
('/routing/config', 'PUT', 'System', '更新路由配置', NOW(), NOW()),
('/routing/proxies', 'GET', 'System', '获取可用代理列表', NOW(), NOW()),
('/hosts/:id/route', 'GET', 'System', '获取主机的路由决策', NOW(), NOW()),

-- File Transfer APIs
('/files/upload', 'POST', 'Bastion', '上传文件', NOW(), NOW()),
('/files/download', 'POST', 'Bastion', '下载文件', NOW(), NOW()),
('/files/transfers', 'GET', 'Bastion', '获取文件传输记录', NOW(), NOW()),

-- User SSH Key APIs (Organization Management)
('/user-management/users/:id/ssh-key/generate', 'POST', 'Organization', '生成SSH密钥', NOW(), NOW()),
('/user-management/users/:id/ssh-key', 'DELETE', 'Organization', '删除SSH密钥', NOW(), NOW()),
('/user-management/users/:id/ssh-key/download', 'GET', 'Organization', '下载私钥', NOW(), NOW()),
('/user-management/users/:id/auth-method', 'PUT', 'Organization', '更新认证方式', NOW(), NOW()),

-- Permission Management APIs (Admin)
('/permissions/menus', 'GET', 'Permission', '获取所有菜单', NOW(), NOW()),
('/permissions/menus', 'POST', 'Permission', '创建菜单', NOW(), NOW()),
('/permissions/menus/:id', 'PUT', 'Permission', '更新菜单', NOW(), NOW()),
('/permissions/menus/:id', 'DELETE', 'Permission', '删除菜单', NOW(), NOW()),
('/permissions/menus/user-group/:id', 'GET', 'Permission', '获取用户组的菜单权限', NOW(), NOW()),
('/permissions/menus/user-group/:id', 'PUT', 'Permission', '更新用户组的菜单权限', NOW(), NOW()),
('/permissions/menus/role/:role', 'GET', 'Permission', '获取角色的菜单权限', NOW(), NOW()),
('/permissions/menus/role/:role', 'PUT', 'Permission', '更新角色的菜单权限', NOW(), NOW()),
('/permissions/apis', 'GET', 'Permission', '获取所有API', NOW(), NOW()),
('/permissions/apis', 'POST', 'Permission', '创建API', NOW(), NOW()),
('/permissions/apis/:id', 'PUT', 'Permission', '更新API', NOW(), NOW()),
('/permissions/apis/:id', 'DELETE', 'Permission', '删除API', NOW(), NOW()),
('/permissions/apis/groups', 'GET', 'Permission', '获取API分组', NOW(), NOW()),
('/permissions/apis/user-group/:id', 'GET', 'Permission', '获取用户组的API权限', NOW(), NOW()),
('/permissions/apis/user-group/:id', 'PUT', 'Permission', '更新用户组的API权限', NOW(), NOW()),
('/permissions/apis/role/:role', 'GET', 'Permission', '获取角色的API权限', NOW(), NOW()),
('/permissions/apis/role/:role', 'PUT', 'Permission', '更新角色的API权限', NOW(), NOW())
ON CONFLICT (path, method) DO UPDATE SET updated_at = NOW();

-- 更新API分组：将组织管理相关的API从'User'改为'Organization'
UPDATE apis SET "group" = 'Organization' WHERE "group" = 'User' AND path LIKE '/user-management/%';

-- ============================================================================
-- Bill Management Tables (账单管理表)
-- ============================================================================

-- 月度汇总账单表
CREATE TABLE IF NOT EXISTS bill_summary (
    id SERIAL PRIMARY KEY ,
    vendor VARCHAR(50) NOT NULL ,
    cycle VARCHAR(10) NOT NULL ,
    consume_amount DECIMAL(25,15) DEFAULT 0 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    UNIQUE (vendor, cycle)

)
;

-- 月度汇总账单详情表
CREATE TABLE IF NOT EXISTS bill_summary_detail (
    id SERIAL PRIMARY KEY ,
    resource_type VARCHAR(50) ,
    resource_code VARCHAR(50) ,
    service_type VARCHAR(50) ,
    service_code VARCHAR(50) ,
    consume_amount DECIMAL(25,15) DEFAULT 0 ,
    summary_id INTEGER NOT NULL ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (summary_id) REFERENCES bill_summary(id) ON DELETE CASCADE
)
;

-- 账单消费记录表
CREATE TABLE IF NOT EXISTS bill_records (
    id SERIAL PRIMARY KEY ,
    vendor VARCHAR(50) NOT NULL ,
    cycle VARCHAR(10) NOT NULL ,
    instance_id VARCHAR(200) ,
    resource_name VARCHAR(200) ,
    spec_desc TEXT ,
    consume_amount DECIMAL(25,15) DEFAULT 0 ,
    resource_type VARCHAR(50) ,
    resource_code VARCHAR(50) ,
    service_type VARCHAR(50) ,
    service_code VARCHAR(50) ,
    extra TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 单价管理表（适用于专有云）
CREATE TABLE IF NOT EXISTS bill_price (
    id SERIAL PRIMARY KEY ,
    vendor VARCHAR(50) NOT NULL ,
    resource_type VARCHAR(50) ,
    scale VARCHAR(50) ,
    cluster VARCHAR(50) ,
    price DECIMAL(25,15) DEFAULT 0 ,
    description TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- Monitor Management Tables (监控管理表)
-- ============================================================================

-- 监控查询语句表（Prometheus 监控表达式）
CREATE TABLE IF NOT EXISTS monitors (
    id SERIAL PRIMARY KEY ,
    name VARCHAR(100) NOT NULL ,
    expr TEXT NOT NULL ,
    created_by VARCHAR(36) ,
    updated_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    UNIQUE (name)

)
;

-- ============================================================================
-- Alert Management Tables (告警管理表)
-- ============================================================================

-- ============================================================================
-- Alert Rule Groups (告警规则组管理)
-- ============================================================================

-- ============================================================================
-- Alert Management Tables (告警管理表)
-- ============================================================================

-- 告警规则数据源表（必须先创建，因为规则组会引用它）
CREATE TABLE IF NOT EXISTS alert_rule_sources (
    id SERIAL PRIMARY KEY ,
    source_name VARCHAR(100) NOT NULL ,
    source_type VARCHAR(100) NOT NULL ,
    address VARCHAR(500) NOT NULL ,
    api_key VARCHAR(200) ,
    auto_sync BOOLEAN DEFAULT TRUE ,
    sync_interval INTEGER DEFAULT 10 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    UNIQUE (source_name)

)
;

-- 告警规则组表（关联数据源）
CREATE TABLE IF NOT EXISTS alert_rule_groups (
    id SERIAL PRIMARY KEY ,
    group_name VARCHAR(100) NOT NULL UNIQUE ,
    description TEXT ,
    department_id VARCHAR(36) ,
    source_id INTEGER NOT NULL ,
    file VARCHAR(500) ,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES alert_rule_sources(id) ON DELETE CASCADE
);

-- 告警组表（用于通知配置）
CREATE TABLE IF NOT EXISTS alert_groups (
    id SERIAL PRIMARY KEY ,
    group_name VARCHAR(100) NOT NULL UNIQUE ,
    description TEXT ,
    department_id VARCHAR(36) ,
    members JSONB ,
    uid INTEGER ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- 告警规则表
CREATE TABLE IF NOT EXISTS alert_rules (
    id SERIAL PRIMARY KEY ,
    name VARCHAR(500) NOT NULL ,
    group_id INTEGER ,
    "group" VARCHAR(100) ,
    expr TEXT NOT NULL ,
    duration INTEGER DEFAULT 0 ,
    labels JSONB ,
    annotations JSONB ,
    health VARCHAR(100) DEFAULT 'unknown' ,
    source_id INTEGER NOT NULL ,
    enabled BOOLEAN DEFAULT TRUE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (name, group_id, source_id),
    FOREIGN KEY (source_id) REFERENCES alert_rule_sources(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES alert_rule_groups(id) ON DELETE SET NULL
)
;

-- 告警事件表
CREATE TABLE IF NOT EXISTS alert_events (
    id SERIAL PRIMARY KEY ,
    alert_title VARCHAR(200) ,
    source_id INTEGER NOT NULL ,
    description VARCHAR(500) ,
    level INTEGER NOT NULL ,
    first_trigger_time TIMESTAMP ,
    first_ack_time TIMESTAMP ,
    trigger_time TIMESTAMP ,
    recover_time TIMESTAMP ,
    annotations JSONB ,
    is_recovered BOOLEAN DEFAULT FALSE ,
    progress SMALLINT DEFAULT 1 ,
    uid VARCHAR(36) ,
    tags JSONB ,
    finger_print VARCHAR(100) ,
    source_ip VARCHAR(50) ,
    department_id VARCHAR(36) ,
    integration_id INTEGER ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 告警日志表
CREATE TABLE IF NOT EXISTS alert_logs (
    id SERIAL PRIMARY KEY ,
    alert_id BIGINT NOT NULL ,
    action VARCHAR(20) ,
    uid VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (alert_id) REFERENCES alert_events(id) ON DELETE CASCADE
);

-- 告警策略表
CREATE TABLE IF NOT EXISTS alert_strategies (
    id SERIAL PRIMARY KEY ,
    strategy_name VARCHAR(100) NOT NULL UNIQUE ,
    department_id VARCHAR(36) ,
    template_id INTEGER ,
    status VARCHAR(20) DEFAULT 'enabled' ,
    weight INTEGER DEFAULT 0 ,
    continuous BOOLEAN DEFAULT FALSE ,
    delay INTEGER DEFAULT 0 ,
    time_slot JSONB ,
    filters JSONB ,
    strategy_set JSONB ,
    uid INTEGER ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 告警等级表
CREATE TABLE IF NOT EXISTS alert_levels (
    id SERIAL PRIMARY KEY ,
    level_name VARCHAR(50) NOT NULL UNIQUE ,
    color VARCHAR(50) NOT NULL ,
    is_default BOOLEAN DEFAULT TRUE ,
    level_desc TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- 告警聚合表
CREATE TABLE IF NOT EXISTS alert_aggregations (
    id SERIAL PRIMARY KEY ,
    aggregation_name VARCHAR(100) ,
    aggregation_desc TEXT ,
    level_dimension BOOLEAN DEFAULT FALSE ,
    tags_dimension JSONB ,
    title_dimension BOOLEAN DEFAULT FALSE ,
    windows INTEGER DEFAULT 0 ,
    storm INTEGER DEFAULT 0 ,
    uid INTEGER ,
    status VARCHAR(20) DEFAULT 'enabled' ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- ============================================================================
-- 证书管理表
-- ============================================================================

-- 域名证书表（监控域名的SSL证书过期情况）
CREATE TABLE IF NOT EXISTS domain_certificates (
    id SERIAL PRIMARY KEY ,
    domain VARCHAR(255) NOT NULL ,
    port INTEGER DEFAULT 443 ,
    ssl_certificate TEXT ,
    ssl_certificate_key TEXT ,
    start_time TIMESTAMP ,
    expire_time TIMESTAMP ,
    expire_days INTEGER DEFAULT 0 ,
    is_monitor BOOLEAN DEFAULT TRUE ,
    auto_update BOOLEAN DEFAULT TRUE ,
    connect_status BOOLEAN ,
    alert_days INTEGER DEFAULT 30 ,
    alert_template_id INTEGER ,
    alert_channel_ids TEXT ,
    last_alert_time TIMESTAMP ,
    comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    UNIQUE (domain, port)
)
;

CREATE INDEX IF NOT EXISTS idx_domain_certificates_domain ON domain_certificates(domain);
CREATE INDEX IF NOT EXISTS idx_domain_certificates_expire_time ON domain_certificates(expire_time);
CREATE INDEX IF NOT EXISTS idx_domain_certificates_is_monitor ON domain_certificates(is_monitor);
CREATE INDEX IF NOT EXISTS idx_domain_certificates_alert_template_id ON domain_certificates(alert_template_id);
CREATE INDEX IF NOT EXISTS idx_domain_certificates_last_alert_time ON domain_certificates(last_alert_time);

-- SSL证书表（手动管理的SSL证书文件）
CREATE TABLE IF NOT EXISTS ssl_certificates (
    id SERIAL PRIMARY KEY ,
    domain VARCHAR(255) NOT NULL ,
    ssl_certificate TEXT ,
    ssl_certificate_key TEXT ,
    start_time TIMESTAMP ,
    expire_time TIMESTAMP ,
    comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

CREATE INDEX IF NOT EXISTS idx_ssl_certificates_domain ON ssl_certificates(domain);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_expire_time ON ssl_certificates(expire_time);

-- 托管证书表（托管在系统中的证书文件）
CREATE TABLE IF NOT EXISTS hosted_certificates (
    id SERIAL PRIMARY KEY ,
    domain VARCHAR(255) NOT NULL ,
    ssl_certificate TEXT ,
    ssl_certificate_key TEXT ,
    start_time TIMESTAMP ,
    expire_time TIMESTAMP ,
    comment TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

CREATE INDEX IF NOT EXISTS idx_hosted_certificates_domain ON hosted_certificates(domain);
CREATE INDEX IF NOT EXISTS idx_hosted_certificates_expire_time ON hosted_certificates(expire_time);

-- 告警静默表
CREATE TABLE IF NOT EXISTS alert_silences (
    id SERIAL PRIMARY KEY ,
    department_id VARCHAR(36) ,
    silence_name VARCHAR(100) ,
    silence_desc TEXT ,
    silence_type VARCHAR(20) ,
    silence_time JSONB ,
    filters JSONB ,
    uid INTEGER ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- 告警抑制表
CREATE TABLE IF NOT EXISTS alert_restrains (
    id SERIAL PRIMARY KEY ,
    restrain_type VARCHAR(20) ,
    fields JSONB ,
    cumulative_time INTEGER ,
    uid INTEGER ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- 告警模板表
CREATE TABLE IF NOT EXISTS alert_templates (
    id SERIAL PRIMARY KEY ,
    template_name VARCHAR(100) ,
    template_desc TEXT ,
    channels JSONB ,
    members JSONB ,
    alert_groups JSONB ,
    enable BOOLEAN DEFAULT TRUE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- 渠道模板表
CREATE TABLE IF NOT EXISTS channel_templates (
    id SERIAL PRIMARY KEY ,
    template_id INTEGER ,
    channel_id INTEGER ,
    content TEXT ,
    finished BOOLEAN DEFAULT FALSE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 告警渠道表
CREATE TABLE IF NOT EXISTS alert_channels (
    id SERIAL PRIMARY KEY ,
    channel_name VARCHAR(100) NOT NULL ,
    channel_type VARCHAR(50) NOT NULL ,
    channel_sign VARCHAR(500) NOT NULL ,
    channel_group VARCHAR(100) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
;

-- 策略日志表
CREATE TABLE IF NOT EXISTS strategy_logs (
    id SERIAL PRIMARY KEY ,
    alert_id BIGINT ,
    uid INTEGER ,
    strategy_content JSONB ,
    strategy_id INTEGER ,
    channels JSONB ,
    is_notify BOOLEAN DEFAULT FALSE ,
    err_message TEXT ,
    notify_type SMALLINT DEFAULT 1 ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- ============================================================================
-- On-Call Schedule Tables (值班排班表)
-- ============================================================================

-- 值班排班表
CREATE TABLE IF NOT EXISTS on_call_schedules (
    id SERIAL PRIMARY KEY ,
    schedule_name VARCHAR(100) NOT NULL ,
    description TEXT ,
    department_id VARCHAR(36) ,
    enabled BOOLEAN DEFAULT TRUE ,
    uid INTEGER ,
    notification_webhook VARCHAR(500) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- 值班班次表
CREATE TABLE IF NOT EXISTS on_call_shifts (
    id SERIAL PRIMARY KEY ,
    schedule_id INTEGER NOT NULL ,
    user_id VARCHAR(36) NOT NULL ,
    start_time TIMESTAMP NOT NULL ,
    end_time TIMESTAMP NOT NULL ,
    shift_type VARCHAR(20) DEFAULT 'manual' ,
    repeat_rule VARCHAR(100) ,
    status VARCHAR(20) DEFAULT 'active' ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (schedule_id) REFERENCES on_call_schedules(id) ON DELETE CASCADE
)
;

-- 告警分配表
CREATE TABLE IF NOT EXISTS on_call_assignments (
    id SERIAL PRIMARY KEY ,
    alert_id BIGINT NOT NULL ,
    user_id VARCHAR(36) NOT NULL ,
    shift_id INTEGER ,
    assigned_at TIMESTAMP NOT NULL ,
    assigned_by VARCHAR(36) ,
    auto_assigned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (alert_id) REFERENCES alert_events(id) ON DELETE CASCADE
);

-- 插入默认告警等级
INSERT INTO alert_levels (level_name, color, is_default, level_desc) VALUES
('P0', '#f53f3f', TRUE, '紧急/灾难性 - 系统完全不可用或核心功能严重故障，需要立即响应（通常要求 5-15 分钟内响应）'),
('P1', '#f77234', TRUE, '严重 - 核心功能受影响但系统仍可用，需要快速响应（通常要求 15-30 分钟内响应）'),
('P2', '#ff7d00', TRUE, '高优先级 - 重要功能受影响，需要及时响应（通常要求 1-2 小时内响应）'),
('P3', '#f7ba1e', TRUE, '中等优先级 - 非核心功能受影响，需要关注（通常要求 4-8 小时内响应）'),
('P4', '#9fdb1d', TRUE, '低优先级 - 轻微问题或优化建议，可以稍后处理（通常要求 1-2 个工作日内响应）')
ON CONFLICT (level_name) DO UPDATE SET
    color = EXCLUDED.color,
    is_default = EXCLUDED.is_default,
    level_desc = EXCLUDED.level_desc,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- Deployment Management Tables (发布管理表)
-- ============================================================================

-- Deployments table (部署记录表)
CREATE TABLE IF NOT EXISTS deployments (
    id VARCHAR(36) PRIMARY KEY ,
    project_name VARCHAR(255) NOT NULL ,
    project_id VARCHAR(100) ,
    env_id VARCHAR(100) ,
    env_name VARCHAR(100) ,
    cluster_id VARCHAR(100) ,
    cluster_name VARCHAR(100) ,
    namespace VARCHAR(100) ,
    deploy_type VARCHAR(50) NOT NULL ,
    deploy_config JSONB ,
    version VARCHAR(100) ,
    artifact_url TEXT ,
    jenkins_job VARCHAR(255) ,
    jenkins_build_number INTEGER ,
    k8s_yaml TEXT ,
    k8s_kind VARCHAR(50) ,
    verify_enabled BOOLEAN DEFAULT FALSE ,
    verify_timeout INTEGER DEFAULT 300 ,
    status VARCHAR(20) DEFAULT 'pending' ,
    log_path TEXT ,
    build_log TEXT ,
    duration INTEGER ,
    description TEXT ,
    created_by VARCHAR(36) ,
    created_by_name VARCHAR(100) ,
    started_at TIMESTAMP ,
    completed_at TIMESTAMP ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Jenkins Servers table (Jenkins服务器配置表)
CREATE TABLE IF NOT EXISTS jenkins_servers (
    id SERIAL PRIMARY KEY ,
    alias VARCHAR(255) NOT NULL ,
    url VARCHAR(500) NOT NULL ,
    username VARCHAR(100) NOT NULL ,
    password VARCHAR(500) NOT NULL ,
    description TEXT ,
    enabled BOOLEAN DEFAULT TRUE ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

)
;

-- Application Deploy Bindings table (应用-发布绑定表)
CREATE TABLE IF NOT EXISTS application_deploy_bindings (
    id VARCHAR(36) PRIMARY KEY ,
    application_id VARCHAR(36) NOT NULL ,
    deploy_type VARCHAR(20) NOT NULL ,
    deploy_config_id VARCHAR(100) NOT NULL ,
    deploy_config_name VARCHAR(255) ,
    environment VARCHAR(50) ,
    jenkins_job VARCHAR(255) ,
    argocd_application VARCHAR(255) ,
    deploy_strategy VARCHAR(32) ,
    strategy_options TEXT ,
    enabled BOOLEAN DEFAULT TRUE ,
    description TEXT ,
    created_by VARCHAR(36) ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_app_deploy_env UNIQUE (application_id, deploy_type, deploy_config_id, environment)
)
;

-- Release Runs table (发布代码运行记录表，Git Webhook / 手动触发)
CREATE TABLE IF NOT EXISTS release_runs (
    id VARCHAR(36) PRIMARY KEY ,
    application_id VARCHAR(36) ,
    repo_url VARCHAR(512) NOT NULL ,
    branch VARCHAR(255) NOT NULL ,
    commit_sha VARCHAR(64) NOT NULL ,
    commit_message TEXT ,
    ref VARCHAR(512) ,
    source VARCHAR(20) DEFAULT 'webhook' ,
    status VARCHAR(20) DEFAULT 'pending' ,
    triggered_by VARCHAR(100) ,
    created_by VARCHAR(36) ,
    rollback_from_run_id VARCHAR(36) ,
    deploy_strategy VARCHAR(32) ,
    deployed_environment VARCHAR(32) ,
    started_at TIMESTAMP ,
    completed_at TIMESTAMP ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_url ON release_runs(repo_url);
CREATE INDEX IF NOT EXISTS idx_release_runs_branch ON release_runs(branch);
CREATE INDEX IF NOT EXISTS idx_release_runs_status ON release_runs(status);
CREATE INDEX IF NOT EXISTS idx_release_runs_created_at ON release_runs(created_at);
CREATE INDEX IF NOT EXISTS idx_release_runs_rollback_from ON release_runs(rollback_from_run_id);

-- ============================================================================
-- Initialize Casbin Model Configuration (Casbin模型配置初始化)
-- ============================================================================

-- 初始化Casbin模型配置（如果不存在则插入）
INSERT INTO casbin_models (section, "key", value, sort, created_at, updated_at) VALUES
-- request_definition: 请求定义
('request_definition', 'r', 'sub, obj, act', 1, NOW(), NOW()),

-- policy_definition: 策略定义
('policy_definition', 'p', 'sub, obj, act', 1, NOW(), NOW()),

-- role_definition: 角色定义
('role_definition', 'g', '_, _', 1, NOW(), NOW()),

-- policy_effect: 策略效果
('policy_effect', 'e', 'some(where (p.eft == allow))', 1, NOW(), NOW()),

-- matchers: 匹配器
('matchers', 'm', 'g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)', 1, NOW(), NOW())
ON CONFLICT (section, "key") DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

-- ============================================================================
-- Initialize Casbin Rules (Casbin权限规则初始化)
-- ============================================================================

-- 为role:admin添加所有API的访问权限（菜单权限通过menu_permissions表管理）
-- Casbin规则格式: p, role:admin, /api/path, method
-- 使用通配符匹配所有API路径和方法，允许role:admin访问所有API
-- 先删除可能存在的旧规则，然后插入新规则
DELETE FROM casbin_rule WHERE ptype = 'p' AND v0 = 'role:admin';

-- 允许role:admin访问所有API路径（使用通配符匹配路径和方法）
-- keyMatch2支持通配符，如 /api/* 可以匹配 /api/users、/api/hosts 等
-- regexMatch支持正则表达式，如 .* 可以匹配所有HTTP方法（GET、POST、PUT、DELETE等）
INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4, v5) VALUES
('p', 'role:admin', '/*', '.*', '', '', '');

-- 注意：菜单权限通过menu_permissions表管理，不需要在Casbin中添加菜单规则
-- API权限通过Casbin规则管理，上面的规则允许role:admin访问所有API

-- ============================================================================
-- Workflow / 工单（精简版，与 Venus 兼容）
-- ============================================================================
CREATE TABLE IF NOT EXISTS workflows (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(40) NOT NULL UNIQUE ,
    title VARCHAR(200) NOT NULL,
    workflow_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' ,
    comment TEXT,
    labels JSONB,
    applicant_id VARCHAR(50),
    applicant_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

) ;

CREATE TABLE IF NOT EXISTS workflow_steps (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(40) NOT NULL,
    step_id VARCHAR(64) NOT NULL,
    step_type VARCHAR(50),
    step_name VARCHAR(200),
    step_comment TEXT,
    step_status INTEGER DEFAULT 0 ,
    func_kwargs_json JSONB,
    who_has_permission JSONB,
    require_steps JSONB,
    step_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (job_id, step_id),
    CONSTRAINT fk_workflow_step_job FOREIGN KEY (job_id) REFERENCES workflows(job_id) ON DELETE CASCADE
) ;

CREATE TABLE IF NOT EXISTS workflow_comments (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(40) NOT NULL,
    step_id VARCHAR(64),
    user_id VARCHAR(50),
    user_name VARCHAR(100),
    action VARCHAR(30),
    comment TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ;

-- ============================================================================
-- Organization Management Tables
-- ============================================================================

-- Organizations table (组织架构表)
CREATE TABLE IF NOT EXISTS organizations (
    id VARCHAR(36) PRIMARY KEY ,
    unit_code VARCHAR(100) UNIQUE NOT NULL ,
    unit_name VARCHAR(255) NOT NULL ,
    unit_type VARCHAR(50) NOT NULL ,
    unit_owner VARCHAR(255) ,
    is_active BOOLEAN DEFAULT TRUE ,
    parent_id VARCHAR(36) ,
    sort_order INTEGER DEFAULT 0 ,
    description TEXT ,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES organizations(id) ON DELETE SET NULL
)
;

-- 添加users表的外键约束（需要在organizations表创建之后）
ALTER TABLE users ADD CONSTRAINT fk_users_organization_id 
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

-- Insert organization test data
-- BizGroup (顶级组织)
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order) VALUES
(gen_random_uuid(), 'tech-group', '技术组', 'BizGroup', '张三', TRUE, NULL, 1),
(gen_random_uuid(), 'business-group', '业务组', 'BizGroup', '李四', TRUE, NULL, 2),
(gen_random_uuid(), 'finance-group', '财务组', 'BizGroup', '王明', TRUE, NULL, 3),
(gen_random_uuid(), 'hr-group', '人力资源组', 'BizGroup', '刘芳', TRUE, NULL, 4),
(gen_random_uuid(), 'marketing-group', '市场组', 'BizGroup', '陈强', TRUE, NULL, 5);

-- LineOfBiz (业务线，属于对应的 BizGroup)
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'tech-rd', '技术研发', 'LineOfBiz', '王五', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-group' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'tech-ops', '技术运维', 'LineOfBiz', '赵磊', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-group' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'tech-qa', '质量保障', 'LineOfBiz', '孙丽', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-group' LIMIT 1), 3
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'business-ops', '业务运营', 'LineOfBiz', '赵六', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-group' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'business-sales', '销售业务', 'LineOfBiz', '周伟', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-group' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'finance-accounting', '财务会计', 'LineOfBiz', '吴敏', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'finance-group' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'finance-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'hr-recruitment', '招聘管理', 'LineOfBiz', '郑华', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'hr-group' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'hr-group');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'marketing-brand', '品牌营销', 'LineOfBiz', '冯军', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'marketing-group' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'marketing-group');

-- Department (部门，属于对应的 LineOfBiz)
-- 技术研发业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'frontend-dept', '前端部门', 'Department', '钱七', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd');

-- 确保 backend-dept 存在且数据正确
-- 先尝试插入（如果不存在）
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'backend-dept', '后端部门', 'Department', '孙八', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd')
  AND NOT EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'backend-dept');

-- 然后更新（如果已存在，确保数据正确）
-- 使用变量避免 MySQL 1093 错误
UPDATE organizations
SET unit_name = '后端部门',
    unit_type = 'Department',
    unit_owner = '孙八',
    is_active = TRUE,
    parent_id = (SELECT id FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1),
    sort_order = 2
WHERE unit_code = 'backend-dept'
  ;

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'mobile-dept', '移动端部门', 'Department', '李九', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1), 3
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'ai-dept', 'AI算法部门', 'Department', '张十', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1), 4
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd');

-- 技术运维业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'infra-dept', '基础设施部门', 'Department', '王十一', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-ops' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-ops');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'security-dept', '安全部门', 'Department', '赵十二', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-ops' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-ops');

-- 质量保障业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'qa-dept', '测试部门', 'Department', '孙十三', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'tech-qa' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-qa');

-- 业务运营业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'product-dept', '产品部门', 'Department', '周九', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-ops' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'operation-dept', '运营部门', 'Department', '吴十四', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-ops' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'customer-dept', '客户服务部门', 'Department', '郑十五', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-ops' LIMIT 1), 3
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops');

-- 销售业务业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'sales-dept', '销售部门', 'Department', '冯十六', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-sales' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-sales');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'channel-dept', '渠道部门', 'Department', '陈十七', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'business-sales' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-sales');

-- 财务会计业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'accounting-dept', '会计部门', 'Department', '刘十八', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'finance-accounting' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'finance-accounting');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'audit-dept', '审计部门', 'Department', '黄十九', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'finance-accounting' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'finance-accounting');

-- 招聘管理业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'recruitment-dept', '招聘部门', 'Department', '林二十', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'hr-recruitment' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'hr-recruitment');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'training-dept', '培训部门', 'Department', '徐二一', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'hr-recruitment' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'hr-recruitment');

-- 品牌营销业务线下的部门
INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'brand-dept', '品牌部门', 'Department', '朱二二', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'marketing-brand' LIMIT 1), 1
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'marketing-brand');

INSERT INTO organizations (id, unit_code, unit_name, unit_type, unit_owner, is_active, parent_id, sort_order)
SELECT gen_random_uuid(), 'pr-dept', '公关部门', 'Department', '马二三', TRUE,
    (SELECT id FROM organizations WHERE unit_code = 'marketing-brand' LIMIT 1), 2
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'marketing-brand');

-- 更新admin用户的部门关联（关联到backend-dept部门）
UPDATE users 
SET organization_id = (SELECT id FROM organizations WHERE unit_code = 'backend-dept' LIMIT 1)
WHERE username = 'admin' 
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'backend-dept');

-- ============================================================================
-- Application Management Tables (服务管理)
-- ============================================================================

-- Applications table (应用服务表)
CREATE TABLE IF NOT EXISTS applications (
    id VARCHAR(36) PRIMARY KEY ,
    org VARCHAR(100) ,
    line_of_biz VARCHAR(100) ,
    name VARCHAR(255) NOT NULL ,
    is_critical BOOLEAN DEFAULT FALSE ,
    srv_type VARCHAR(50) NOT NULL ,
    virtual_tech VARCHAR(50) ,
    status VARCHAR(50) NOT NULL DEFAULT 'Initializing' ,
    department VARCHAR(100) ,
    site VARCHAR(50) ,
    description TEXT ,
    online_at TIMESTAMP ,
    offline_at TIMESTAMP ,
    git_url VARCHAR(500) ,
    ops_owners JSONB ,
    test_owners JSONB ,
    dev_owners JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org) REFERENCES organizations(unit_code) ON DELETE SET NULL,
    FOREIGN KEY (line_of_biz) REFERENCES organizations(unit_code) ON DELETE SET NULL,
    FOREIGN KEY (department) REFERENCES organizations(unit_code) ON DELETE SET NULL
)
;

-- Insert application test data
-- 通过外键关联 organizations 表，使用 SELECT 查询获取 unit_code，避免硬编码
-- 注意：org 字段存储的是 BizGroup 的 unit_code，line_of_biz 字段存储的是 LineOfBiz 的 unit_code，department 字段存储的是 Department 的 unit_code
-- site 字段为扩展字段，可填写：大陆、香港、北美、欧洲等

-- 技术组（tech-group）下的应用
-- 技术研发业务线（tech-rd）下的应用
INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1),
    'xxl-job-admin', FALSE, 'MIDDLEWARE', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'frontend-dept' LIMIT 1),
    '大陆',
    '业务定时任务调度系统', '2024-06-19 13:34:48', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'frontend-dept');

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1),
    'web-frontend', FALSE, 'WEB', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'frontend-dept' LIMIT 1),
    '大陆',
    'Web前端应用', '2024-04-01 11:00:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'frontend-dept');

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'tech-rd' LIMIT 1),
    'nginx-proxy', FALSE, 'SERVER', 'ECS', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'frontend-dept' LIMIT 1),
    '大陆',
    'Nginx反向代理', '2024-01-05 09:00:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'tech-rd')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'frontend-dept');

-- 业务组（business-group）下的应用
-- 业务运营业务线（business-ops）下的应用
INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-ops' LIMIT 1),
    'crm-system', TRUE, 'WEB', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'product-dept' LIMIT 1),
    '大陆',
    'CRM客户管理系统（核心业务系统）', '2024-01-20 10:00:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'product-dept');

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-ops' LIMIT 1),
    'report-service', FALSE, 'WEB', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'product-dept' LIMIT 1),
    '香港',
    '报表服务系统', '2024-03-15 14:00:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'product-dept');

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-ops' LIMIT 1),
    'analytics-service', FALSE, 'DATAWARE', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'product-dept' LIMIT 1),
    '北美',
    '数据分析服务', '2024-04-10 11:00:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'product-dept');

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-ops' LIMIT 1),
    'notification-service', FALSE, 'MIDDLEWARE', 'K8S', 'Running',
    (SELECT unit_code FROM organizations WHERE unit_code = 'product-dept' LIMIT 1),
    '大陆',
    '消息通知服务', '2024-02-25 09:30:00', NULL
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'product-dept');

-- 一些已下线的应用

INSERT INTO applications (id, org, line_of_biz, name, is_critical, srv_type, virtual_tech, status, department, site, description, online_at, offline_at)
SELECT gen_random_uuid(),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-group' LIMIT 1),
    (SELECT unit_code FROM organizations WHERE unit_code = 'business-ops' LIMIT 1),
    'old-crm', FALSE, 'WEB', 'EC2', 'Stopped',
    (SELECT unit_code FROM organizations WHERE unit_code = 'product-dept' LIMIT 1),
    '香港',
    '旧版CRM系统（已迁移）', '2019-06-01 00:00:00', '2023-12-31 23:59:59'
WHERE EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-group')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'business-ops')
  AND EXISTS (SELECT 1 FROM organizations WHERE unit_code = 'product-dept');

-- 初始化中的应用

-- ============================================================================
-- End of Schema
-- ============================================================================

-- Show created tables
SELECT
    table_name,
    0,
    pg_size_pretty(pg_total_relation_size(quote_ident(table_schema)||'.'||quote_ident(table_name))) AS size,
    'Table'
FROM
    information_schema.tables
WHERE
    table_schema = 'public'
ORDER BY
    table_name;

-- ============================================================================
-- 最终检查：确保菜单和权限数据已正确初始化
-- ============================================================================
-- 注意：此部分会在脚本执行时再次检查，确保权限分配正确
-- 即使上面的权限分配已经执行过，这里也会重新执行以确保数据一致性

-- 确保 admin 角色拥有所有菜单权限
-- 先删除可能存在的旧权限，然后重新分配所有菜单权限
DELETE FROM menu_permissions WHERE role_id = 'role:admin';
INSERT INTO menu_permissions (role_id, menu_id, created_at)
SELECT 'role:admin', menus.id, NOW() FROM menus
ON CONFLICT (role_id, menu_id) DO NOTHING;
-- 确保模型配置菜单对 admin 可见
INSERT INTO menu_permissions (role_id, menu_id, created_at) VALUES
('role:admin', 'menu-ai-assistant-models', NOW())
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- 确保 user 角色拥有基础菜单权限
DELETE FROM menu_permissions WHERE role_id = 'role:user';
INSERT INTO menu_permissions (role_id, menu_id, created_at)
SELECT 'role:user', menus.id, NOW() FROM menus
WHERE menus.id IN (
    'menu-home',
    'menu-org-dashboard',
    'menu-app-dashboard',
    'menu-system-dashboard',
    'menu-k8s-dashboard',
    'menu-personal'
)
ON CONFLICT (role_id, menu_id) DO UPDATE SET created_at = NOW();

SELECT 'Database initialized successfully!' AS Status;