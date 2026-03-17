# Smart City Monitor — Frontend

## Overview

The frontend is a single-page React 19 application built with TypeScript and Vite. It visualizes live sensor data on a Google Maps interface and displays real-time anomaly and event feeds.

**Source:** `frontend/src/`
**Main component:** `frontend/src/App.tsx` (monolithic component)

## Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| React | 19 | UI framework |
| TypeScript | — | Type safety |
| Vite | — | Build tooling and dev server |
| Google Maps API | — | Interactive sensor map |
| SSE (EventSource) | — | Real-time data streaming |

## Key Files

| File | Purpose |
|------|---------|
| `src/App.tsx` | Main application component (monolithic) |
| `src/constants/app.ts` | API URLs, map defaults, thresholds |
| `src/types/app.ts` | TypeScript type definitions |
| `src/lib/maps.ts` | Google Maps initialization and helpers |

## Data Sources

### REST API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/sensors` | Load sensor list |
| `GET /api/v1/readings/latest` | Initial latest readings |
| `GET /api/v1/metrics/nerds` | System metrics for nerd stats page |
| `GET /api/v1/session/config` | Session configuration |

### SSE Stream

Connects to `/stream?session_id=<uuid>` on load. Receives three event types:

| Event Type | Display |
|------------|---------|
| `sensor_update` | Updates sensor markers on map, refreshes stat cards |
| `anomaly` | Appends to anomaly feed panel in sidebar (max 20 events, scrolling) |
| `scenario_detected` | Shows scenario banner at top of page (auto-dismissing) |

## Session Management

- A UUID is generated on first visit and stored in `localStorage` (`SESSION_KEY`).
- The session ID is sent with every API request via `X-Session-ID` header and as SSE query param.
- Sensors created via the UI are scoped to the session.
- On browser close, a `sendBeacon` call to `POST /api/v1/session/teardown` triggers cleanup.

## UI Features

- Interactive Google Maps with color-coded sensor markers by type and status.
- Real-time stat cards showing city-wide averages (temperature, pollution, humidity, noise).
- Anomaly feed panel in sidebar showing recent anomaly detections (scrolling list, max 20).
- Scenario banners that appear when zone-level patterns are detected (auto-dismiss).
- Admin controls for simulation rate, scenario activation, sensor management.
- Stats for Nerds page with SSE-first updates and low-frequency fallback polling.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `VITE_API_URL` | `http://localhost:8080/api/v1` | Backend API base URL |
| `VITE_GOOGLE_MAPS_API_KEY` | — | Google Maps JavaScript API key |

## Development

```bash
cd frontend
npm install
npm run dev       # Dev server at http://localhost:5173
npm run build     # TypeScript check + Vite production build
npm run lint      # ESLint
```
