# PostgreSQL Architecture Notes

## Role

PostgreSQL is the durable source of truth for sensors, readings, and analytics data.

## Usage

- Data ingestion writes sensor readings.
- API gateway reads sensors/readings and analytics data.
- Admin operations mutate sensor lifecycle records.

Redis should not be treated as persistent history storage.
