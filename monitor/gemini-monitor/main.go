package main

import (
	"./geminimonitor"
)

func main() {
	geminimonitor.OnModuleStart("geminimonitor/conf.json")
}
