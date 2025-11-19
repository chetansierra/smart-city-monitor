# Smart City Monitor - Frontend

A React-based dashboard for monitoring smart city sensors in real-time.

## Features

- Interactive map with sensor locations using Leaflet
- Real-time sensor data display
- Custom markers based on sensor type
- Detailed popups with latest readings
- Responsive design

## Tech Stack

- **React 18** - UI framework
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **Leaflet** - Interactive maps
- **React-Leaflet** - React components for Leaflet
- **Axios** - HTTP client
- **Socket.io-client** - WebSocket connections
- **Recharts** - Charts and data visualization

## Prerequisites

- Node.js 18+ and npm
- Backend API running on http://localhost:8080

## Setup

1. Install dependencies:
```bash
npm install
```

2. Configure environment variables:
```bash
cp .env.example .env
```

Edit `.env` to set your API URL:
```
VITE_API_URL=http://localhost:8080/api/v1
```

3. Start the development server:
```bash
npm run dev
```

The app will be available at http://localhost:5173

## Development

### Project Structure

```
src/
├── components/       # React components
│   └── Map.tsx      # Map component with sensor markers
├── services/        # API services
│   └── api.ts       # API client and endpoints
├── types/           # TypeScript type definitions
│   └── sensor.ts    # Sensor-related types
├── App.tsx          # Main app component
└── main.tsx         # App entry point
```

### Available Scripts

- `npm run dev` - Start development server with hot reload
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

## API Integration

The frontend connects to the following API endpoints:

- `GET /api/v1/sensors` - Get all sensors
- `GET /api/v1/readings/latest` - Get latest readings for all sensors
- `GET /api/v1/analytics/city-stats` - Get city-wide statistics
- `GET /api/v1/alerts` - Get active alerts
- `WS /ws` - WebSocket for real-time updates

## Map Features

### Sensor Markers

- Color-coded by sensor type:
  - 🔴 Temperature sensors (red)
  - 🔵 Humidity sensors (blue)
  - 🟢 Pollution sensors (teal)
  - 🟠 Noise sensors (orange)

### Popups

Click any marker to view:
- Sensor name and type
- Current status
- Latest readings with units
- Timestamp of last update
- Area/location information

## Next Steps (Week 3 Day 2-7)

- [ ] Dashboard layout with stats cards
- [ ] Time-series charts
- [ ] Real-time WebSocket updates
- [ ] Alert notifications
- [ ] Dark mode toggle
- [ ] Mobile optimization

## Troubleshooting

### Map not loading
- Ensure the backend API is running
- Check console for CORS errors
- Verify API URL in `.env` file

### Markers not showing
- Confirm sensors exist in database
- Check browser console for API errors
- Verify sensor location data has valid lat/lon

## License

MIT
