package main

import (
	"github.com/grishy/go-avahi-cname/avahi"
)

func main() {
	avahi.PublishCNAME("test.local")
	// cmd.Execute()
}
