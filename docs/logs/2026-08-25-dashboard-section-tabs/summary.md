# Dashboard sections match Settings grouping

Date: 2026-08-25
Status: complete

## What changed

中控台 (`/dashboard`) now uses the same tab + card grouping as 设置:

- Tabs: 概览 / Token / 审计
- 概览: one 运行状态 card (会话、活跃运行、待处理 Review) and one 近期活动 card
- Token / 审计: one card each, with title and description

Removed the three separate KPI cards and the single long scroll of every block.

## Unchanged

Token fake data, AuditPanel contents, DemoBanner on the dashboard route.

## Scope

UI only.
