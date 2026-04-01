// Package internal contains the fleet management business logic.
//
// The FleetManager tracks registered IronGolem OS instances, receives
// health reports, and aggregates fleet-wide statistics for the dashboard.
package internal

import (
	"log/slog"
	"sync"
	"time"
)

// InstanceStatus represents the operational state of a managed instance.
type InstanceStatus string

const (
	InstanceStatusOnline   InstanceStatus = "online"
	InstanceStatusDegraded InstanceStatus = "degraded"
	InstanceStatusOffline  InstanceStatus = "offline"
	InstanceStatusUpdating InstanceStatus = "updating"
)

// Instance represents a single IronGolem OS deployment managed by the fleet.
type Instance struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	URL              string         `json:"url"`
	Version          string         `json:"version"`
	Status           InstanceStatus `json:"status"`
	Region           string         `json:"region"`
	LastHealthReport *HealthReport  `json:"last_health_report,omitempty"`
	RegisteredAt     time.Time      `json:"registered_at"`
	LastSeenAt       time.Time      `json:"last_seen_at"`
}

// ServiceStatusEntry holds the health state of one service within an instance.
type ServiceStatusEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Uptime string `json:"uptime,omitempty"`
}

// ConnectorStatusEntry holds the health state of one connector.
type ConnectorStatusEntry struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Latency   string `json:"latency,omitempty"`
}

// ResourceUsage captures resource consumption for an instance.
type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPercent   float64 `json:"disk_percent"`
}

// HealthReport is the periodic health payload an instance sends to the fleet.
type HealthReport struct {
	InstanceID        string                 `json:"instance_id"`
	ReportedAt        time.Time              `json:"reported_at"`
	ServiceStatuses   []ServiceStatusEntry   `json:"service_statuses"`
	ConnectorStatuses []ConnectorStatusEntry  `json:"connector_statuses"`
	Resources         ResourceUsage          `json:"resources"`
	ActiveRecipes     int                    `json:"active_recipes"`
	ActiveSquads      int                    `json:"active_squads"`
}

// FleetOverview is the aggregated summary returned by the dashboard endpoint.
type FleetOverview struct {
	TotalInstances int                       `json:"total_instances"`
	ByStatus       map[InstanceStatus]int    `json:"by_status"`
	ByRegion       map[string]int            `json:"by_region"`
	AverageHealth  float64                   `json:"average_health"`
	TotalRecipes   int                       `json:"total_recipes"`
	TotalSquads    int                       `json:"total_squads"`
	GeneratedAt    time.Time                 `json:"generated_at"`
}

// FleetManager manages the lifecycle and health tracking of fleet instances.
type FleetManager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	logger    *slog.Logger
}

// NewFleetManager creates a new FleetManager.
func NewFleetManager(logger *slog.Logger) *FleetManager {
	return &FleetManager{
		instances: make(map[string]*Instance),
		logger:    logger,
	}
}

// Register adds a new instance to the fleet.
func (fm *FleetManager) Register(inst Instance) *Instance {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	inst.RegisteredAt = time.Now().UTC()
	inst.LastSeenAt = inst.RegisteredAt
	if inst.Status == "" {
		inst.Status = InstanceStatusOnline
	}

	fm.instances[inst.ID] = &inst
	fm.logger.Info("instance registered",
		slog.String("id", inst.ID),
		slog.String("name", inst.Name),
		slog.String("region", inst.Region),
	)
	return &inst
}

// Unregister removes an instance from the fleet.
func (fm *FleetManager) Unregister(id string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.instances[id]; !ok {
		return false
	}
	delete(fm.instances, id)
	fm.logger.Info("instance unregistered", slog.String("id", id))
	return true
}

// Get returns an instance by ID, or nil if not found.
func (fm *FleetManager) Get(id string) *Instance {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	inst := fm.instances[id]
	return inst
}

// List returns all registered instances.
func (fm *FleetManager) List() []*Instance {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make([]*Instance, 0, len(fm.instances))
	for _, inst := range fm.instances {
		result = append(result, inst)
	}
	return result
}

// RecordHealth processes a health report from an instance and updates its
// status accordingly.
func (fm *FleetManager) RecordHealth(report HealthReport) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	inst, ok := fm.instances[report.InstanceID]
	if !ok {
		return false
	}

	report.ReportedAt = time.Now().UTC()
	inst.LastHealthReport = &report
	inst.LastSeenAt = report.ReportedAt
	inst.Status = deriveStatus(report)

	fm.logger.Info("health report recorded",
		slog.String("instance_id", report.InstanceID),
		slog.String("status", string(inst.Status)),
	)
	return true
}

// Overview computes and returns a fleet-wide summary.
func (fm *FleetManager) Overview() FleetOverview {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	overview := FleetOverview{
		TotalInstances: len(fm.instances),
		ByStatus:       make(map[InstanceStatus]int),
		ByRegion:       make(map[string]int),
		GeneratedAt:    time.Now().UTC(),
	}

	healthyCount := 0
	for _, inst := range fm.instances {
		overview.ByStatus[inst.Status]++
		if inst.Region != "" {
			overview.ByRegion[inst.Region]++
		}
		if inst.Status == InstanceStatusOnline {
			healthyCount++
		}
		if inst.LastHealthReport != nil {
			overview.TotalRecipes += inst.LastHealthReport.ActiveRecipes
			overview.TotalSquads += inst.LastHealthReport.ActiveSquads
		}
	}

	if overview.TotalInstances > 0 {
		overview.AverageHealth = float64(healthyCount) / float64(overview.TotalInstances) * 100.0
	}

	return overview
}

// deriveStatus determines the instance status based on a health report.
func deriveStatus(report HealthReport) InstanceStatus {
	// If resource usage is very high, mark as degraded.
	if report.Resources.CPUPercent > 90 || report.Resources.MemoryPercent > 90 {
		return InstanceStatusDegraded
	}

	// If any service is down, mark as degraded.
	for _, svc := range report.ServiceStatuses {
		if svc.Status != "healthy" && svc.Status != "ok" {
			return InstanceStatusDegraded
		}
	}

	return InstanceStatusOnline
}
