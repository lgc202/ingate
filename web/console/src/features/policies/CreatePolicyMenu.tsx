import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, Gauge, MessageSquare, Plus, ShieldCheck, TimerReset, WandSparkles } from 'lucide-react';
import { Button } from '@/components/ui';
import { standardPluginPackages } from '@/domain/plugin';
import type { GovernancePolicyKind } from '@/domain/policy';

interface CreatePolicyMenuProps {
  transformerAvailable: boolean;
  mockResponseAvailable: boolean;
  onSelect: (kind: GovernancePolicyKind) => void;
}

export function CreatePolicyMenu({
  transformerAvailable,
  mockResponseAvailable,
  onSelect,
}: CreatePolicyMenuProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;

    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutsideClick);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  const select = (kind: GovernancePolicyKind) => {
    setOpen(false);
    onSelect(kind);
  };

  return (
    <div ref={rootRef} className="policy-create">
      <Button aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((current) => !current)}>
        <Plus className="h-4 w-4" />创建策略<ChevronDown className={`h-4 w-4 policy-create-chevron${open ? ' is-open' : ''}`} />
      </Button>
      {open ? (
        <div className="policy-create-menu" role="menu" aria-label="选择策略类型">
          <div className="policy-create-menu-title">选择策略类型</div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">访问控制</div>
            <button type="button" role="menuitem" onClick={() => select('IPRestrictionPolicy')}>
              <span className="policy-create-icon"><ShieldCheck aria-hidden="true" /></span>
              <span><strong>IP 访问限制</strong><small>按来源地址限制网关或路由访问</small></span>
            </button>
          </div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">流量治理</div>
            <button type="button" role="menuitem" onClick={() => select('RateLimitPolicy')}>
              <span className="policy-create-icon"><TimerReset aria-hidden="true" /></span>
              <span><strong>请求限流</strong><small>按目标、客户端 IP 或请求头限制请求频率</small></span>
            </button>
          </div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">流量处理</div>
            <button type="button" role="menuitem" disabled={!transformerAvailable} onClick={() => select('HeaderTransformationPolicy')}>
              <span className="policy-create-icon"><WandSparkles aria-hidden="true" /></span>
              <span><strong>请求响应转换</strong><small>按路由修改请求与响应 Header</small></span>
            </button>
            {!transformerAvailable ? <Link className="policy-create-prerequisite" to={`/plugins?install=${standardPluginPackages.transformer}`} onClick={() => setOpen(false)}>请先安装请求响应转换插件</Link> : null}
            <button type="button" role="menuitem" disabled={!mockResponseAvailable} onClick={() => select('MockResponsePolicy')}>
              <span className="policy-create-icon"><MessageSquare aria-hidden="true" /></span>
              <span><strong>模拟响应</strong><small>不访问服务，直接返回固定 HTTP 响应</small></span>
            </button>
            {!mockResponseAvailable ? <Link className="policy-create-prerequisite" to={`/plugins?install=${standardPluginPackages.mockResponse}`} onClick={() => setOpen(false)}>请先安装模拟响应插件</Link> : null}
          </div>
          <div className="policy-create-group">
            <div className="policy-create-group-title">AI 治理</div>
            <button type="button" role="menuitem" onClick={() => select('TokenQuotaPolicy')}>
              <span className="policy-create-icon"><Gauge aria-hidden="true" /></span>
              <span><strong>Token 额度</strong><small>按调用方限制模型 Token 用量</small></span>
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
