package docking

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"
)

func getFloat64(input map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := input[key].(float64); ok {
		return v
	}
	return defaultVal
}

func getInt(input map[string]interface{}, key string, defaultVal int) int {
	if v, ok := input[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

func getBool(input map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := input[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getString(input map[string]interface{}, key string, defaultVal string) string {
	if v, ok := input[key].(string); ok {
		return v
	}
	return defaultVal
}

func mockDelay(ctx context.Context, minMs, maxMs int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond):
	}
	return nil
}

func filepathBase(path string) string {
	return filepath.Base(path)
}

func fmtReceptorLigandCmd(receptor, ligand, tool string) string {
	return fmt.Sprintf("%s --receptor %s --ligand %s ...", tool, filepathBase(receptor), filepathBase(ligand))
}