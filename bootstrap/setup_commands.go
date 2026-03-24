// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const setupCmdTimeout = 90 * time.Second

type commandRunner func(ctx context.Context, workdir string, name string, args ...string) error
type commandLocator func(name string) (string, error)

type installer struct {
	name    string
	check   string
	install func(ctx context.Context, run commandRunner) error
}

type startupCmd struct {
	name  string
	check string
	cmd   string
	args  []string
}

func commandExists(name string) (string, error) {
	return exec.LookPath(name)
}

func runSetupCommand(
	ctx context.Context,
	workdir string,
	name string,
	args ...string,
) error {
	if !isAllowedSetupCommand(name) {
		return fmt.Errorf("unsupported setup command: %s", name)
	}
	cmdPath, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("setup command %q not found: %w", name, err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, setupCmdTimeout)
	defer cancel()

	// #nosec G204 -- cmdPath/args are from internal allowlisted setup command definitions.
	cmd := exec.CommandContext(cmdCtx, cmdPath, args...)
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = workdir
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), msg)
}

func isAllowedSetupCommand(name string) bool {
	switch name {
	case "go", "sudo", "brew", "winget", "redis-server":
		return true
	default:
		return false
	}
}

func installRedisIfMissing(
	ctx context.Context,
	run commandRunner,
	exists commandLocator,
) error {
	if _, err := exists("redis-server"); err == nil {
		return nil
	}
	errs := make([]string, 0, len(redisInstallers()))
	for _, item := range redisInstallers() {
		if _, err := exists(item.check); err != nil {
			continue
		}
		if err := item.install(ctx, run); err == nil {
			return nil
		} else {
			errs = append(errs, item.name+": "+err.Error())
		}
	}
	if len(errs) == 0 {
		return errors.New("redis auto-install skipped: no supported package manager found")
	}
	return fmt.Errorf("redis auto-install failed: %s", strings.Join(errs, "; "))
}

func startRedisService(
	ctx context.Context,
	run commandRunner,
	exists commandLocator,
) error {
	errs := make([]string, 0, len(redisStartCommands()))
	for _, c := range redisStartCommands() {
		if _, err := exists(c.check); err != nil {
			continue
		}
		if err := run(ctx, "", c.cmd, c.args...); err == nil {
			return nil
		} else {
			errs = append(errs, c.name+": "+err.Error())
		}
	}
	if len(errs) == 0 {
		return errors.New("redis auto-start skipped: no known service command found")
	}
	return fmt.Errorf("redis auto-start failed: %s", strings.Join(errs, "; "))
}

func redisInstallers() []installer {
	return []installer{
		{name: "apt-get", check: "apt-get", install: installWithApt},
		{name: "dnf", check: "dnf", install: installWithDnf},
		{name: "yum", check: "yum", install: installWithYum},
		{name: "pacman", check: "pacman", install: installWithPacman},
		{name: "brew", check: "brew", install: installWithBrew},
		{name: "winget", check: "winget", install: installWithWinget},
	}
}

func installWithApt(ctx context.Context, run commandRunner) error {
	if err := run(ctx, "", "sudo", "-n", "apt-get", "update", "-y"); err != nil {
		return err
	}
	return run(ctx, "", "sudo", "-n", "apt-get", "install", "-y", "redis-server")
}

func installWithDnf(ctx context.Context, run commandRunner) error {
	return run(ctx, "", "sudo", "-n", "dnf", "install", "-y", "redis")
}

func installWithYum(ctx context.Context, run commandRunner) error {
	return run(ctx, "", "sudo", "-n", "yum", "install", "-y", "redis")
}

func installWithPacman(ctx context.Context, run commandRunner) error {
	return run(ctx, "", "sudo", "-n", "pacman", "-Sy", "--noconfirm", "redis")
}

func installWithBrew(ctx context.Context, run commandRunner) error {
	return run(ctx, "", "brew", "install", "redis")
}

func installWithWinget(ctx context.Context, run commandRunner) error {
	return run(ctx, "", "winget", "install", "--id", "Memurai.MemuraiDeveloper", "-e")
}

func redisStartCommands() []startupCmd {
	return []startupCmd{
		{
			name:  "systemctl redis-server",
			check: "systemctl",
			cmd:   "sudo",
			args:  []string{"-n", "systemctl", "start", "redis-server"},
		},
		{
			name:  "systemctl redis",
			check: "systemctl",
			cmd:   "sudo",
			args:  []string{"-n", "systemctl", "start", "redis"},
		},
		{
			name:  "brew services start redis",
			check: "brew",
			cmd:   "brew",
			args:  []string{"services", "start", "redis"},
		},
		{
			name:  "redis-server daemonize",
			check: "redis-server",
			cmd:   "redis-server",
			args:  []string{"--daemonize", "yes"},
		},
	}
}
