type DecimalFormatOptions = {
  fallback?: string;
  grouping?: boolean;
  maximumFractionDigits?: number;
};

const incrementDigits = (value: string) => {
  const digits = value.split('');
  for (let index = digits.length - 1; index >= 0; index -= 1) {
    const next = Number(digits[index]) + 1;
    if (next < 10) {
      digits[index] = String(next);
      return digits.join('');
    }
    digits[index] = '0';
  }
  return `1${digits.join('')}`;
};

// Formats fixed-point values as strings so exchange precision is not lost to JavaScript Number.
export const formatDecimal = (
  value: unknown,
  { fallback = '-', grouping = false, maximumFractionDigits }: DecimalFormatOptions = {},
) => {
  if (value === null || value === undefined) return fallback;

  const text = String(value).trim();
  if (!text) return fallback;

  const match = text.match(/^([+-]?)(\d+)(?:\.(\d*))?$/);
  if (!match) return text;

  const [, rawSign, rawInteger, rawFraction = ''] = match;
  let integer = rawInteger.replace(/^0+(?=\d)/, '') || '0';
  let fraction = rawFraction;
  if (
    maximumFractionDigits !== undefined &&
    maximumFractionDigits >= 0 &&
    fraction.length > maximumFractionDigits
  ) {
    const kept = fraction.slice(0, maximumFractionDigits);
    const shouldRound = Number(fraction[maximumFractionDigits]) >= 5;
    if (maximumFractionDigits === 0) {
      integer = shouldRound ? incrementDigits(integer) : integer;
      fraction = '';
    } else {
      const digits = (shouldRound ? incrementDigits(`${integer}${kept}`) : `${integer}${kept}`).padStart(
        maximumFractionDigits + 1,
        '0',
      );
      integer = digits.slice(0, -maximumFractionDigits);
      fraction = digits.slice(-maximumFractionDigits);
    }
  }
  fraction = fraction.replace(/0+$/, '');
  const isZero = integer === '0' && fraction === '';
  const sign = isZero ? '' : rawSign;
  const displayedInteger = grouping
    ? integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
    : integer;

  return `${sign}${displayedInteger}${fraction ? `.${fraction}` : ''}`;
};

export const renderDecimal = (value: unknown) => formatDecimal(value);
