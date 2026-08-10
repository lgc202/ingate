import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listGateways } from '@/api/gateways';
import { listUpstreams } from '@/api/upstreams';
import { listCertificates } from '@/api/certificates';
import { getPolicyWorkspace } from '@/api/policies';
import { getConfigurationSummary } from '@/api/configuration';
import { useResource } from '@/api/useResource';
import { Badge, PageFrame, Panel, StatCard, StatusDot } from '@/components/ui';
import {
  Sparkles,
  ShieldCheck,
  Zap,
  Layers3,
  Route as RouteIcon,
  Server,
  KeyRound,
  ArrowRight,
  Cpu,
  CheckCircle2,
  Lock,
  CreditCard,
  Gauge,
  Activity,
} from 'lucide-react';

export function OverviewPage() {
  const navigate = useNavigate();
  const gateways = useResource(listGateways);
  const upstreams = useResource(listUpstreams);
  const certificates = useResource(listCertificates);
  const policies = useResource(getPolicyWorkspace);
  const status = useResource(getConfigurationSummary);

  const gatewayList = gateways.data?.gateways ?? [];
  const upstreamList = upstreams.data?.upstreams ?? [];
  const certificateList = certificates.data?.certificates ?? [];
  const policyData = policies.data;
  const statusData = status.data;

  const modelUpstreams = upstreamList.filter((u) => u.type === 'model');
  const appUpstreams = upstreamList.filter((u) => u.type !== 'model');

  const totalPolicies = policyData?.policies.length ?? 0;

  return (
    <PageFrame
      title="Ingate 控制台总览 (Overview Hub)"
      subtitle="声明式 Envoy 控制面状态、AI 代理拓扑与业务场景建置入口"
    >
      <div className="space-y-6 mt-2">
        {/* Top Operational Metrics Row */}
        <div className="grid grid-cols-4 gap-4">
          <StatCard
            title="配置发布状态"
            value={statusData
              ? statusData.error > 0
                ? '需处理'
                : statusData.pending > 0
                  ? '同步中'
                  : '正常'
              : '检查中'}
            subvalue={statusData
              ? `${statusData.ready}/${statusData.total} 项配置已生效`
              : '正在获取配置状态'}
            icon={Activity}
            trend={statusData?.error ? `${statusData.error} 项异常` : undefined}
          />
          <StatCard
            title="已接入 AI 大模型"
            value={modelUpstreams.length}
            subvalue={`兼容 DeepSeek / OpenAI 等`}
            icon={Sparkles}
          />
          <StatCard
            title="活跃网关监听器"
            value={gatewayList.length}
            subvalue={`端口 80 / 443 集中分发`}
            icon={Layers3}
          />
          <StatCard
            title="治理策略"
            value={totalPolicies}
            subvalue="限流 / IP 访问限制 / Token 配额"
            icon={ShieldCheck}
          />
        </div>

        {/* Scenario Presets Section (场景化快速建置) */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-900 flex items-center gap-2">
              <Zap className="w-4 h-4 text-amber-500" />
              业务场景快速建置入口 (Scenario Presets)
            </h3>
            <span className="text-xs text-slate-500">点击快捷卡片直接进入目标场景配置</span>
          </div>

          <div className="grid grid-cols-4 gap-4">
            {/* Scenario 1: AI Model Proxy */}
            <button
              type="button"
              onClick={() => navigate('/services')}
              className="p-4 bg-gradient-to-br from-purple-900 to-indigo-950 text-white rounded-xl text-left shadow-sm hover:shadow-md hover:scale-[1.01] transition-all cursor-pointer group"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="p-2 bg-purple-500/20 rounded-lg text-purple-300">
                  <Sparkles className="w-5 h-5" />
                </div>
                <ArrowRight className="w-4 h-4 text-purple-400 group-hover:translate-x-1 transition-transform" />
              </div>
              <h4 className="text-xs font-bold text-white">接入 DeepSeek / OpenAI 代理</h4>
              <p className="text-[11px] text-purple-200/80 mt-1 line-clamp-2">
                录入 API Key，自动暴露 OpenAI 规范统一聊天补全接口
              </p>
            </button>

            {/* Scenario 2: API Rate Limiting */}
            <button
              type="button"
              onClick={() => navigate('/policies')}
              className="p-4 bg-gradient-to-br from-slate-900 to-blue-950 text-white rounded-xl text-left shadow-sm hover:shadow-md hover:scale-[1.01] transition-all cursor-pointer group"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="p-2 bg-blue-500/20 rounded-lg text-blue-300">
                  <Gauge className="w-5 h-5" />
                </div>
                <ArrowRight className="w-4 h-4 text-blue-400 group-hover:translate-x-1 transition-transform" />
              </div>
              <h4 className="text-xs font-bold text-white">API 接口秒级限流防刷</h4>
              <p className="text-[11px] text-blue-200/80 mt-1 line-clamp-2">
                按 IP、Header 或路径配置共享计数限流与平滑爆发表
              </p>
            </button>

            <button
              type="button"
              onClick={() => navigate('/policies')}
              className="p-4 bg-gradient-to-br from-slate-900 to-emerald-950 text-white rounded-xl text-left shadow-sm hover:shadow-md hover:scale-[1.01] transition-all cursor-pointer group"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="p-2 bg-emerald-500/20 rounded-lg text-emerald-300">
                  <Lock className="w-5 h-5" />
                </div>
                <ArrowRight className="w-4 h-4 text-emerald-400 group-hover:translate-x-1 transition-transform" />
              </div>
              <h4 className="text-xs font-bold text-white">内部服务 IP 安全白名单</h4>
              <p className="text-[11px] text-emerald-200/80 mt-1 line-clamp-2">
                拦截未知网段请求，仅向指定 CIDR 白名单暴露接口
              </p>
            </button>

            {/* Scenario 4: Token Quota Budget */}
            <button
              type="button"
              onClick={() => navigate('/policies')}
              className="p-4 bg-gradient-to-br from-purple-950 to-slate-900 text-white rounded-xl text-left shadow-sm hover:shadow-md hover:scale-[1.01] transition-all cursor-pointer group"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="p-2 bg-purple-500/20 rounded-lg text-purple-300">
                  <CreditCard className="w-5 h-5" />
                </div>
                <ArrowRight className="w-4 h-4 text-purple-400 group-hover:translate-x-1 transition-transform" />
              </div>
              <h4 className="text-xs font-bold text-white">AI 开发者 Token 每日用量额度</h4>
              <p className="text-[11px] text-purple-200/80 mt-1 line-clamp-2">
                自动归一化输入/输出 Token Usage 并限制预算上限
              </p>
            </button>
          </div>
        </section>

        {/* Global Architecture Topology Diagram */}
        <Panel title="Ingate 声明式 Envoy 拓扑全景流图 (Architecture Topology Flow)">
          <div className="p-4 bg-slate-900 text-slate-100 rounded-xl space-y-4">
            <div className="grid grid-cols-5 gap-3 items-center text-xs font-mono text-center">
              <div className="p-3 bg-slate-800 border border-slate-700 rounded-lg">
                <div className="text-[10px] text-slate-400 font-sans uppercase">1. 外部客户端</div>
                <div className="text-blue-300 font-semibold mt-1">HTTP / HTTPS 流量</div>
                <div className="text-[10px] text-slate-400 mt-0.5">80 / 443 端口接入</div>
              </div>

              <div className="text-slate-500 flex justify-center">➔</div>

              <div className="p-3 bg-blue-950/60 border border-blue-800 rounded-lg">
                <div className="text-[10px] text-blue-300 font-sans uppercase">2. Envoy 数据面 (xDS)</div>
                <div className="text-white font-semibold mt-1">Ingate Controller 编译</div>
                <div className="text-[10px] text-blue-200/70 mt-0.5">唯一高性能代理数据面</div>
              </div>

              <div className="text-slate-500 flex justify-center">➔</div>

              <div className="p-3 bg-purple-950/60 border border-purple-800 rounded-lg">
                <div className="text-[10px] text-purple-300 font-sans uppercase">3. 上游 Cluster</div>
                <div className="text-purple-200 font-semibold mt-1">AI 模型 / 应用微服务</div>
                <div className="text-[10px] text-purple-300/70 mt-0.5">OpenAI, DeepSeek, REST</div>
              </div>
            </div>
          </div>
        </Panel>

        {/* Config Perception & System Health Bottom Row */}
        <div className="grid grid-cols-2 gap-6">
          <Panel title="快捷操作 (Quick Actions)">
            <div className="space-y-2">
              <Link
                to="/services"
                className="flex items-center justify-between p-3 bg-slate-50 hover:bg-slate-100 rounded-lg transition-colors text-xs font-medium text-slate-800"
              >
                <div className="flex items-center gap-2">
                  <Server className="w-4 h-4 text-blue-600" />
                  <span>配置与管理上游服务 (Upstreams & Models)</span>
                </div>
                <ArrowRight className="w-4 h-4 text-slate-400" />
              </Link>
              <Link
                to="/routes"
                className="flex items-center justify-between p-3 bg-slate-50 hover:bg-slate-100 rounded-lg transition-colors text-xs font-medium text-slate-800"
              >
                <div className="flex items-center gap-2">
                  <RouteIcon className="w-4 h-4 text-purple-600" />
                  <span>管理路径与 AI 模型路由规则 (Routes)</span>
                </div>
                <ArrowRight className="w-4 h-4 text-slate-400" />
              </Link>
              <Link
                to="/policies"
                className="flex items-center justify-between p-3 bg-slate-50 hover:bg-slate-100 rounded-lg transition-colors text-xs font-medium text-slate-800"
              >
                <div className="flex items-center gap-2">
                  <ShieldCheck className="w-4 h-4 text-emerald-600" />
                  <span>设置限流、IP 访问限制与 Token 配额策略</span>
                </div>
                <ArrowRight className="w-4 h-4 text-slate-400" />
              </Link>
            </div>
          </Panel>

          <Panel title="系统环境与依赖组件健康度">
            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between p-2.5 bg-slate-50 rounded-lg">
                <span className="font-medium text-slate-700">数据平面 (Data Plane)</span>
                <span className="font-mono text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
                  Envoy Ready
                </span>
              </div>
              <div className="flex items-center justify-between p-2.5 bg-slate-50 rounded-lg">
                <span className="font-medium text-slate-700">持久化存储 (Metadata)</span>
                <span className="font-mono text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
                  etcd Persistent
                </span>
              </div>
              <div className="flex items-center justify-between p-2.5 bg-slate-50 rounded-lg">
                <span className="font-medium text-slate-700">策略共享状态 (Quota / RateLimit)</span>
                <span className="font-mono text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
                  Redis Cluster Connected
                </span>
              </div>
            </div>
          </Panel>
        </div>
      </div>
    </PageFrame>
  );
}
