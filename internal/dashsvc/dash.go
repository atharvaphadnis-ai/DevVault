// Package dashsvc aggregates dashboard statistics.
package dashsvc

import (
	"context"
	"database/sql"
	"math"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// Stats is the dashboard payload.
type Stats struct {
	Projects      int            `json:"projects"`
	Snippets      int            `json:"snippets"`
	TasksByStatus map[string]int `json:"tasksByStatus"`
	RemoteEnabled bool           `json:"remoteEnabled"`
	CPUPercent    float64        `json:"cpuPercent"`
	MemPercent    float64        `json:"memPercent"`
	MemTotal      uint64         `json:"memTotal"`
	MemUsed       uint64         `json:"memUsed"`
	GoVersion     string         `json:"goVersion"`
	NumCPU        int            `json:"numCPU"`
	GeneratedAt   string         `json:"generatedAt"`
	Remote        RemoteInfo     `json:"remote"`
}

// RemoteInfo summarises DevVault Anywhere state for the dashboard.
type RemoteInfo struct {
	Enabled         bool   `json:"enabled"`
	Connected       bool   `json:"connected"`
	OnlineDevices   int    `json:"onlineDevices"`
	PendingRequests int    `json:"pendingRequests"`
	Detail          string `json:"detail,omitempty"`
}

type Service struct {
	db *sql.DB
	// RemoteStatus is injected by the app to avoid an import cycle.
	RemoteStatus func() RemoteInfo
}

func New(db *sql.DB) *Service { return &Service{db: db} }

// Get computes current stats. CPU sampling uses a short interval.
func (s *Service) Get() (*Stats, error) {
	st := &Stats{TasksByStatus: map[string]int{}, GoVersion: runtime.Version(), NumCPU: runtime.NumCPU()}
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&st.Projects)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM snippets`).Scan(&st.Snippets)
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k string
			var v int
			if rows.Scan(&k, &v) == nil {
				st.TasksByStatus[k] = v
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pcts, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(pcts) > 0 {
		st.CPUPercent = round1(pcts[0])
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		st.MemPercent = round1(vm.UsedPercent)
		st.MemTotal = vm.Total
		st.MemUsed = vm.Used
	}
	if s.RemoteStatus != nil {
		st.Remote = s.RemoteStatus()
		st.RemoteEnabled = st.Remote.Enabled
	}
	st.GeneratedAt = time.Now().Format(time.RFC3339)
	return st, nil
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
