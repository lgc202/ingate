import { useState } from 'react';
import { Clock3 } from 'lucide-react';
import type { AIUsageMetrics, AIUsageTrendPoint } from '@/domain/aiUsage';
import {
  formatUsageCount,
  formatUsageCountExact,
  formatUsagePercent,
  usageNumber,
} from '@/domain/aiUsage';

export type AIUsageTrendMetric = 'tokens' | 'calls';

interface TrendSeries {
  key: 'input' | 'output' | 'calls';
  label: string;
  values: number[];
}

export function AIUsageTrendChart({
  points,
  metric,
  startTime,
  endTime,
}: {
  points: AIUsageTrendPoint[];
  metric: AIUsageTrendMetric;
  startTime: string;
  endTime: string;
}) {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  if (points.length === 0) {
    return <div className="ai-usage-chart-empty"><Clock3 /><span>当前范围没有模型调用</span></div>;
  }

  const samples = chartSamples(points, startTime, endTime);
  const series: TrendSeries[] = metric === 'tokens'
    ? [
        { key: 'input', label: '输入 Token', values: samples.map((point) => usageNumber(point.metrics.inputTokens)) },
        { key: 'output', label: '输出 Token', values: samples.map((point) => usageNumber(point.metrics.outputTokens)) },
      ]
    : [{ key: 'calls', label: '模型调用', values: samples.map((point) => usageNumber(point.metrics.callCount)) }];
  const maximum = niceMaximum(Math.max(...series.flatMap((item) => item.values), 1));
  const width = 960;
  const height = 230;
  const top = 12;
  const bottom = 10;
  const plotHeight = height - top - bottom;
  const x = (index: number) => samples.length === 1 ? width / 2 : (index / (samples.length - 1)) * width;
  const y = (value: number) => top + plotHeight - (value / maximum) * plotHeight;
  const activeIndex = hoveredIndex !== null && hoveredIndex < samples.length ? hoveredIndex : null;
  const activeLeft = activeIndex === null ? 0 : samples.length === 1 ? 50 : (activeIndex / (samples.length - 1)) * 100;
  const labels = chartLabels(samples);

  return (
    <div className={`ai-usage-trend-chart is-${metric}`}>
      <div className="ai-usage-chart-plot">
        <div className="ai-usage-chart-scale"><span>{formatUsageCount(maximum)}</span><span>{formatUsageCount(maximum / 2)}</span><span>0</span></div>
        {metric === 'tokens' ? <div className="ai-usage-chart-legend">{series.map((item) => <span key={item.key}><i className={`is-${item.key}`} />{item.label}</span>)}</div> : null}
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={metric === 'tokens' ? '输入与输出 Token 趋势' : '模型调用趋势'}
          preserveAspectRatio="none"
          onPointerMove={(event) => {
            const bounds = event.currentTarget.getBoundingClientRect();
            const ratio = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
            setHoveredIndex(Math.round(ratio * (samples.length - 1)));
          }}
          onPointerLeave={() => setHoveredIndex(null)}
        >
          <defs>
            <linearGradient id="ai-usage-call-area" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#4057d5" stopOpacity="0.2" />
              <stop offset="100%" stopColor="#4057d5" stopOpacity="0.01" />
            </linearGradient>
          </defs>
          {[0, 0.5, 1].map((ratio) => <line key={ratio} x1="0" x2={width} y1={top + plotHeight * ratio} y2={top + plotHeight * ratio} className="ai-usage-grid-line" />)}
          {series.map((item) => {
            const path = linePath(item.values, x, y);
            const area = metric === 'calls' ? `${path} L ${width} ${top + plotHeight} L 0 ${top + plotHeight} Z` : '';
            return <g key={item.key}>{area ? <path d={area} fill="url(#ai-usage-call-area)" /> : null}<path d={path} className={`ai-usage-trend-line is-${item.key}`} /></g>;
          })}
          <rect x="0" y="0" width={width} height={height} className="ai-usage-chart-hit-area" />
          {activeIndex !== null ? <>
            <line x1={x(activeIndex)} x2={x(activeIndex)} y1={top} y2={top + plotHeight} className="ai-usage-hover-line" />
            {series.map((item) => <circle key={item.key} cx={x(activeIndex)} cy={y(item.values[activeIndex])} r="4.5" className={`ai-usage-trend-dot is-${item.key}`} />)}
          </> : null}
        </svg>
        {activeIndex !== null ? <AIUsageChartTooltip sample={samples[activeIndex]} metric={metric} left={activeLeft} /> : null}
      </div>
      <div className="ai-usage-chart-labels">{labels.map((label) => <span key={label}>{label}</span>)}</div>
    </div>
  );
}

