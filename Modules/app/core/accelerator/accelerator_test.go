//go:build ignore
// +build ignore

package accelerator_test

import (
	"os"
	"testing"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/core/accelerator"
	"github.com/stretchr/testify/assert"
)

// helper to clear environment flags
func clearEnv() {
	_ = os.Unsetenv("HAS_GPU")
	_ = os.Unsetenv("HAS_TPU")
}

func TestDetectDevices_CPUOnly(t *testing.T) {
	clearEnv()

	info := accelerator.DetectDevices()

	assert.NotEmpty(t, info.Devices, "CPU device should be detected")
	assert.Equal(t, accelerator.CPU, info.Devices[0].Type)
	assert.True(t, info.Devices[0].Cores > 0)

	assert.Contains(t, info.Warnings, "No GPU detected")
	assert.Contains(t, info.Warnings, "No TPU detected")

	assert.Empty(t, info.Errors)
}

func TestDetectDevices_WithGPU(t *testing.T) {
	clearEnv()
	_ = os.Setenv("HAS_GPU", "1")

	info := accelerator.DetectDevices()

	foundGPU := false
	for _, d := range info.Devices {
		if d.Type == accelerator.GPU {
			foundGPU = true
			assert.Equal(t, "Generic GPU", d.Name)
		}
	}

	assert.True(t, foundGPU, "GPU should be detected")
	assert.Contains(t, info.Info, "Generic GPU detected")
}

func TestDetectDevices_WithTPU(t *testing.T) {
	clearEnv()
	_ = os.Setenv("HAS_TPU", "1")

	info := accelerator.DetectDevices()

	foundTPU := false
	for _, d := range info.Devices {
		if d.Type == accelerator.TPU {
			foundTPU = true
			assert.Equal(t, "Generic TPU", d.Name)
		}
	}

	assert.True(t, foundTPU, "TPU should be detected")
	assert.Contains(t, info.Info, "Generic TPU detected")
}

func TestDetectDevices_GPUAndTPU(t *testing.T) {
	clearEnv()
	_ = os.Setenv("HAS_GPU", "1")
	_ = os.Setenv("HAS_TPU", "1")

	info := accelerator.DetectDevices()

	var gpu, tpu bool
	for _, d := range info.Devices {
		if d.Type == accelerator.GPU {
			gpu = true
		}
		if d.Type == accelerator.TPU {
			tpu = true
		}
	}

	assert.True(t, gpu)
	assert.True(t, tpu)
	assert.Empty(t, info.Warnings)
}

func TestGetBestDevice(t *testing.T) {
	clearEnv()
	_ = os.Setenv("HAS_GPU", "1")
	_ = os.Setenv("HAS_TPU", "1")

	best, info, warn, errs := accelerator.GetBestDevice()

	assert.NotNil(t, best)
	assert.Empty(t, errs)
	assert.NotEmpty(t, info)
	assert.Empty(t, warn)

	// TPU has highest score = 80
	assert.Equal(t, accelerator.TPU, best.Type)
	assert.Equal(t, 80, best.Score)
}

func TestGetBestDevice_NoDevices(t *testing.T) {
	// Simulate zero CPU core scenario? Not possible directly.
	// Instead: unset GPU and TPU and rely on CPU being always present.
	// So this test ensures stability rather than forcing errors.

	clearEnv()
	best, info, warn, errs := accelerator.GetBestDevice()

	assert.NotNil(t, info)
	assert.NotNil(t, warn)
	assert.Empty(t, errs, "CPU should always exist on Go runtime")
	assert.Equal(t, accelerator.CPU, best.Type)
}

func TestDeviceString_CPU(t *testing.T) {
	d := accelerator.Device{
		Type:  accelerator.CPU,
		ID:    0,
		Name:  "CPU x8 cores",
		Score: 10,
		Cores: 8,
	}

	out := d.String()
	assert.Contains(t, out, "cpu:0")
	assert.Contains(t, out, "CPU x8 cores")
	assert.Contains(t, out, "score=10")
}

func TestDeviceString_Generic(t *testing.T) {
	d := accelerator.Device{
		Type:  accelerator.GPU,
		ID:    2,
		Name:  "Test GPU",
		Score: 40,
	}

	out := d.String()
	assert.Contains(t, out, "gpu:2")
	assert.Contains(t, out, "Test GPU")
	assert.Contains(t, out, "score=40")
}
