package avahi

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

const (
	// From Avahi Python bindings
	DBUS_NAME             = "org.freedesktop.Avahi"
	DBUS_INTERFACE_SERVER = DBUS_NAME + ".Server"
	DBUS_PATH_SERVER      = "/"
)

func PublishCNAME(cname string) {
	fmt.Println("Publishing CNAME:", cname)

	fmt.Println("Connecting to session bus")
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to session bus:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Connecting to Avahi server")

}
