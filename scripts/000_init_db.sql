-- 新闻总结服务 - PostgreSQL 数据库初始化脚本
-- 创建独立数据库 newsdb（与 gold_analysis 的 golddb 物理隔离）
-- 执行身份：postgres 超级用户
--   psql -U postgres -f 000_init_db.sql
--
-- 前置条件：
--   用户 hwan 已在 gold_analysis/scripts/000_init_db.sql 中创建。
--   如果是独立部署（没跑过 gold_analysis 的初始化），请先取消下面 CREATE USER 一行的注释。

-- 创建数据库用户（如已存在请保持注释）
-- CREATE USER hwan WITH PASSWORD '******';

-- 创建数据库
CREATE DATABASE agentlydb OWNER agently;

-- 连接到 agentlydb 数据库
\c agentlydb;

-- 授予用户 agently 对 agentlydb 数据库的所有权限
GRANT ALL PRIVILEGES ON DATABASE agentlydb TO agently;

-- 授予用户 agently 对 public 模式的所有权限
GRANT ALL PRIVILEGES ON SCHEMA public TO agently;
