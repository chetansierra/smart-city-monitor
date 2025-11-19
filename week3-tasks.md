# Week 3 Tasks - Frontend Development

> **Duration**: Dec 3-9, 2025
> **Goal**: React dashboard with map, real-time updates, and charts

---

## Day 1 (Dec 3) - React Setup & Map ✅

- [x] **1.1** Create React app with Vite + TypeScript
- [x] **1.2** Install dependencies (Leaflet, Recharts, axios, socket.io-client)
- [x] **1.3** Setup Leaflet map component
- [x] **1.4** Display sensors as markers on map
- [x] **1.5** Add marker popups with sensor details

## Day 2 (Dec 4) - Dashboard Layout ✅

- [x] **2.1** Create main dashboard layout (header, sidebar, map, stats)
- [x] **2.2** Build city stats cards (temp, pollution, humidity, noise)
- [x] **2.3** Create sensor list sidebar with filters
- [x] **2.4** Add responsive design (mobile/desktop)

## Day 3 (Dec 5) - Charts & Analytics ✅

- [x] **3.1** Build time-series chart for sensor readings
- [x] **3.2** Create bar chart for top polluted areas
- [x] **3.3** Add real-time updating charts
- [x] **3.4** Implement date range selector

## Day 4 (Dec 6) - Real-time Updates

- [ ] **4.1** Setup WebSocket connection to API
- [ ] **4.2** Subscribe to sensor updates
- [ ] **4.3** Update map markers in real-time
- [ ] **4.4** Update charts and stats live
- [ ] **4.5** Add connection status indicator

## Day 5 (Dec 7) - Alerts & Notifications

- [ ] **5.1** Create alerts panel component
- [ ] **5.2** Display active alerts with severity badges
- [ ] **5.3** Real-time alert notifications (toast/banner)
- [ ] **5.4** Alert acknowledgment functionality
- [ ] **5.5** Filter alerts by severity/sensor

## Day 6 (Dec 8) - Polish & Testing

- [ ] **6.1** Add loading states and error handling
- [ ] **6.2** Implement data caching and optimization
- [ ] **6.3** Add dark mode toggle
- [ ] **6.4** Test on different browsers
- [ ] **6.5** Mobile responsiveness testing

## Day 7 (Dec 9) - Deployment

- [ ] **7.1** Build production bundle
- [ ] **7.2** Add nginx configuration
- [ ] **7.3** Update docker-compose with frontend service
- [ ] **7.4** Test full stack deployment
- [ ] **7.5** Document frontend setup in README

---

## Tech Stack

- **Framework**: React 18 + Vite + TypeScript
- **Map**: Leaflet + React-Leaflet
- **Charts**: Recharts
- **HTTP**: Axios
- **WebSocket**: socket.io-client
- **Styling**: CSS Modules / Tailwind CSS
- **State**: React Context / Zustand

## Key Features

1. Interactive map with sensor locations
2. Real-time sensor data updates
3. City-wide statistics dashboard
4. Time-series and comparison charts
5. Live alert notifications
6. Responsive mobile/desktop UI
7. Dark mode support

## API Integration

- `GET /api/v1/sensors` - Sensor list
- `GET /api/v1/readings/latest` - Latest readings
- `GET /api/v1/analytics/city-stats` - Dashboard stats
- `GET /api/v1/alerts` - Active alerts
- `WS /ws` - Real-time updates

---

**Target**: Fully functional React dashboard by Dec 9
