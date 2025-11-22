package Resource

import (
	"fmt"
	"os"
	"runtime"
)

// DeviceType represents the type of compute device.
type DeviceType string

const (
	CPU DeviceType = "cpu"
	GPU DeviceType = "gpu"
	TPU DeviceType = "tpu"
)

// Device holds information about a compute device.
type Device struct {
	Type  DeviceType
	ID    int
	Name  string
	Score int
	Cores int // only for CPU summarization
}

// ResourceInfo holds the results of detection, including any messages.
type ResourceInfo struct {
	Devices  []Device
	Warnings []string
	Info     []string
	Errors   []error
}

// DetectDevices detects CPU, GPU, TPU in a vendor-agnostic way
func DetectDevices() ResourceInfo {
	info := ResourceInfo{}

	// CPU detection (summarized)
	numCPU := runtime.NumCPU()
	if numCPU < 1 {
		info.Errors = append(info.Errors, fmt.Errorf("no CPU cores detected"))
	} else {
		info.Info = append(info.Info, fmt.Sprintf("%d CPU cores detected", numCPU))
		info.Devices = append(info.Devices, Device{
			Type:  CPU,
			ID:    0,
			Name:  fmt.Sprintf("CPU x%d cores", numCPU),
			Score: 10,
			Cores: numCPU,
		})
	}

	// GPU detection (generic, environment variable)
	if os.Getenv("HAS_GPU") == "1" {
		info.Devices = append(info.Devices, Device{
			Type:  GPU,
			ID:    0,
			Name:  "Generic GPU",
			Score: 50,
		})
		info.Info = append(info.Info, "Generic GPU detected")
	} else {
		info.Warnings = append(info.Warnings, "No GPU detected")
	}

	// TPU detection (generic, environment variable)
	if os.Getenv("HAS_TPU") == "1" {
		info.Devices = append(info.Devices, Device{
			Type:  TPU,
			ID:    0,
			Name:  "Generic TPU",
			Score: 80,
		})
		info.Info = append(info.Info, "Generic TPU detected")
	} else {
		info.Warnings = append(info.Warnings, "No TPU detected")
	}

	// Final check: at least one device must exist
	if len(info.Devices) == 0 {
		info.Errors = append(info.Errors, fmt.Errorf("no devices available"))
	}

	return info
}

// GetBestDevice returns the device with highest score or an error
func GetBestDevice() (Device, []string, []string, []error) {
	info := DetectDevices()

	if len(info.Devices) == 0 {
		return Device{}, info.Info, info.Warnings, info.Errors
	}

	best := info.Devices[0]
	for _, d := range info.Devices {
		if d.Score > best.Score {
			best = d
		}
	}

	return best, info.Info, info.Warnings, info.Errors
}

func (d Device) String() string {
	if d.Type == CPU && d.Cores > 0 {
		return fmt.Sprintf("%s:%d (%s, score=%d)", d.Type, d.ID, d.Name, d.Score)
	}
	return fmt.Sprintf("%s:%d (%s, score=%d)", d.Type, d.ID, d.Name, d.Score)
}
