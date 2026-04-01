/**
 * Chinese (Simplified) translations for IronGolem OS.
 */

import type { en } from "./en";

type TranslationKeys = typeof en;

export const zh: TranslationKeys = {
  // ---------------------------------------------------------------------------
  // Primary navigation
  // ---------------------------------------------------------------------------
  "nav.home": "首页",
  "nav.inbox": "收件箱",
  "nav.recipes": "配方",
  "nav.squads": "小队",
  "nav.health": "健康中心",
  "nav.settings": "设置",
  "nav.admin": "管理控制台",
  "nav.plugins": "插件",
  "nav.research": "研究",
  "nav.history": "历史记录",
  "nav.connectors": "连接器",
  "nav.fleet": "集群",

  // ---------------------------------------------------------------------------
  // Heartbeat status labels
  // ---------------------------------------------------------------------------
  "heartbeat.healthy": "健康",
  "heartbeat.quietly_recovering": "静默恢复中",
  "heartbeat.needs_attention": "需要关注",
  "heartbeat.paused": "已暂停",
  "heartbeat.quarantined": "已隔离",
  "heartbeat.unknown": "未知",
  "heartbeat.last_seen": "最后在线 {{time}}",
  "heartbeat.never_reported": "从未报告",

  // ---------------------------------------------------------------------------
  // Risk level labels
  // ---------------------------------------------------------------------------
  "risk.none": "无风险",
  "risk.low": "低风险",
  "risk.medium": "中等风险",
  "risk.high": "高风险",
  "risk.critical": "严重风险",
  "risk.description": "风险等级：{{level}}",

  // ---------------------------------------------------------------------------
  // Safety card section headers
  // ---------------------------------------------------------------------------
  "safety.what_it_does": "功能说明",
  "safety.what_it_needs": "所需条件",
  "safety.what_could_go_wrong": "可能出现的问题",
  "safety.how_to_undo": "如何撤销",
  "safety.permissions_required": "所需权限",
  "safety.data_accessed": "访问的数据",
  "safety.estimated_impact": "预估影响",
  "safety.rollback_available": "可回滚",

  // ---------------------------------------------------------------------------
  // Approval action labels
  // ---------------------------------------------------------------------------
  "approval.approve": "批准",
  "approval.reject": "拒绝",
  "approval.approve_once": "批准一次",
  "approval.approve_always": "始终批准",
  "approval.review_details": "查看详情",
  "approval.pending": "待审批",
  "approval.approved": "已批准",
  "approval.rejected": "已拒绝",
  "approval.expired": "已过期",
  "approval.request_from": "来自 {{agent}} 的请求",
  "approval.action_description": "操作：{{action}}",

  // ---------------------------------------------------------------------------
  // Recipe labels
  // ---------------------------------------------------------------------------
  "recipe.activate": "激活配方",
  "recipe.deactivate": "停用配方",
  "recipe.edit": "编辑配方",
  "recipe.duplicate": "复制配方",
  "recipe.delete": "删除配方",
  "recipe.active": "活跃",
  "recipe.inactive": "未激活",
  "recipe.draft": "草稿",
  "recipe.running": "运行中",
  "recipe.paused": "已暂停",
  "recipe.created_by": "由 {{author}} 创建",
  "recipe.last_run": "上次运行 {{time}}",

  // ---------------------------------------------------------------------------
  // Squad labels
  // ---------------------------------------------------------------------------
  "squad.inbox": "收件箱小队",
  "squad.research": "研究小队",
  "squad.ops": "运维小队",
  "squad.security": "安全小队",
  "squad.executive_assistant": "行政助理小队",
  "squad.agents_count": "{{count}} 个代理",
  "squad.active_tasks": "{{count}} 个活跃任务",

  // ---------------------------------------------------------------------------
  // Common UI phrases
  // ---------------------------------------------------------------------------
  "common.loading": "加载中...",
  "common.error": "发生错误",
  "common.retry": "重试",
  "common.cancel": "取消",
  "common.save": "保存",
  "common.delete": "删除",
  "common.confirm": "确认",
  "common.close": "关闭",
  "common.back": "返回",
  "common.next": "下一步",
  "common.search": "搜索",
  "common.filter": "筛选",
  "common.sort": "排序",
  "common.refresh": "刷新",
  "common.no_results": "未找到结果",
  "common.created_at": "创建于 {{time}}",
  "common.updated_at": "更新于 {{time}}",
  "common.version": "版本 {{version}}",
  "common.enabled": "已启用",
  "common.disabled": "已禁用",
  "common.on": "开",
  "common.off": "关",
  "common.yes": "是",
  "common.no": "否",
  "common.unknown": "未知",
  "common.all": "全部",
  "common.none": "无",
  "common.more": "更多",
  "common.less": "更少",
  "common.show_all": "显示全部",
  "common.collapse": "收起",
  "common.expand": "展开",

  // ---------------------------------------------------------------------------
  // Settings
  // ---------------------------------------------------------------------------
  "settings.general": "通用",
  "settings.appearance": "外观",
  "settings.language": "语言",
  "settings.notifications": "通知",
  "settings.security": "安全",
  "settings.integrations": "集成",
  "settings.workspace": "工作区",
  "settings.deployment_mode": "部署模式",
  "settings.solo_mode": "单人模式",
  "settings.household_mode": "家庭模式",
  "settings.team_mode": "团队模式",

  // ---------------------------------------------------------------------------
  // Plugins
  // ---------------------------------------------------------------------------
  "plugin.install": "安装插件",
  "plugin.uninstall": "卸载插件",
  "plugin.enable": "启用插件",
  "plugin.disable": "禁用插件",
  "plugin.installed": "已安装",
  "plugin.available": "可用",
  "plugin.update_available": "有更新",
  "plugin.permissions": "插件权限",
  "plugin.by_author": "作者：{{author}}",
} as const;
