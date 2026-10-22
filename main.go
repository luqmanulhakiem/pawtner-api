package main

import (
	"pawtner-api/config"
	"pawtner-api/routes"
)

func main() {
	// Connect Databases
	config.ConnectDatabase()

	// Setup Router
	r := routes.SetupRouter()

	// Run on PORT 8000
	r.Run(":8000")
}
