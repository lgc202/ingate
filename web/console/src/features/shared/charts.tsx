import type { ReactNode } from 'react';

export function linePath(values: number[]) {
  const max = Math.max(...values);
  const min = Math.min(...values);
  const points = values
    .map((value, index) => {
      const x = 20 + (index * 460) / (values.length - 1);
      const y = 150 - ((value - min) / (max - min || 1)) * 110;
      return `${x},${y}`;
    })
    .join(' ');

  return (
    <>
      <polyline fill="none" stroke="#2f6b4f" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" points={points} />
      <polyline fill="none" stroke="rgba(47,107,79,.18)" strokeWidth="10" strokeLinecap="round" strokeLinejoin="round" points={points} />
      <line x1="20" y1="156" x2="480" y2="156" stroke="rgba(221,215,205,.7)" strokeWidth="2" />
    </>
  );
}

export function sectionList(items: [string, string, string][]) {
  return items.map(([a, b, c], index) => (
    <div
      key={a}
      className="legend-row"
      style={{ padding: '7px 0', borderBottom: index === items.length - 1 ? 0 : '1px solid var(--line)' }}
    >
      <span>{a}</span>
      <span>{b}</span>
      <span className="badge">{c}</span>
    </div>
  ));
}
