import { useState } from 'react';
import './DateRangeSelector.css';

type PresetRange = '1h' | '6h' | '24h' | '7d' | 'custom';
type QuickRange = Exclude<PresetRange, 'custom'>;

interface DateRangeSelectorProps {
  onRangeChange: (startDate: string, endDate: string) => void;
  defaultRange?: PresetRange;
}

const DateRangeSelector = ({ onRangeChange, defaultRange = '24h' }: DateRangeSelectorProps) => {
  const [selectedRange, setSelectedRange] = useState<PresetRange>(defaultRange);
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');
  const [showCustom, setShowCustom] = useState(false);

  const handleQuickSelect = (range: QuickRange) => {
    setSelectedRange(range);
    setShowCustom(false);

    const now = new Date();
    let startDate = new Date();

    switch (range) {
      case '1h':
        startDate.setHours(now.getHours() - 1);
        break;
      case '6h':
        startDate.setHours(now.getHours() - 6);
        break;
      case '24h':
        startDate.setHours(now.getHours() - 24);
        break;
      case '7d':
        startDate.setDate(now.getDate() - 7);
        break;
      default:
        return;
    }

    onRangeChange(startDate.toISOString(), now.toISOString());
  };

  const handleCustomApply = () => {
    if (customStart && customEnd) {
      const start = new Date(customStart);
      const end = new Date(customEnd);

      if (start < end) {
        onRangeChange(start.toISOString(), end.toISOString());
        setSelectedRange('custom');
      } else {
        alert('Start date must be before end date');
      }
    }
  };

  const toggleCustom = () => {
    setShowCustom(!showCustom);
    if (!showCustom) {
      setSelectedRange('custom');
    }
  };

  return (
    <div className="date-range-selector">
      <div className="quick-select">
        <button
          className={`range-btn ${selectedRange === '1h' ? 'active' : ''}`}
          onClick={() => handleQuickSelect('1h')}
        >
          1 Hour
        </button>
        <button
          className={`range-btn ${selectedRange === '6h' ? 'active' : ''}`}
          onClick={() => handleQuickSelect('6h')}
        >
          6 Hours
        </button>
        <button
          className={`range-btn ${selectedRange === '24h' ? 'active' : ''}`}
          onClick={() => handleQuickSelect('24h')}
        >
          24 Hours
        </button>
        <button
          className={`range-btn ${selectedRange === '7d' ? 'active' : ''}`}
          onClick={() => handleQuickSelect('7d')}
        >
          7 Days
        </button>
        <button
          className={`range-btn ${selectedRange === 'custom' ? 'active' : ''}`}
          onClick={toggleCustom}
        >
          Custom
        </button>
      </div>

      {showCustom && (
        <div className="custom-range">
          <div className="custom-inputs">
            <div className="input-group">
              <label htmlFor="start-date">Start Date</label>
              <input
                id="start-date"
                type="datetime-local"
                value={customStart}
                onChange={(e) => setCustomStart(e.target.value)}
              />
            </div>
            <div className="input-group">
              <label htmlFor="end-date">End Date</label>
              <input
                id="end-date"
                type="datetime-local"
                value={customEnd}
                onChange={(e) => setCustomEnd(e.target.value)}
              />
            </div>
          </div>
          <button
            className="apply-btn"
            onClick={handleCustomApply}
            disabled={!customStart || !customEnd}
          >
            Apply Range
          </button>
        </div>
      )}
    </div>
  );
};

export default DateRangeSelector;
