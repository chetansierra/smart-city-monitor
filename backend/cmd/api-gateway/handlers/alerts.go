package handlers

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AlertsHandler handles alert-related requests
type AlertsHandler struct {
	db          *postgres.DB
	redisClient *redis.Client
}

// NewAlertsHandler creates a new alerts handler
func NewAlertsHandler(db *postgres.DB, redisClient *redis.Client) *AlertsHandler {
	return &AlertsHandler{
		db:          db,
		redisClient: redisClient,
	}
}

// Alert represents an alert in the system
type Alert struct {
	ID             int64      `json:"id"`
	SensorID       string     `json:"sensor_id"`
	SensorName     string     `json:"sensor_name,omitempty"`
	SensorType     string     `json:"sensor_type,omitempty"`
	AlertType      string     `json:"alert_type"`
	Severity       string     `json:"severity"`
	Message        string     `json:"message"`
	Value          float64    `json:"value"`
	Threshold      float64    `json:"threshold"`
	Timestamp      time.Time  `json:"timestamp"`
	Acknowledged   bool       `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string    `json:"acknowledged_by,omitempty"`
}

// AcknowledgeRequest represents an acknowledgment request
type AcknowledgeRequest struct {
	AcknowledgedBy string `json:"acknowledged_by"`
}

// GetAll returns all alerts with optional filtering
// GET /api/v1/alerts?active=true&severity=high&limit=50&page=1
func (h *AlertsHandler) GetAll(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	active := c.Query("active", "")
	severity := c.Query("severity", "")
	limitStr := c.Query("limit", "50")
	pageStr := c.Query("page", "1")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// Build query
	query := `
		SELECT a.id, a.sensor_id, a.alert_type, a.severity, a.message,
		       a.value, a.threshold, a.timestamp, a.acknowledged,
		       a.acknowledged_at, a.acknowledged_by,
		       s.name as sensor_name, s.type as sensor_type
		FROM alerts a
		JOIN sensors s ON a.sensor_id = s.id
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if active == "true" {
		query += ` AND a.acknowledged = false`
	} else if active == "false" {
		query += ` AND a.acknowledged = true`
	}

	if severity != "" {
		query += ` AND a.severity = $` + strconv.Itoa(argIdx)
		args = append(args, severity)
		argIdx++
	}

	query += ` ORDER BY a.timestamp DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query alerts")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to retrieve alerts",
			},
		})
	}
	defer rows.Close()

	alerts := []Alert{}
	for rows.Next() {
		var alert Alert
		var sensorName, sensorType sql.NullString

		err := rows.Scan(
			&alert.ID,
			&alert.SensorID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Message,
			&alert.Value,
			&alert.Threshold,
			&alert.Timestamp,
			&alert.Acknowledged,
			&alert.AcknowledgedAt,
			&alert.AcknowledgedBy,
			&sensorName,
			&sensorType,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan alert")
			continue
		}

		if sensorName.Valid {
			alert.SensorName = sensorName.String
		}
		if sensorType.Valid {
			alert.SensorType = sensorType.String
		}

		alerts = append(alerts, alert)
	}

	// Get total count for pagination
	countQuery := `SELECT COUNT(*) FROM alerts WHERE 1=1`
	countArgs := []interface{}{}
	if active == "true" {
		countQuery += ` AND acknowledged = false`
	} else if active == "false" {
		countQuery += ` AND acknowledged = true`
	}
	if severity != "" {
		countQuery += ` AND severity = $1`
		countArgs = append(countArgs, severity)
	}

	var total int
	err = h.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		log.Error().Err(err).Msg("Failed to count alerts")
		total = len(alerts)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    alerts,
		Meta: &Meta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GetByID returns a specific alert by ID
// GET /api/v1/alerts/:id
func (h *AlertsHandler) GetByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_ID",
				Message: "Invalid alert ID",
			},
		})
	}

	query := `
		SELECT a.id, a.sensor_id, a.alert_type, a.severity, a.message,
		       a.value, a.threshold, a.timestamp, a.acknowledged,
		       a.acknowledged_at, a.acknowledged_by,
		       s.name as sensor_name, s.type as sensor_type
		FROM alerts a
		JOIN sensors s ON a.sensor_id = s.id
		WHERE a.id = $1
	`

	var alert Alert
	var sensorName, sensorType sql.NullString

	err = h.db.QueryRowContext(ctx, query, id).Scan(
		&alert.ID,
		&alert.SensorID,
		&alert.AlertType,
		&alert.Severity,
		&alert.Message,
		&alert.Value,
		&alert.Threshold,
		&alert.Timestamp,
		&alert.Acknowledged,
		&alert.AcknowledgedAt,
		&alert.AcknowledgedBy,
		&sensorName,
		&sensorType,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "ALERT_NOT_FOUND",
				Message: "Alert not found",
			},
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to query alert")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to retrieve alert",
			},
		})
	}

	if sensorName.Valid {
		alert.SensorName = sensorName.String
	}
	if sensorType.Valid {
		alert.SensorType = sensorType.String
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    alert,
	})
}

// Acknowledge marks an alert as acknowledged
// POST /api/v1/alerts/:id/acknowledge
func (h *AlertsHandler) Acknowledge(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_ID",
				Message: "Invalid alert ID",
			},
		})
	}

	var req AcknowledgeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request body",
			},
		})
	}

	if req.AcknowledgedBy == "" {
		req.AcknowledgedBy = "system"
	}

	// Update alert
	now := time.Now()
	query := `
		UPDATE alerts
		SET acknowledged = true,
		    acknowledged_at = $1,
		    acknowledged_by = $2
		WHERE id = $3 AND acknowledged = false
		RETURNING id
	`

	var alertID int64
	err = h.db.QueryRowContext(ctx, query, now, req.AcknowledgedBy, id).Scan(&alertID)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "ALERT_NOT_FOUND",
				Message: "Alert not found or already acknowledged",
			},
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to acknowledge alert")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to acknowledge alert",
			},
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":              id,
			"acknowledged":    true,
			"acknowledged_at": now,
			"acknowledged_by": req.AcknowledgedBy,
		},
	})
}

// GetBySensorID returns alerts for a specific sensor
// GET /api/v1/sensors/:id/alerts?limit=50
func (h *AlertsHandler) GetBySensorID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sensorIDStr := c.Params("id")
	sensorID, err := uuid.Parse(sensorIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "INVALID_SENSOR_ID",
				Message: "Invalid sensor ID format",
			},
		})
	}

	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	query := `
		SELECT a.id, a.sensor_id, a.alert_type, a.severity, a.message,
		       a.value, a.threshold, a.timestamp, a.acknowledged,
		       a.acknowledged_at, a.acknowledged_by,
		       s.name as sensor_name, s.type as sensor_type
		FROM alerts a
		JOIN sensors s ON a.sensor_id = s.id
		WHERE a.sensor_id = $1
		ORDER BY a.timestamp DESC
		LIMIT $2
	`

	rows, err := h.db.QueryContext(ctx, query, sensorID, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query sensor alerts")
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "DATABASE_ERROR",
				Message: "Failed to retrieve alerts",
			},
		})
	}
	defer rows.Close()

	alerts := []Alert{}
	for rows.Next() {
		var alert Alert
		var sensorName, sensorType sql.NullString

		err := rows.Scan(
			&alert.ID,
			&alert.SensorID,
			&alert.AlertType,
			&alert.Severity,
			&alert.Message,
			&alert.Value,
			&alert.Threshold,
			&alert.Timestamp,
			&alert.Acknowledged,
			&alert.AcknowledgedAt,
			&alert.AcknowledgedBy,
			&sensorName,
			&sensorType,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan alert")
			continue
		}

		if sensorName.Valid {
			alert.SensorName = sensorName.String
		}
		if sensorType.Valid {
			alert.SensorType = sensorType.String
		}

		alerts = append(alerts, alert)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    alerts,
		Meta: &Meta{
			Total: len(alerts),
		},
	})
}
