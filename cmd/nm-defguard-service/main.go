package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/graipher/network-manager-defguard-plugin/internal/nmplugin"
)

func main() {
	bus := flag.String("bus", "system", "D-Bus bus: system or session")
	flag.Parse()
	var conn *dbus.Conn
	var err error
	if *bus == "session" {
		conn, err = dbus.SessionBus()
	} else {
		conn, err = dbus.SystemBus()
	}
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	service := nmplugin.New(conn, log.New(os.Stderr, "nm-defguard-service: ", log.LstdFlags))
	if err := service.Export(); err != nil {
		log.Fatal(err)
	}
	reply, err := conn.RequestName(nmplugin.BusName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("acquire %s: reply=%v err=%v", nmplugin.BusName, reply, err)
	}
	defer conn.ReleaseName(nmplugin.BusName)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
