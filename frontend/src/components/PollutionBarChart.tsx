import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, Cell } from 'recharts';
import './TimeSeriesChart.css';

interface PollutionData {
  area: string;
  pollution: number;
  sensor_count?: number;
}

interface PollutionBarChartProps {
  data: PollutionData[];
  title?: string;
  height?: number;
}

const PollutionBarChart = ({ data, title = 'Top Polluted Areas', height = 300 }: PollutionBarChartProps) => {
  // Color gradient based on pollution level
  const getBarColor = (value: number) => {
    if (value > 150) return '#dc2626'; // Unhealthy
    if (value > 100) return '#f59e0b'; // Moderate
    if (value > 50) return '#fbbf24'; // Fair
    return '#22c55e'; // Good
  };

  // Custom tooltip
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload;
      return (
        <div style={{
          backgroundColor: 'white',
          border: '1px solid #e2e8f0',
          borderRadius: '8px',
          padding: '12px',
          boxShadow: '0 2px 8px rgba(0,0,0,0.1)'
        }}>
          <p style={{ margin: '0 0 8px 0', fontWeight: 600, color: '#1e293b' }}>
            {data.area}
          </p>
          <p style={{ margin: '4px 0', color: '#64748b', fontSize: '14px' }}>
            <strong>Pollution:</strong> {data.pollution.toFixed(1)} AQI
          </p>
          {data.sensor_count && (
            <p style={{ margin: '4px 0', color: '#64748b', fontSize: '14px' }}>
              <strong>Sensors:</strong> {data.sensor_count}
            </p>
          )}
        </div>
      );
    }
    return null;
  };

  if (!data || data.length === 0) {
    return (
      <div className="chart-container">
        {title && <h3 className="chart-title">{title}</h3>}
        <div className="chart-empty">
          <p>No pollution data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="chart-container">
      {title && <h3 className="chart-title">{title}</h3>}
      <ResponsiveContainer width="100%" height={height}>
        <BarChart
          data={data}
          margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
        >
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis
            dataKey="area"
            stroke="#64748b"
            style={{ fontSize: '12px' }}
            angle={-45}
            textAnchor="end"
            height={80}
          />
          <YAxis
            stroke="#64748b"
            style={{ fontSize: '12px' }}
            label={{ value: 'AQI', angle: -90, position: 'insideLeft' }}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend
            wrapperStyle={{ fontSize: '14px', paddingTop: '10px' }}
          />
          <Bar
            dataKey="pollution"
            name="Pollution Level (AQI)"
            radius={[8, 8, 0, 0]}
          >
            {data.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={getBarColor(entry.pollution)} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
};

export default PollutionBarChart;