function AIUsageChartTooltip({ sample, metric, left }: { sample: AIUsageTrendPoint; metric: AIUsageTrendMetric; left: number }) {
  const calls = usageNumber(sample.metrics.callCount);
  return (
    <div className={`ai-usage-chart-tooltip is-${metric}`} style={{ left: `${Math.min(91, Math.max(9, left))}%` }}>
      <time>{formatTrendTime(sample.startedAt)}</time>
      {metric === 'tokens' ? <dl>
        <div><dt><i className="is-input" />输入 Token</dt><dd>{formatUsageCountExact(sample.metrics.inputTokens)}</dd></div>
        <div><dt><i className="is-output" />输出 Token</dt><dd>{formatUsageCountExact(sample.metrics.outputTokens)}</dd></div>
        <div><dt>总 Token</dt><dd>{formatUsageCountExact(sample.metrics.totalTokens)}</dd></div>
      </dl> : <>
        <div><strong>{formatUsageCountExact(calls)}</strong><span>模型调用</span></div>
        <dl><div><dt>正常响应率</dt><dd>{formatUsagePercent(usageNumber(sample.metrics.normalResponseCount), calls)}</dd></div></dl>
      </>}
    </div>
  );
}

function chartSamples(points: AIUsageTrendPoint[], startTime: string, endTime: string): AIUsageTrendPoint[] {
  const start = new Date(startTime).getTime();
  const end = new Date(endTime).getTime();
  const interval = bucketMilliseconds(end - start);
  const firstBucket = Math.floor(start / interval) * interval;
  const lastBucket = Math.floor((end - 1) / interval) * interval;
  const pointByTime = new Map(points.map((point) => [new Date(point.startedAt).getTime(), point]));
  const samples: AIUsageTrendPoint[] = [];
  for (let startedAt = firstBucket; startedAt <= lastBucket; startedAt += interval) {
    samples.push(pointByTime.get(startedAt) ?? { startedAt: new Date(startedAt).toISOString(), metrics: emptyMetrics() });
  }
  return samples;
}

function emptyMetrics(): AIUsageMetrics {
  return {
    callCount: 0,
    normalResponseCount: 0,
    tokenReportedCallCount: 0,
    inputTokens: 0,
    outputTokens: 0,
    totalTokens: 0,
  };
}

function bucketMilliseconds(range: number): number {
  if (range <= 2 * 60 * 60 * 1000) return 60 * 1000;
  if (range <= 24 * 60 * 60 * 1000) return 5 * 60 * 1000;
  if (range <= 7 * 24 * 60 * 60 * 1000) return 60 * 60 * 1000;
  return 24 * 60 * 60 * 1000;
}

function linePath(values: number[], x: (index: number) => number, y: (value: number) => number): string {
  return values.map((value, index) => `${index === 0 ? 'M' : 'L'} ${x(index)} ${y(value)}`).join(' ');
}

function niceMaximum(value: number): number {
  const exponent = 10 ** Math.floor(Math.log10(value));
  const normalized = value / exponent;
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return step * exponent;
}

function chartLabels(points: AIUsageTrendPoint[]): string[] {
  const indexes = [...new Set([0, Math.floor((points.length - 1) / 2), points.length - 1])];
  return indexes.map((index) => formatTrendTime(points[index].startedAt));
}

function formatTrendTime(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value));
}
