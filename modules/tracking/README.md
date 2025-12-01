# Tracking Module - School & Corporate Transport

## Overview

High-performance GPS tracking system for school buses and corporate transport with real-time notifications, event tracking, and route management.

## Use Cases

1. **School Transport**: Track buses, students, parents receive notifications
2. **Corporate Transport**: Track employee shuttles, supervisors monitor routes
3. **Mixed Operations**: Transport companies serving multiple schools/corporations

## Key Features

### ✅ High-Performance Architecture
- **Partitioned location table** by month for fast inserts (handles millions of GPS points)
- **Single GPS stream per vehicle** (not per rider) - optimized for high traffic
- **Batch location updates** support
- **Indexed queries** for real-time status

### ✅ Real-Time Tracking
- Vehicle current location with age indicator
- Route status dashboard (boarded/arrived/pending counts)
- Rider status for guardian notifications
- Vehicle location history

### ✅ Event Management
- **Check-in (boarded)**: Student/employee got on bus
- **Checkout (arrived_destination)**: Arrived at school/work
- **No-show**: Did not board as expected
- Events linked to specific stops and routes

### ✅ Route Alerts
- Delay notifications with estimated minutes
- Breakdown/traffic/cancellation alerts
- Severity levels (low, medium, high, critical)
- Affected rider count

### ✅ Multi-Company Support
- Transport companies, schools with own fleets, corporate fleets
- Company types: `school`, `corporate`, `transport_provider`
- Tenant-isolated data

## Database Schema

### Core Entities

**tracking.transport_companies**
- Organizations managing transport (schools, companies, bus companies)
- Types: school, corporate, transport_provider

**tracking.vehicles**
- Buses/vans with GPS tracking
- License plate, capacity, driver info, GPS device ID

**tracking.routes**
- Origin → Destination with scheduled times
- Schedule types: morning (home→school), afternoon (school→home), custom
- Assigned to specific vehicle

**tracking.route_stops**
- Intermediate stops along route
- Ordered sequence with scheduled arrival offsets

**tracking.riders**
- Students or employees using transport
- Guardian/parent linkage to auth.users
- Emergency contact information

**tracking.rider_assignments**
- Maps riders to routes with pickup/dropoff stops
- Active/inactive status

**tracking.vehicle_locations** (PARTITIONED)
- GPS coordinates with timestamp
- Partitioned by month for performance
- Supports speed, heading, altitude, accuracy

**tracking.ride_events**
- Check-in/checkout events for riders
- Linked to stops, routes, vehicles

**tracking.route_alerts**
- Delay/breakdown/cancellation notifications
- Severity and estimated delay

## Stored Procedures (High Performance)

### Location Tracking
- `sp_update_vehicle_location`: Insert GPS coordinates (optimized for high frequency)
- `sp_get_vehicle_current_location`: Get most recent location with age

### Event Management
- `sp_record_ride_event`: Record boarded/arrived/no-show events
- Returns rider name, event type, stop name

### Status Queries
- `sp_get_route_realtime_status`: Comprehensive route dashboard
  - Vehicle location, rider counts, alert count
- `sp_get_rider_status`: Guardian view of their child
  - Bus location, last event, scheduled stops, active alerts

### Alerts
- `sp_create_route_alert`: Create delay/breakdown notification
  - Returns affected rider count for notifications

## Performance Optimizations

1. **Table Partitioning**: `vehicle_locations` partitioned by month (12 partitions pre-created)
2. **Strategic Indexes**:
   - `vehicle_id + recorded_at DESC` for latest location queries
   - `rider_id + event_time DESC` for event history
   - `route_id` for route-based queries
3. **Minimal Validation**: GPS update SP skips heavy validation for speed
4. **LATERAL Joins**: Efficient "latest location" queries
5. **Batch Support**: Location updates can be batched externally

## API Endpoints (To Be Implemented)

### Location Tracking
- `POST /tracking/locations` - Update vehicle location (GPS device)
- `GET /tracking/vehicles/:id/location` - Get current location

### Event Management
- `POST /tracking/events` - Record check-in/checkout
- `GET /tracking/riders/:id/events` - Get rider event history

### Status Queries
- `GET /tracking/routes/:id/status` - Real-time route dashboard
- `GET /tracking/riders/:id/status` - Rider status for guardians

### Alerts
- `POST /tracking/alerts` - Create route alert
- `GET /tracking/routes/:id/alerts` - Get active alerts for route
- `PATCH /tracking/alerts/:id/resolve` - Resolve alert

### CRUD Operations
- Companies, vehicles, routes, route_stops, riders, assignments

## Guardian Notification Flow

1. **School invites parent**: Create rider with `guardian_user_id` (links to auth.users)
2. **Parent logs in**: Use existing auth system
3. **Parent subscribes**: Frontend subscribes to rider status for their children
4. **Bus updates GPS**: Driver app calls `sp_update_vehicle_location` every 5-30 seconds
5. **Student boards**: Driver/automated system calls `sp_record_ride_event` (event_type: 'boarded')
6. **Parent gets notification**: "Juan has boarded the bus" + bus location
7. **Student arrives**: `sp_record_ride_event` (event_type: 'arrived_destination')
8. **Parent gets notification**: "Juan has arrived at school"

**Optimization**: Only **1 GPS stream** (from bus) serves **30+ parents** (not 30 individual GPS streams)

## Scalability Considerations

- **Location partitions**: Add new partitions monthly via cron job
- **Data retention**: Purge old location data after `TRACKING_LOCATION_RETENTION_DAYS`
- **Rate limiting**: `TRACKING_LOCATION_RATE_LIMIT_SEC` prevents GPS spam
- **WebSocket support**: Future enhancement for real-time push notifications
- **Read replicas**: For high-traffic status queries (route dashboard)

## Configuration (from .env)

```env
# GPS location settings
TRACKING_MAX_LOCATION_BATCH_SIZE=100
TRACKING_LOCATION_RETENTION_DAYS=90
TRACKING_LOCATION_RATE_LIMIT_SEC=5

# Real-time notifications
TRACKING_ENABLE_REALTIME_NOTIFICATIONS=true
TRACKING_NOTIFICATION_DELAY_THRESHOLD=15m

# Alert settings
TRACKING_ALERT_AUTO_RESOLVE_HOURS=24
TRACKING_ENABLE_ALERT_NOTIFICATIONS=true

# Performance
TRACKING_ENABLE_LOCATION_PARTITIONING=true
TRACKING_MAX_EVENTS_PER_RIDER=1000
```

## Error Codes

See `modules/tracking/errors/errors.go` for all error constants.

Examples:
- `TR0001`: Vehicle not found
- `TR0002`: Rider not found
- `TR0003`: Route not found

## Next Steps

1. ✅ Database schema created
2. ✅ Stored procedures implemented
3. ✅ Models and DTOs defined
4. ✅ Configuration setup
5. ⏳ Implement repositories (call stored procedures)
6. ⏳ Implement services (business logic)
7. ⏳ Implement controllers (HTTP handlers)
8. ⏳ Register routes
9. ⏳ Add Swagger documentation
10. ⏳ Test with real GPS data

## Migration Status

16 migrations created:
- Schema initialization
- 10 tables (companies, vehicles, routes, stops, riders, assignments, locations, events, alerts)
- 6 stored procedures (update location, get location, record event, route status, rider status, create alert)
