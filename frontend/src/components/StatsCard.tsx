import './StatsCard.css';

interface StatsCardProps {
  title: string;
  value: string | number;
  unit: string;
  icon: string;
  color: string;
  trend?: {
    value: number;
    isPositive: boolean;
  };
}

const StatsCard = ({ title, value, unit, icon, color, trend }: StatsCardProps) => {
  return (
    <div className="stats-card" style={{ borderLeftColor: color }}>
      <div className="stats-card-header">
        <div className="stats-card-icon" style={{ backgroundColor: color }}>
          {icon}
        </div>
        <h3 className="stats-card-title">{title}</h3>
      </div>
      <div className="stats-card-body">
        <div className="stats-card-value">
          {value}
          <span className="stats-card-unit">{unit}</span>
        </div>
        {trend && (
          <div className={`stats-card-trend ${trend.isPositive ? 'positive' : 'negative'}`}>
            <span>{trend.isPositive ? '↑' : '↓'}</span>
            {Math.abs(trend.value)}%
          </div>
        )}
      </div>
    </div>
  );
};

export default StatsCard;
