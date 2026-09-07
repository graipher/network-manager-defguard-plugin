package importer

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/graipher/network-manager-defguard-plugin/internal/defguard"
)

const serviceType = "org.freedesktop.NetworkManager.defguard"

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}
type runner struct{}

func (runner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func Run(ctx context.Context, dryRun bool, dg defguard.Runner, commands CommandRunner) error {
	if commands == nil {
		commands = runner{}
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	locations, err := (defguard.Client{Runner: dg}).List(ctx)
	if err != nil {
		return err
	}
	for _, location := range locations.Locations {
		name := profileName(location)
		uuid := profileUUID(current.Username, location.ID)
		existing, err := commands.Run(ctx, "nmcli", "-g", "vpn.service-type", "connection", "show", "uuid", uuid)
		switch {
		case err == nil && isDefguardServiceType(string(existing)):
			if dryRun {
				fmt.Printf("would update %s\n", name)
				continue
			}
		case err == nil:
			return fmt.Errorf("NetworkManager profile %q exists but is not managed by Defguard", name)
		case dryRun:
			fmt.Printf("would create %s\n", name)
			continue
		default:
			if _, err := commands.Run(ctx, "nmcli", "connection", "add", "type", "vpn", "vpn-type", "defguard", "con-name", name, "ifname", "--", "connection.uuid", uuid); err != nil {
				return fmt.Errorf("create %q: %w", name, err)
			}
		}
		data := fmt.Sprintf("location-id=%d,instance=%s,user=%s,endpoint=%s", location.ID, dataValue(location.Instance), current.Username, dataValue(location.Endpoint))
		if out, err := commands.Run(ctx, "nmcli", "connection", "modify", "uuid", uuid, "connection.id", name, "connection.permissions", "user:"+current.Username, "vpn.service-type", serviceType, "vpn.data", data); err != nil {
			return fmt.Errorf("update %q: %s: %w", name, out, err)
		}
	}
	return nil
}

func profileUUID(username string, locationID int64) string {
	sum := sha1.Sum([]byte("network-manager-defguard:" + username + ":" + strconv.FormatInt(locationID, 10)))
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func dataValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, `,`, `\,`).Replace(value)
}

func profileName(location defguard.Location) string {
	clean := func(value string) string { return strings.Join(strings.Fields(value), " ") }
	return clean(location.Name) + " (Defguard)"
}

func isDefguardServiceType(value string) bool {
	value = strings.TrimSpace(value)
	return value == "defguard" || value == serviceType
}
