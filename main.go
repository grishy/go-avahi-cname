package main

import (
	"os"

	"github.com/grishy/go-avahi-cname/avahi"
)

func main() {
	avahi.PublishCNAME(os.Args[1:])
}
