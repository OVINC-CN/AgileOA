import { useEffect, useMemo, useState } from 'react';

import { useHomeCalendar } from '@/features/home/calendar';
import { calendarDate, calendarKey, calendarLabel } from '@/lib/calendar';
import { getConfig } from '@/lib/config';

const millisecondsPerSecond = 1000;
const surroundingDateOffsets = [-3, -2, -1, 0, 1, 2, 3];
const weekdayLabels = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];

function useCurrentTime() {
  const [now, setNow] = useState(() => new Date());

  // 对齐整秒刷新，页面重新可见时立即同步，避免后台计时节流造成时间滞后。
  useEffect(() => {
    let timeoutID: number;

    function synchronize() {
      window.clearTimeout(timeoutID);
      const current = new Date();
      setNow(current);

      if (document.visibilityState === 'visible') {
        timeoutID = window.setTimeout(
          synchronize,
          millisecondsPerSecond - current.getMilliseconds(),
        );
      }
    }

    timeoutID = window.setTimeout(
      synchronize,
      millisecondsPerSecond - new Date().getMilliseconds(),
    );
    document.addEventListener('visibilitychange', synchronize);
    window.addEventListener('pageshow', synchronize);

    // 离开首页后释放计时器及页面事件。
    return () => {
      window.clearTimeout(timeoutID);
      document.removeEventListener('visibilitychange', synchronize);
      window.removeEventListener('pageshow', synchronize);
    };
  }, []);

  return now;
}

export function OverviewPage() {
  const { displayTimeZone } = getConfig();
  const now = useCurrentTime();
  const formatter = useMemo(
    () =>
      new Intl.DateTimeFormat('zh-CN', {
        timeZone: displayTimeZone,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        weekday: 'long',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hourCycle: 'h23',
        timeZoneName: 'long',
      }),
    [displayTimeZone],
  );
  const {
    year = '',
    month = '',
    day = '',
    weekday = '',
    hour = '',
    minute = '',
    second = '',
    timeZoneName = '',
  } = Object.fromEntries(formatter.formatToParts(now).map((part) => [part.type, part.value]));
  const todayKey = `${year}-${month}-${day}`;
  const { holidays, countdown, illustration } = useHomeCalendar(todayKey);
  const dateLabel = `${calendarLabel(todayKey)} ${weekday}`;
  const timeLabel = `${hour}:${minute}:${second}`;

  // 按展示时区确定当天，再用纯日期运算生成前后日期，兼容跨月、跨年和夏令时。
  const dates = useMemo(() => {
    const today = calendarDate(todayKey);
    if (!today) return [];

    return surroundingDateOffsets.map((offset) => {
      const date = new Date(today);
      date.setUTCDate(today.getUTCDate() + offset);
      return {
        key: calendarKey(date),
        day: String(date.getUTCDate()).padStart(2, '0'),
        weekday: weekdayLabels[date.getUTCDay()],
        offset,
      };
    });
  }, [todayKey]);

  // 将当天置于清晰的视觉焦点，相邻日期仅作为模糊的时间背景。
  return (
    <section
      aria-labelledby="overview-date-heading"
      className="relative isolate flex min-w-0 flex-1 flex-col items-center justify-center overflow-hidden bg-background py-8 text-center"
    >
      {illustration ? (
        <div
          aria-hidden="true"
          className="overview-holiday-background pointer-events-none absolute inset-0 -z-10 opacity-75"
        >
          <img src={illustration} alt="" className="h-full w-full object-contain" />
        </div>
      ) : null}
      <div className="w-full max-w-5xl" role="group" aria-label="当前日期">
        <h1
          id="overview-date-heading"
          className="flex items-center justify-center gap-3 text-base font-normal tracking-widest text-muted-foreground"
        >
          <span>{year} 年</span>
          <span aria-hidden="true" className="size-1 rounded-full bg-primary/50" />
          <span>{Number(month)} 月</span>
        </h1>
        {holidays.length > 0 || countdown ? (
          <p className="mt-2 px-4 text-base leading-relaxed font-normal break-words text-muted-foreground">
            {holidays.length > 0 ? (
              <span className="text-accent-foreground">{holidays.join(' · ')}</span>
            ) : null}
            {holidays.length > 0 && countdown ? <span aria-hidden="true"> · </span> : null}
            {countdown ? (
              <span>
                距离{countdown.holidays.join(' · ')}还有{' '}
                <span className="font-medium text-accent-foreground tabular-nums">
                  {countdown.days}
                </span>{' '}
                天
              </span>
            ) : null}
          </p>
        ) : null}
        <div className="overview-calendar-strip">
          <div className="overview-calendar-track">
            {dates.map((date) =>
              date.offset === 0 ? (
                <time
                  key={date.key}
                  dateTime={todayKey}
                  aria-current="date"
                  aria-label={`${dateLabel}，今天`}
                  className="glass-panel overview-calendar-today"
                >
                  <span className="text-base tracking-widest text-muted-foreground">
                    {weekday}
                  </span>
                  <span className="overview-calendar-day">{day}</span>
                  <span className="rounded-full bg-primary/10 px-3 py-1 text-xs tracking-widest text-accent-foreground">
                    今天
                  </span>
                </time>
              ) : (
                <div
                  key={date.key}
                  aria-hidden="true"
                  data-distance={Math.abs(date.offset)}
                  className="overview-calendar-neighbor"
                >
                  <span className="text-xs tracking-widest">{date.weekday}</span>
                  <span className="overview-calendar-neighbor-day">{date.day}</span>
                </div>
              ),
            )}
          </div>
        </div>
      </div>

      <div className="mt-2 flex flex-col items-center">
        <time
          dateTime={now.toISOString()}
          role="timer"
          aria-live="off"
          aria-label={`当前时间 ${timeLabel}，${timeZoneName}`}
          className="flex items-baseline gap-1 text-[clamp(1.875rem,5vw,2.5rem)] leading-none font-light tracking-[-0.055em] tabular-nums"
        >
          <span>
            {hour}:{minute}
          </span>
          <span className="text-[0.55em] text-accent-foreground">:{second}</span>
        </time>
      </div>
    </section>
  );
}
