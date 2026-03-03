# Frontend Architecture

## Stack

- React + TypeScript + Vite
- Tailwind/CSS styling
- Browser EventSource for SSE

## Responsibilities

- Session bootstrap and persistence in browser.
- Home experience and sensor controls.
- Live chart rendering from SSE updates.
- Map rendering and sensor markers.
- Stats for Nerds page with SSE-first updates and low-frequency fallback polling.

## Data Contracts Used

- REST: `/api/v1/sensors`, `/api/v1/readings/latest`, `/api/v1/metrics/nerds`, `/api/v1/session/config`
- SSE: `/stream?session_id=<id>`
  - `sensor_update`
  - `stats_update`
