import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import './TimeSeriesChart.css';

interface DataPoint {
  timestamp: string;
  temperature?: number;
  humidity?: number;
  pollution?: number;
  noise?: number;
}

interface TimeSeriesChartProps {
  data: DataPoint[];
  title?: string;
  dataKeys?: string[];
  height?: number;
}

const TimeSeriesChart = ({ data, title = 'Sensor Readings Over Time', dataKeys, height = 300 }: TimeSeriesChartProps) => {
  // Format timestamp for display
  const formatXAxis = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  };

  // Format tooltip timestamp
  const formatTooltipLabel = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  // Define colors for different metrics
  const lineColors: { [key: string]: string } = {
    temperature: '#ef4444',
    humidity: '#3b82f6',
    pollution: '#f59e0b',
    noise: '#8b5cf6'
  };

  // Get active data keys from the data
  const activeKeys = dataKeys || Object.keys(data[0] || {}).filter(key => key !== 'timestamp');

  if (!data || data.length === 0) {
    return (
      <div className="chart-container">
        {title && <h3 className="chart-title">{title}</h3>}
        <div className="chart-empty">
          <p>No data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="chart-container">
      {title && <h3 className="chart-title">{title}</h3>}
      <ResponsiveContainer width="100%" height={height}>
        <LineChart
          data={data}
          margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
        >
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis
            dataKey="timestamp"
            tickFormatter={formatXAxis}
            stroke="#64748b"
            style={{ fontSize: '12px' }}
          />
          <YAxis
            stroke="#64748b"
            style={{ fontSize: '12px' }}
          />
          <Tooltip
            labelFormatter={formatTooltipLabel}
            contentStyle={{
              backgroundColor: 'white',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              padding: '10px'
            }}
          />
          <Legend
            wrapperStyle={{ fontSize: '14px', paddingTop: '10px' }}
          />
          {activeKeys.map(key => (
            <Line
              key={key}
              type="monotone"
              dataKey={key}
              stroke={lineColors[key] || '#667eea'}
              strokeWidth={2}
              dot={{ r: 3 }}
              activeDot={{ r: 5 }}
              name={key.charAt(0).toUpperCase() + key.slice(1)}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default TimeSeriesChart;
