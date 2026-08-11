package shared

import "time"

type ExperimentSpec struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Env         map[string]string `json:"env"`
	Timeout     time.Duration     `json:"timeout"`
	Resources   ResourceLimits    `json:"resources"`
	InputFiles  map[string]string `json:"input_files"`
	OutputFiles []string          `json:"output_files"`
}

type ResourceLimits struct {
	MaxCPUPercent  float64 `json:"max_cpu_percent"`
	MaxMemoryMB    int64   `json:"max_memory_mb"`
	MaxDiskMB      int64   `json:"max_disk_mb"`
	MaxProcesses   int     `json:"max_processes"`
	NetworkEnabled bool    `json:"network_enabled"`
}