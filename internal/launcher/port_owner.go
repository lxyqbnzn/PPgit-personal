package launcher

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"ppgit-go/internal/platform"
)

func loopbackEndpoint(host, port string) (netip.AddrPort, error) {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ip, err := netip.ParseAddr(host)
	number, portErr := strconv.Atoi(port)
	if err != nil || !ip.IsLoopback() || portErr != nil || number < 1 || number > 65535 {
		return netip.AddrPort{}, fmt.Errorf("stop requires a loopback host and a port between 1 and 65535")
	}
	return netip.AddrPortFrom(ip.Unmap(), uint16(number)), nil
}

func StopPortOwner(host, port string) (bool, error) {
	address, err := loopbackEndpoint(host, port)
	if err != nil {
		return false, err
	}
	listener, err := net.Listen("tcp", address.String())
	if err == nil {
		listener.Close()
		return false, nil
	}
	if !platform.IsAddressInUse(err) {
		return false, fmt.Errorf("cannot check preferred port: %w", err)
	}
	processes, err := platform.PPGitProcesses()
	if err != nil {
		return false, err
	}
	return stopPortOwner(address, processes)
}

func stopPortOwner(address netip.AddrPort, processes []platform.Process) (bool, error) {
	for _, process := range processes {

		runtimePath := filepath.Join(Root(process.Executable), "data", "runtime.json")
		info, err := Read(runtimePath)
		if err != nil || info.PID != process.PID {
			continue
		}
		parsed, err := url.Parse(info.URL)
		if err != nil {
			continue
		}
		candidate, err := loopbackEndpoint(parsed.Hostname(), parsed.Port())
		if err != nil || candidate != address {
			continue
		}
		if err := requestStop(info); err != nil {
			return false, err
		}
		fmt.Printf("Verified PPGit Go at %s (data: %s).\n", info.URL, filepath.Dir(runtimePath))
		return true, nil
	}
	return false, fmt.Errorf("no verified PPGit Go instance owns %s; no process was stopped. For a custom data directory, use the original installation's stop command", address)
}
