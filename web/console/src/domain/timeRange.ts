export interface TimeRange {
  startTime: string;
  endTime: string;
}

// recentTimeRange 将结束时间推进到下一分钟，避免按分钟展示的快捷范围漏掉当前分钟内的新请求
export function recentTimeRange(hours: number, now = new Date()): TimeRange {
  const end = roundUpToMinute(now);
  return {
    startTime: localDateTime(new Date(end.getTime() - hours * 60 * 60 * 1000)),
    endTime: localDateTime(end),
  };
}

export function roundUpToMinute(value: Date): Date {
  const result = new Date(value);
  if (result.getSeconds() !== 0 || result.getMilliseconds() !== 0) {
    result.setMinutes(result.getMinutes() + 1, 0, 0);
  }
  return result;
}

export function localDateTime(value: Date): string {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}
