// devvault-cli is the remote client for DevVault Anywhere. It pairs with a
// desktop device through a relay and opens encrypted tunnels:
//
//	pair:    devvault-cli pair --relay relay.example.com:7921 --fingerprint <fp> --code ABCD1234 --name work-laptop
//	status:  devvault-cli devices --identity ~/.devvault/client.json
//	tunnel:  devvault-cli tunnel --identity ~/.devvault/client.json --device <device-id> --port 7922
//
// Then call the agent API through the tunnel:
//
//	curl -H "X-DevVault-Client: cli" http://127.0.0.1:7922/api/health
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"devvault/internal/remote"
)

const defaultIdentity = "~/.devvault/client.json"

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "pair":
		err = cmdPair(os.Args[2:])
	case "devices":
		err = cmdDevices(os.Args[2:])
	case "tunnel":
		err = cmdTunnel(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`devvault-cli — remote client for DevVault Anywhere

Usage:
  devvault-cli pair    --relay <host:port> --fingerprint <sha256> --code <code> [--name my-laptop] [--identity path]
  devvault-cli devices --identity path
  devvault-cli tunnel  --identity path --device <device-id> --port <local-port>

The pairing code is generated in the DevVault desktop app (Remote page).
The relay TLS fingerprint is printed by the relay on startup and must be
pinned to prevent man-in-the-middle attacks.
`)
}

func cmdPair(args []string) error {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	relayAddr := fs.String("relay", "", "relay address host:port (required)")
	fp := fs.String("fingerprint", "", "relay TLS certificate SHA-256 fingerprint (required)")
	code := fs.String("code", "", "pairing code shown in the DevVault app (required)")
	name := fs.String("name", "devvault-cli", "a name for this client")
	identity := fs.String("identity", defaultIdentity, "where to save the client identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *relayAddr == "" || *code == "" {
		return fmt.Errorf("--relay and --code are required")
	}
	fmt.Println("Requesting pairing... approve it in the DevVault app within 2 minutes.")
	id, err := remote.Pair(*relayAddr, *fp, *code, *name, 130000000000) // 130s
	if err != nil {
		return err
	}
	if err := id.Save(expandPath(*identity)); err != nil {
		return fmt.Errorf("save identity: %w", err)
	}
	fmt.Printf("Paired as client %s.\nIdentity saved to %s\nNext: devvault-cli devices --identity %s\n",
		id.ClientID, expandPath(*identity), expandPath(*identity))
	return nil
}

func cmdDevices(args []string) error {
	fs := flag.NewFlagSet("devices", flag.ExitOnError)
	identity := fs.String("identity", defaultIdentity, "client identity file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := remote.LoadClientIdentity(expandPath(*identity))
	if err != nil {
		return err
	}
	devices, err := remote.ListDevices(id)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No devices linked to this client yet. Re-run pair to link one.")
		return nil
	}
	for _, d := range devices {
		state := "offline"
		if d.Online {
			state = "online"
		}
		fmt.Printf("%s  %-24s %-8s last seen: %s\n", d.DeviceID, d.Name, state, d.LastSeen)
	}
	return nil
}

func cmdTunnel(args []string) error {
	fs := flag.NewFlagSet("tunnel", flag.ExitOnError)
	identity := fs.String("identity", defaultIdentity, "client identity file")
	deviceID := fs.String("device", "", "target device id (required)")
	port := fs.Int("port", 7922, "local port to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *deviceID == "" {
		return fmt.Errorf("--device is required")
	}
	id, err := remote.LoadClientIdentity(expandPath(*identity))
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		return fmt.Errorf("cannot listen on 127.0.0.1:%d: %w", *port, err)
	}
	defer ln.Close()
	fmt.Printf("Tunnel ready: http://127.0.0.1:%d -> device %s\n", *port, *deviceID)
	fmt.Println(`Example: curl -H "X-DevVault-Client: cli" http://127.0.0.1:` + strconv.Itoa(*port) + `/api/health`)
	fmt.Println("Press Ctrl+C to stop.")
	for {
		local, err := ln.Accept()
		if err != nil {
			return nil
		}
		go func() {
			if err := remote.OpenTunnel(id, *deviceID, local); err != nil {
				fmt.Fprintln(os.Stderr, "tunnel error:", err)
				if closer, ok := local.(io.Closer); ok {
					_ = closer.Close()
				}
			}
		}()
	}
}
