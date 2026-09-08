package defguard

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
)

type Location struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Instance string `json:"instance"`
	Endpoint string `json:"endpoint"`
}

type List struct {
	Locations []Location `json:"locations"`
}

type Active struct {
	Type      string `json:"connection_type"`
	Name      string `json:"name"`
	Interface string `json:"interface"`
}

type Status struct {
	Active []Active `json:"active"`
}

type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type CommandRunner struct {
	Binary string
	User   string
}

func (r CommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	binary := r.Binary
	if binary == "" {
		binary = "defguard-client"
	}
	if r.User != "" {
		u, err := user.Lookup(r.User)
		if err != nil {
			return nil, fmt.Errorf("look up user %q: %w", r.User, err)
		}
		runtimeDir := filepath.Join("/run/user", u.Uid)
		environment := []string{"env", "HOME=" + u.HomeDir, "USER=" + r.User, "LOGNAME=" + r.User, "XDG_RUNTIME_DIR=" + runtimeDir, "DBUS_SESSION_BUS_ADDRESS=unix:path=" + filepath.Join(runtimeDir, "bus"), "systemd-run", "--user", "--pipe", "--wait", "--collect", "--quiet", "--", binary}
		args = append([]string{"--user", r.User, "--"}, append(environment, args...)...)
		binary = "runuser"
	}
	out, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", string(out), err)
	}
	return out, nil
}

type Client struct{ Runner Runner }

func (c Client) List(ctx context.Context) (List, error) {
	var result List
	err := c.json(ctx, &result, "list", "--json")
	return result, err
}

func (c Client) Status(ctx context.Context) (Status, error) {
	var result Status
	err := c.json(ctx, &result, "status", "--json")
	return result, err
}

func (c Client) Connect(ctx context.Context, id int64, instance, trafficMode string) error {
	args := []string{"connect", "--id", strconv.FormatInt(id, 10), "--json"}
	if instance != "" {
		args = append(args, "--instance", instance)
	}
	switch trafficMode {
	case "all":
		args = append(args, "--all-traffic")
	case "predefined":
		args = append(args, "--predefined-traffic")
	default:
		return fmt.Errorf("invalid traffic mode %q", trafficMode)
	}
	_, err := c.Runner.Run(ctx, args...)
	return err
}

func (c Client) Disconnect(ctx context.Context, id int64, instance string) error {
	args := []string{"disconnect", "--id", strconv.FormatInt(id, 10), "--json"}
	if instance != "" {
		args = append(args, "--instance", instance)
	}
	_, err := c.Runner.Run(ctx, args...)
	return err
}

func (c Client) json(ctx context.Context, target any, args ...string) error {
	out, err := c.Runner.Run(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, target); err != nil {
		return fmt.Errorf("decode defguard-client output: %w", err)
	}
	return nil
}

func FindLocation(list List, id int64, instance string) (Location, error) {
	for _, location := range list.Locations {
		if location.ID == id && (instance == "" || location.Instance == instance) {
			return location, nil
		}
	}
	return Location{}, fmt.Errorf("Defguard location %d in instance %q not found", id, instance)
}

func NewInterface(before, after Status, locationName string) (string, error) {
	known := make(map[string]bool, len(before.Active))
	for _, active := range before.Active {
		known[active.Interface] = true
	}
	for _, active := range after.Active {
		if active.Type == "location" && active.Name == locationName && !known[active.Interface] {
			return active.Interface, nil
		}
	}
	var found string
	for _, active := range after.Active {
		if active.Type == "location" && active.Name == locationName {
			if found != "" {
				return "", fmt.Errorf("multiple active Defguard locations named %q", locationName)
			}
			found = active.Interface
		}
	}
	if found == "" {
		return "", fmt.Errorf("connected Defguard location %q has no active interface", locationName)
	}
	return found, nil
}

func HasInterface(status Status, interfaceName, locationName string) bool {
	for _, active := range status.Active {
		if active.Type == "location" && active.Interface == interfaceName && (locationName == "" || active.Name == locationName) {
			return true
		}
	}
	return false
}
