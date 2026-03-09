import React from 'react';

type LogsVariant = 'light' | 'dark';

type ANSIStyleState = {
  bold: boolean;
  color?: string;
};

type ANSISegment = {
  text: string;
  style: React.CSSProperties;
};

const ANSI_PATTERN = new RegExp(`${String.fromCharCode(27)}\\[([0-9;]*)m`, 'g');

const ANSI_COLORS: Record<LogsVariant, Record<number, string>> = {
  light: {
    30: '#111827',
    31: '#b91c1c',
    32: '#166534',
    33: '#a16207',
    34: '#1d4ed8',
    35: '#7e22ce',
    36: '#0f766e',
    37: '#374151',
    90: '#4b5563',
    91: '#dc2626',
    92: '#16a34a',
    93: '#ca8a04',
    94: '#2563eb',
    95: '#9333ea',
    96: '#0891b2',
    97: '#111827'
  },
  dark: {
    30: '#d1d5db',
    31: '#f87171',
    32: '#4ade80',
    33: '#facc15',
    34: '#60a5fa',
    35: '#c084fc',
    36: '#22d3ee',
    37: '#f9fafb',
    90: '#9ca3af',
    91: '#fca5a5',
    92: '#86efac',
    93: '#fde047',
    94: '#93c5fd',
    95: '#d8b4fe',
    96: '#67e8f9',
    97: '#ffffff'
  }
};

const DEFAULT_STYLE_STATE: ANSIStyleState = {
  bold: false
};

const toReactStyle = (styleState: ANSIStyleState): React.CSSProperties => ({
  color: styleState.color,
  fontWeight: styleState.bold ? 700 : undefined
});

const applySGRCode = (
  code: number,
  styleState: ANSIStyleState,
  variant: LogsVariant,
  codes: number[],
  index: number
): number => {
  if (code === 0) {
    styleState.bold = false;
    delete styleState.color;
    return index;
  }

  if (code === 1) {
    styleState.bold = true;
    return index;
  }

  if (code === 22) {
    styleState.bold = false;
    return index;
  }

  if (code === 39) {
    delete styleState.color;
    return index;
  }

  if (code in ANSI_COLORS[variant]) {
    styleState.color = ANSI_COLORS[variant][code];
    return index;
  }

  if (code === 38) {
    const mode = codes[index + 1];

    if (mode === 5 && typeof codes[index + 2] === 'number') {
      styleState.color = normalizeIndexedColor(codes[index + 2]);
      return index + 2;
    }

    if (
      mode === 2 &&
      typeof codes[index + 2] === 'number' &&
      typeof codes[index + 3] === 'number' &&
      typeof codes[index + 4] === 'number'
    ) {
      styleState.color = `rgb(${codes[index + 2]}, ${codes[index + 3]}, ${codes[index + 4]})`;
      return index + 4;
    }
  }

  return index;
};

const normalizeIndexedColor = (index: number): string => {
  if (index < 16) {
    const palette = [
      '#000000',
      '#800000',
      '#008000',
      '#808000',
      '#000080',
      '#800080',
      '#008080',
      '#c0c0c0',
      '#808080',
      '#ff0000',
      '#00ff00',
      '#ffff00',
      '#0000ff',
      '#ff00ff',
      '#00ffff',
      '#ffffff'
    ];
    return palette[index];
  }

  if (index >= 16 && index <= 231) {
    const adjusted = index - 16;
    const r = Math.floor(adjusted / 36);
    const g = Math.floor((adjusted % 36) / 6);
    const b = adjusted % 6;
    const channel = (value: number) => (value === 0 ? 0 : value * 40 + 55);
    return `rgb(${channel(r)}, ${channel(g)}, ${channel(b)})`;
  }

  const gray = 8 + (index - 232) * 10;
  return `rgb(${gray}, ${gray}, ${gray})`;
};

export const parseAnsiLine = (
  line: string,
  variant: LogsVariant
): ANSISegment[] => {
  const content = line.replace(/\r/g, '');
  const segments: ANSISegment[] = [];
  const styleState: ANSIStyleState = { ...DEFAULT_STYLE_STATE };

  ANSI_PATTERN.lastIndex = 0;

  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = ANSI_PATTERN.exec(content)) !== null) {
    if (match.index > lastIndex) {
      segments.push({
        text: content.slice(lastIndex, match.index),
        style: toReactStyle(styleState)
      });
    }

    const rawCodes = match[1] === '' ? [0] : match[1].split(';').map(Number);
    for (let i = 0; i < rawCodes.length; i += 1) {
      i = applySGRCode(rawCodes[i], styleState, variant, rawCodes, i);
    }

    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < content.length || content.length === 0) {
    segments.push({
      text: content.slice(lastIndex),
      style: toReactStyle(styleState)
    });
  }

  return segments;
};

export const AnsiLogLine: React.FC<{
  line: string;
  variant: LogsVariant;
}> = ({ line, variant }) => {
  const segments = parseAnsiLine(line, variant);

  return (
    <pre className="m-0 whitespace-pre-wrap break-all font-mono text-sm leading-6">
      {segments.length === 0
        ? '\u00A0'
        : segments.map((segment, index) => (
            <span key={`${index}-${segment.text}`} style={segment.style}>
              {segment.text || '\u00A0'}
            </span>
          ))}
    </pre>
  );
};
