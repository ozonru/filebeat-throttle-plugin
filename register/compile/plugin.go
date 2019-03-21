// package main contains only import to register throttle plugin in filebeat.
package main

import (
	"github.com/elastic/beats/libbeat/processors"
	"github.com/ozonru/filebeat-throttle-plugin"
)

func init() {
	processors.RegisterPlugin("throttle", throttleplugin.NewProcessor)
}


